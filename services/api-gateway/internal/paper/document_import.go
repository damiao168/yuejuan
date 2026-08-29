package paper

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/files"
)

const maxDocumentTextBytes = 700_000

var errDocumentOCRRequired = errors.New("document OCR required")

type DocumentImportService struct {
	store   Store
	files   files.Store
	objects files.ObjectStorage
	client  *http.Client
	baseURL string
	token   string
}

type documentParseRequest struct {
	RequestID  string `json:"request_id"`
	Subject    string `json:"subject"`
	PaperText  string `json:"paper_text"`
	AnswerText string `json:"answer_text"`
}

type documentParseResponse struct {
	Questions []PaperImportDraftQuestion `json:"questions"`
	Issues    []string                   `json:"issues"`
}

func NewDocumentImportService(store Store, fileStore files.Store, objects files.ObjectStorage, baseURL, token string, timeout time.Duration) *DocumentImportService {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &DocumentImportService{store: store, files: fileStore, objects: objects, client: &http.Client{Timeout: timeout}, baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), token: token}
}

func (s *DocumentImportService) Start(ctx context.Context, tenantID, examID, userID string, input CreatePaperImportInput) (PaperImportJob, error) {
	job, err := s.store.CreatePaperImport(ctx, tenantID, examID, userID, input)
	if err != nil {
		return PaperImportJob{}, err
	}
	paperText, paperErr := s.assetText(ctx, tenantID, input.PaperFileAssetID)
	answerText, answerErr := s.assetText(ctx, tenantID, input.AnswerFileAssetID)
	ocrAssets := []PaperImportOCRAsset{}
	for _, candidate := range []struct {
		role, id string
		err      error
	}{{"paper", input.PaperFileAssetID, paperErr}, {"answer", input.AnswerFileAssetID, answerErr}} {
		if candidate.err == nil {
			continue
		}
		if !errors.Is(candidate.err, errDocumentOCRRequired) {
			failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, candidate.role+"_text_unavailable", []string{candidate.err.Error()})
			return failed, nil
		}
		asset, getErr := s.files.Get(ctx, tenantID, candidate.id)
		if getErr != nil {
			return PaperImportJob{}, getErr
		}
		ocrAssets = append(ocrAssets, PaperImportOCRAsset{Role: candidate.role, FileAssetID: candidate.id, ContentType: asset.ContentType})
	}
	if len(ocrAssets) > 0 {
		runtime, ok := s.store.(PaperImportRuntime)
		if !ok {
			failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "paper_ocr_unavailable", []string{"扫描版 PDF 需要 OCR Worker，但当前存储未配置任务运行时"})
			return failed, nil
		}
		if err := runtime.QueuePaperImportOCR(ctx, tenantID, job, userID, ocrAssets); err != nil {
			return PaperImportJob{}, err
		}
		return job, nil
	}
	return s.completeParsedText(ctx, tenantID, job, paperText, answerText, nil)
}

func (s *DocumentImportService) completeParsedText(ctx context.Context, tenantID string, job PaperImportJob, paperText, answerText string, extraIssues []string) (PaperImportJob, error) {
	parsed, err := s.parse(ctx, job.ID, job.Subject, paperText, answerText)
	if err != nil {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "ai_parse_failed", []string{"AI 解析服务暂不可用，可稍后重试"})
		return failed, nil
	}
	if len(parsed.Questions) == 0 {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "no_questions_detected", []string{"未识别到题目，请检查文件是否包含可复制文字"})
		return failed, nil
	}
	for i := range parsed.Questions {
		parsed.Questions[i].QuestionNo = strings.TrimSpace(parsed.Questions[i].QuestionNo)
		parsed.Questions[i].QuestionType = strings.TrimSpace(parsed.Questions[i].QuestionType)
		parsed.Questions[i].Stem = strings.TrimSpace(parsed.Questions[i].Stem)
		if err := validateQuestionInput(parsed.Questions[i].QuestionNo, parsed.Questions[i].QuestionType, parsed.Questions[i].Score); err != nil {
			parsed.Questions[i].Issues = append(parsed.Questions[i].Issues, err.Error())
		}
	}
	parsed.Issues = append(parsed.Issues, extraIssues...)
	return s.store.CompletePaperImport(ctx, tenantID, job.ID, parsed.Questions, parsed.Issues)
}

func (s *DocumentImportService) CompleteOCR(ctx context.Context, tenantID, importID string, blocks []PaperImportOCRBlock) (PaperImportJob, error) {
	job, err := s.store.GetPaperImport(ctx, tenantID, importID)
	if err != nil {
		return PaperImportJob{}, err
	}
	if job.Status != "processing" {
		return PaperImportJob{}, ErrConflict
	}
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Role == blocks[j].Role {
			return blocks[i].PageNo < blocks[j].PageNo
		}
		return blocks[i].Role < blocks[j].Role
	})
	texts := map[string][]string{"paper": {}, "answer": {}}
	issues := []string{}
	for _, block := range blocks {
		if block.Role != "paper" && block.Role != "answer" {
			continue
		}
		if strings.TrimSpace(block.Text) != "" {
			texts[block.Role] = append(texts[block.Role], block.Text)
		}
		if block.Confidence < 0.75 {
			issues = append(issues, fmt.Sprintf("%s 第 %d 页包含低置信度 OCR 文本（%.0f%%），必须人工核对", block.Role, block.PageNo, block.Confidence*100))
		}
	}
	resolve := func(role, assetID string) (string, error) {
		if len(texts[role]) > 0 {
			return strings.Join(texts[role], "\n"), nil
		}
		text, textErr := s.assetText(ctx, tenantID, assetID)
		if errors.Is(textErr, errDocumentOCRRequired) {
			return "", errors.New("OCR 未返回可用文字")
		}
		return text, textErr
	}
	paperText, err := resolve("paper", job.PaperFileAssetID)
	if err != nil {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "paper_ocr_failed", []string{err.Error()})
		return failed, nil
	}
	answerText, err := resolve("answer", job.AnswerFileAssetID)
	if err != nil {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "answer_ocr_failed", []string{err.Error()})
		return failed, nil
	}
	return s.completeParsedText(ctx, tenantID, job, paperText, answerText, issues)
}

func (s *DocumentImportService) assetText(ctx context.Context, tenantID, id string) (string, error) {
	asset, err := s.files.Get(ctx, tenantID, id)
	if err != nil {
		return "", ErrInvalidInput
	}
	body, err := s.objects.Get(ctx, asset.StorageBucket, asset.StorageKey)
	if err != nil {
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 32<<20))
	if err != nil {
		return "", err
	}
	var text string
	switch {
	case strings.Contains(asset.ContentType, "wordprocessingml") || strings.HasSuffix(strings.ToLower(asset.OriginalName), ".docx"):
		text, err = extractDOCXText(data)
	case asset.ContentType == "application/pdf" || strings.HasSuffix(strings.ToLower(asset.OriginalName), ".pdf"):
		text, err = extractPDFText(data)
	default:
		err = errors.New("仅支持 PDF 或 DOCX 文件")
	}
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if len([]rune(text)) < 20 {
		if asset.ContentType == "application/pdf" || strings.HasSuffix(strings.ToLower(asset.OriginalName), ".pdf") {
			return "", errDocumentOCRRequired
		}
		return "", errors.New("文件没有可提取文字")
	}
	if len(text) > maxDocumentTextBytes {
		text = text[:maxDocumentTextBytes]
	}
	return text, nil
}

func (s *DocumentImportService) parse(ctx context.Context, requestID, subject, paperText, answerText string) (documentParseResponse, error) {
	if s.baseURL == "" || len(s.token) < 32 {
		return documentParseResponse{}, errors.New("AI service not configured")
	}
	body, _ := json.Marshal(documentParseRequest{RequestID: requestID, Subject: subject, PaperText: paperText, AnswerText: answerText})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/paper/parse", bytes.NewReader(body))
	if err != nil {
		return documentParseResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return documentParseResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return documentParseResponse{}, fmt.Errorf("paper parser returned %d", resp.StatusCode)
	}
	var out documentParseResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	if err := dec.Decode(&out); err != nil {
		return documentParseResponse{}, err
	}
	return out, nil
}

func extractDOCXText(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("DOCX 文件损坏")
	}
	for _, file := range r.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		raw, err := io.ReadAll(io.LimitReader(rc, maxDocumentTextBytes*2))
		rc.Close()
		if err != nil {
			return "", err
		}
		s := string(raw)
		s = regexp.MustCompile(`</w:p>`).ReplaceAllString(s, "\n")
		s = regexp.MustCompile(`<w:(tab|br)[^>]*/>`).ReplaceAllString(s, "\t")
		s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
		return html.UnescapeString(s), nil
	}
	return "", errors.New("DOCX 正文缺失")
}

func extractPDFText(data []byte) (string, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", errors.New("PDF 文件损坏")
	}
	// Conservative fallback for text PDFs. Encoded/scanned PDFs deliberately fail
	// closed so that OCR is used instead of inventing question content.
	re := regexp.MustCompile(`\(([^()]*)\)\s*Tj`)
	matches := re.FindAllSubmatch(data, -1)
	var b strings.Builder
	for _, match := range matches {
		value := strings.ReplaceAll(string(match[1]), `\(`, "(")
		value = strings.ReplaceAll(value, `\)`, ")")
		value = strings.ReplaceAll(value, `\\`, `\`)
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

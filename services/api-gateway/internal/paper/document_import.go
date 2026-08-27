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
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/files"
)

const maxDocumentTextBytes = 700_000

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
	paperText, err := s.assetText(ctx, tenantID, input.PaperFileAssetID)
	if err != nil {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "paper_text_unavailable", []string{err.Error()})
		return failed, nil
	}
	answerText, err := s.assetText(ctx, tenantID, input.AnswerFileAssetID)
	if err != nil {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "answer_text_unavailable", []string{err.Error()})
		return failed, nil
	}
	parsed, err := s.parse(ctx, job.ID, input.Subject, paperText, answerText)
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
	return s.store.CompletePaperImport(ctx, tenantID, job.ID, parsed.Questions, parsed.Issues)
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
		return "", errors.New("文件没有可提取文字；扫描版请先完成 OCR")
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

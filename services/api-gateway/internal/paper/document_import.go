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
	RequestID string                     `json:"request_id"`
	Subject   string                     `json:"subject"`
	Documents []normalizedImportDocument `json:"documents"`
}

type documentParseResponse struct {
	Documents          []PaperImportDetectedDocument `json:"documents"`
	QuestionCandidates []QuestionCandidate           `json:"question_candidates"`
	AnswerCandidates   []AnswerCandidate             `json:"answer_candidates"`
	SolutionCandidates []SolutionCandidate           `json:"solution_candidates"`
	RubricCandidates   []RubricCandidate             `json:"rubric_candidates"`
	Issues             []PaperImportIssue            `json:"issues"`
}

type normalizedImportDocument struct {
	SourceID      string                `json:"source_id"`
	FileAssetID   string                `json:"file_asset_id"`
	DocumentIndex int                   `json:"document_index"`
	RoleHint      string                `json:"role_hint"`
	Content       string                `json:"content"`
	Blocks        []PaperImportOCRBlock `json:"blocks"`
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
	return s.processSources(ctx, tenantID, userID, job)
}

func (s *DocumentImportService) AddSources(ctx context.Context, tenantID, userID, importID string, input AddPaperImportSourcesInput) (PaperImportJob, error) {
	job, err := s.store.AddPaperImportSources(ctx, tenantID, importID, userID, input)
	if err != nil {
		return PaperImportJob{}, err
	}
	return s.processSources(ctx, tenantID, userID, job)
}

func (s *DocumentImportService) ReplaceSources(ctx context.Context, tenantID, userID, importID string, input ReplacePaperImportSourcesInput) (PaperImportJob, error) {
	job, err := s.store.ReplacePaperImportSources(ctx, tenantID, importID, userID, input)
	if err != nil {
		return PaperImportJob{}, err
	}
	return s.processSources(ctx, tenantID, userID, job)
}

func (s *DocumentImportService) processSources(ctx context.Context, tenantID, userID string, job PaperImportJob) (PaperImportJob, error) {
	documents := []normalizedImportDocument{}
	ocrAssets := []PaperImportOCRAsset{}
	for _, source := range job.Sources {
		text, textErr := s.assetText(ctx, tenantID, source.FileAssetID)
		if textErr == nil {
			documents = append(documents, normalizedImportDocument{SourceID: source.ID, FileAssetID: source.FileAssetID, DocumentIndex: source.DocumentIndex, RoleHint: source.RoleHint, Content: text, Blocks: []PaperImportOCRBlock{}})
			continue
		}
		if !errors.Is(textErr, errDocumentOCRRequired) {
			failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "source_text_unavailable", []string{textErr.Error()})
			return failed, nil
		}
		asset, getErr := s.files.Get(ctx, tenantID, source.FileAssetID)
		if getErr != nil {
			return PaperImportJob{}, getErr
		}
		ocrAssets = append(ocrAssets, PaperImportOCRAsset{SourceID: source.ID, DocumentIndex: source.DocumentIndex, RoleHint: source.RoleHint, FileAssetID: source.FileAssetID, ContentType: asset.ContentType})
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
	return s.completeParsedDocuments(ctx, tenantID, job, documents, nil)
}

func (s *DocumentImportService) completeParsedDocuments(ctx context.Context, tenantID string, job PaperImportJob, documents []normalizedImportDocument, extraIssues []PaperImportIssue) (PaperImportJob, error) {
	parsed, err := s.parse(ctx, job.ID, job.Subject, documents)
	if err != nil {
		failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "ai_parse_failed", []string{"AI 解析服务暂不可用，可稍后重试"})
		return failed, nil
	}
	parsed.Issues = append(parsed.Issues, extraIssues...)
	return s.store.CompletePaperImportCandidates(ctx, tenantID, job.ID, parsed.Documents, parsed.QuestionCandidates, parsed.AnswerCandidates, parsed.SolutionCandidates, parsed.RubricCandidates, parsed.Issues)
}

func (s *DocumentImportService) CompleteOCR(ctx context.Context, tenantID, importID string, blocks []PaperImportOCRBlock) (PaperImportJob, error) {
	job, err := s.store.GetPaperImport(ctx, tenantID, importID)
	if err != nil {
		return PaperImportJob{}, err
	}
	if job.Status != "processing" {
		return PaperImportJob{}, ErrConflict
	}
	sortPaperImportOCRBlocks(blocks)
	bySource := map[string][]PaperImportOCRBlock{}
	sourceByID := map[string]PaperImportSource{}
	for _, source := range job.Sources {
		sourceByID[source.ID] = source
	}
	issues := []PaperImportIssue{}
	for _, block := range blocks {
		source, known := sourceByID[block.SourceID]
		if !known || source.DocumentIndex != block.DocumentIndex {
			return PaperImportJob{}, ErrInvalidInput
		}
		if strings.TrimSpace(block.Text) != "" {
			bySource[block.SourceID] = append(bySource[block.SourceID], block)
		}
		if block.Confidence < 0.75 {
			c := block.Confidence
			issues = append(issues, PaperImportIssue{Code: "LOW_OCR_CONFIDENCE", Severity: "error", Certainty: "confirmed", Message: fmt.Sprintf("第 %d 页部分内容识别不确定（%.0f%%）", block.PageNo, block.Confidence*100), Confidence: &c, SourceRefs: []PaperImportSourceRef{{SourceID: block.SourceID, FileAssetID: source.FileAssetID, DocumentIndex: block.DocumentIndex, PageNo: block.PageNo, BlockID: block.BlockID, BBox: block.BBox, OCRConfidence: &c}}, ResolutionHint: "请对照原图核对"})
		}
	}
	issues = append(issues, possiblePageMissingIssues(job.Sources, blocks)...)
	documents := []normalizedImportDocument{}
	for _, source := range job.Sources {
		sourceBlocks := bySource[source.ID]
		contentParts := []string{}
		for _, block := range sourceBlocks {
			contentParts = append(contentParts, block.Text)
		}
		content := strings.Join(contentParts, "\n")
		if content == "" {
			text, textErr := s.assetText(ctx, tenantID, source.FileAssetID)
			if textErr != nil {
				failed, _ := s.store.FailPaperImport(ctx, tenantID, job.ID, "source_ocr_failed", []string{"OCR 未返回可用文字"})
				return failed, nil
			}
			content = text
		}
		documents = append(documents, normalizedImportDocument{SourceID: source.ID, FileAssetID: source.FileAssetID, DocumentIndex: source.DocumentIndex, RoleHint: source.RoleHint, Content: content, Blocks: sourceBlocks})
	}
	return s.completeParsedDocuments(ctx, tenantID, job, documents, issues)
}

func sortPaperImportOCRBlocks(blocks []PaperImportOCRBlock) {
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].DocumentIndex == blocks[j].DocumentIndex {
			if blocks[i].PageNo == blocks[j].PageNo {
				return false
			}
			return blocks[i].PageNo < blocks[j].PageNo
		}
		return blocks[i].DocumentIndex < blocks[j].DocumentIndex
	})
}

func possiblePageMissingIssues(sources []PaperImportSource, blocks []PaperImportOCRBlock) []PaperImportIssue {
	pagesBySource := map[string]map[int]bool{}
	for _, block := range blocks {
		if block.PageNo > 0 {
			if pagesBySource[block.SourceID] == nil {
				pagesBySource[block.SourceID] = map[int]bool{}
			}
			pagesBySource[block.SourceID][block.PageNo] = true
		}
	}
	issues := []PaperImportIssue{}
	for _, source := range sources {
		pages := pagesBySource[source.ID]
		maxPage := 0
		for page := range pages {
			if page > maxPage {
				maxPage = page
			}
		}
		for page := 1; page < maxPage; page++ {
			if pages[page] {
				continue
			}
			issues = append(issues, candidateIssue("POSSIBLE_PAGE_MISSING", "warning", "suspected", "", fmt.Sprintf("资料第 %d 页没有可用 OCR 文本", page), "请核对该页是否空白、漏传或识别失败", []PaperImportSourceRef{{SourceID: source.ID, FileAssetID: source.FileAssetID, DocumentIndex: source.DocumentIndex, PageNo: page}}))
		}
	}
	return issues
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
	case strings.HasPrefix(asset.ContentType, "image/"):
		return "", errDocumentOCRRequired
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

func (s *DocumentImportService) parse(ctx context.Context, requestID, subject string, documents []normalizedImportDocument) (documentParseResponse, error) {
	if s.baseURL == "" || len(s.token) < 32 {
		return documentParseResponse{}, errors.New("AI service not configured")
	}
	body, _ := json.Marshal(documentParseRequest{RequestID: requestID, Subject: subject, Documents: documents})
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

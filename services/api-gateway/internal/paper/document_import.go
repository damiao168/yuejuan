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
	"sync"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/files"
)

const maxDocumentTextBytes = 700_000

var errDocumentOCRRequired = errors.New("document OCR required")

type DocumentImportService struct {
	store         Store
	files         files.Store
	objects       files.ObjectStorage
	client        *http.Client
	baseURL       string
	token         string
	parseMu       sync.Mutex
	activeParses  map[string]*paperImportParse
	modelResolver func(context.Context, string) (*DocumentModelConfig, error)
}

// DocumentModelConfig travels only on the authenticated internal parser request.
// It must never be persisted in a paper import or returned to the browser.
type DocumentModelConfig struct {
	AdapterType  string `json:"adapter_type"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
}

func (s *DocumentImportService) WithModelResolver(resolve func(context.Context, string) (*DocumentModelConfig, error)) *DocumentImportService {
	s.modelResolver = resolve
	return s
}

type paperImportParse struct {
	cancel context.CancelFunc
}

type documentParseRequest struct {
	RequestID    string                     `json:"request_id"`
	Subject      string                     `json:"subject"`
	Documents    []normalizedImportDocument `json:"documents"`
	ManagedModel *DocumentModelConfig       `json:"managed_model,omitempty"`
}

type documentParseResponse struct {
	Documents          []PaperImportDetectedDocument `json:"documents"`
	QuestionCandidates []QuestionCandidate           `json:"question_candidates"`
	AnswerCandidates   []AnswerCandidate             `json:"answer_candidates"`
	SolutionCandidates []SolutionCandidate           `json:"solution_candidates"`
	RubricCandidates   []RubricCandidate             `json:"rubric_candidates"`
	Issues             []PaperImportIssue            `json:"issues"`
}

// The parser input is persisted before execution so it must have one stable
// wire representation shared by the request builder and the task executor.
type normalizedImportDocument = PaperImportParseDocument

func NewDocumentImportService(store Store, fileStore files.Store, objects files.ObjectStorage, baseURL, token string, timeout time.Duration) *DocumentImportService {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &DocumentImportService{store: store, files: fileStore, objects: objects, client: &http.Client{Timeout: timeout}, baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), token: token, activeParses: map[string]*paperImportParse{}}
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
	if runtime, ok := s.store.(PaperImportRuntime); ok {
		if err := runtime.QueuePaperImportParse(ctx, tenantID, job, userID, PaperImportParseRequest{Documents: documents}); err != nil {
			return PaperImportJob{}, err
		}
		return job, nil
	}
	return s.completeParsedDocuments(ctx, tenantID, job, documents, nil)
}

func (s *DocumentImportService) completeParsedDocuments(ctx context.Context, tenantID string, job PaperImportJob, documents []normalizedImportDocument, extraIssues []PaperImportIssue) (PaperImportJob, error) {
	parseContext, release := s.beginParse(ctx, tenantID, job.ID)
	defer release()
	parsed, err := s.parseForTenant(parseContext, tenantID, job.ID, job.Subject, documents)
	if err != nil {
		if errors.Is(parseContext.Err(), context.Canceled) {
			return PaperImportJob{}, ErrConflict
		}
		failed, failErr := s.store.FailPaperImport(ctx, tenantID, job.ID, "ai_parse_failed", []string{"AI 解析服务暂不可用，可稍后重试"})
		if failErr != nil {
			return PaperImportJob{}, failErr
		}
		return failed, nil
	}
	parsed.Issues = append(parsed.Issues, extraIssues...)
	parsed.Issues = appendNoExamContentIssue(parsed, documents)
	return s.store.CompletePaperImportCandidates(ctx, tenantID, job.ID, parsed.Documents, parsed.QuestionCandidates, parsed.AnswerCandidates, parsed.SolutionCandidates, parsed.RubricCandidates, parsed.Issues)
}

func (s *DocumentImportService) beginParse(ctx context.Context, tenantID, importID string) (context.Context, func()) {
	parseContext, cancel := context.WithCancel(ctx)
	entry := &paperImportParse{cancel: cancel}
	key := tenantID + "\x00" + importID
	s.parseMu.Lock()
	previous := s.activeParses[key]
	s.activeParses[key] = entry
	s.parseMu.Unlock()
	if previous != nil {
		previous.cancel()
	}
	return parseContext, func() {
		cancel()
		s.parseMu.Lock()
		if s.activeParses[key] == entry {
			delete(s.activeParses, key)
		}
		s.parseMu.Unlock()
	}
}

func (s *DocumentImportService) Cancel(tenantID, importID string) {
	key := tenantID + "\x00" + importID
	s.parseMu.Lock()
	entry := s.activeParses[key]
	s.parseMu.Unlock()
	if entry != nil {
		entry.cancel()
	}
}

func appendNoExamContentIssue(parsed documentParseResponse, documents []normalizedImportDocument) []PaperImportIssue {
	issues := append([]PaperImportIssue{}, parsed.Issues...)
	if len(parsed.QuestionCandidates) > 0 || len(parsed.AnswerCandidates) > 0 || len(parsed.SolutionCandidates) > 0 || len(parsed.RubricCandidates) > 0 {
		return issues
	}
	for _, issue := range issues {
		if issue.Code == "NO_EXAM_CONTENT_DETECTED" {
			return issues
		}
	}
	refs := make([]PaperImportSourceRef, 0, len(documents))
	for _, document := range documents {
		refs = append(refs, PaperImportSourceRef{SourceID: document.SourceID, FileAssetID: document.FileAssetID, DocumentIndex: document.DocumentIndex})
	}
	return append(issues, candidateIssue(
		"NO_EXAM_CONTENT_DETECTED",
		"error",
		"confirmed",
		"",
		"未识别到与考试有关的题目、答案、解析或评分标准，请检查是否上传了无关图片或错误文件",
		"请重新上传正确的考试资料；若图片确属考试资料，可重试识别或手动补充题目",
		refs,
	))
}

func (s *DocumentImportService) PrepareOCRParse(ctx context.Context, tenantID, importID string, blocks []PaperImportOCRBlock) (PaperImportJob, PaperImportParseRequest, error) {
	job, err := s.store.GetPaperImport(ctx, tenantID, importID)
	if err != nil {
		return PaperImportJob{}, PaperImportParseRequest{}, err
	}
	if job.Status != "processing" {
		return PaperImportJob{}, PaperImportParseRequest{}, ErrConflict
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
			return PaperImportJob{}, PaperImportParseRequest{}, ErrInvalidInput
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
				return PaperImportJob{}, PaperImportParseRequest{}, fmt.Errorf("%w: OCR returned no usable text", ErrInvalidInput)
			}
			content = text
		}
		documents = append(documents, normalizedImportDocument{SourceID: source.ID, FileAssetID: source.FileAssetID, DocumentIndex: source.DocumentIndex, RoleHint: source.RoleHint, Content: content, Blocks: sourceBlocks})
	}
	return job, PaperImportParseRequest{Documents: documents, ExtraIssues: issues}, nil
}

func (s *DocumentImportService) ExecuteParse(ctx context.Context, tenantID, importID string, input PaperImportParseRequest) (PaperImportJob, error) {
	job, err := s.store.GetPaperImport(ctx, tenantID, importID)
	if err != nil {
		return PaperImportJob{}, err
	}
	if job.Status == "review_required" {
		return job, nil
	}
	if job.Status != "processing" {
		return PaperImportJob{}, ErrConflict
	}
	parseContext, release := s.beginParse(ctx, tenantID, job.ID)
	defer release()
	parsed, err := s.parseForTenant(parseContext, tenantID, job.ID, job.Subject, input.Documents)
	if err != nil {
		return PaperImportJob{}, err
	}
	parsed.Issues = append(parsed.Issues, input.ExtraIssues...)
	parsed.Issues = appendNoExamContentIssue(parsed, input.Documents)
	return s.store.CompletePaperImportCandidates(ctx, tenantID, job.ID, parsed.Documents, parsed.QuestionCandidates, parsed.AnswerCandidates, parsed.SolutionCandidates, parsed.RubricCandidates, parsed.Issues)
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

func (s *DocumentImportService) parseForTenant(ctx context.Context, tenantID, requestID, subject string, documents []normalizedImportDocument) (documentParseResponse, error) {
	if s.baseURL == "" || len(s.token) < 32 {
		return documentParseResponse{}, errors.New("AI service not configured")
	}
	var model *DocumentModelConfig
	if s.modelResolver != nil && tenantID != "" {
		var err error
		model, err = s.modelResolver(ctx, tenantID)
		if err != nil {
			return documentParseResponse{}, errors.New("school model configuration unavailable")
		}
	}
	body, _ := json.Marshal(documentParseRequest{RequestID: requestID, Subject: subject, Documents: documents, ManagedModel: model})
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

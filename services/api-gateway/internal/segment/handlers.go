package segment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	submissionpkg "edugrade-enterprise/services/api-gateway/internal/submission"
)

type Handler struct {
	store       Store
	papers      paper.Store
	submissions submissionpkg.Store
	audit       auth.Store
}

func NewHandler(store Store, papers paper.Store, submissions submissionpkg.Store, audit auth.Store) *Handler {
	return &Handler{store: store, papers: papers, submissions: submissions, audit: audit}
}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	submissionID := r.PathValue("id")
	sub, err := h.submissions.Get(r.Context(), user.TenantID, submissionID)
	if err != nil {
		writeStoreError(w, r, ErrNotFound)
		return
	}
	if sub.Status != "ready_for_ocr" {
		writeStoreError(w, r, ErrNotReady)
		return
	}
	questions, err := h.papers.ListQuestions(r.Context(), user.TenantID, sub.ExamID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "segment_question_load_failed", "failed to load questions")
		return
	}
	pages, err := h.submissions.ListPages(r.Context(), user.TenantID, submissionID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "segment_page_load_failed", "failed to load submission pages")
		return
	}
	pageByNo := map[int]string{}
	for _, page := range pages {
		pageByNo[page.PageNo] = page.ID
	}

	issues := []Issue{}
	inputs := []CreateSegmentInput{}
	if len(questions) == 0 {
		issues = append(issues, Issue{Code: "no_questions", Message: "exam has no configured questions"})
	}
	for _, question := range questions {
		if question.AnswerArea == nil {
			issues = append(issues, Issue{Code: "answer_area_missing", Message: "question " + question.QuestionNo + " is missing answer_area"})
			continue
		}
		pageNo, bbox, err := ParseAnswerArea(question.AnswerArea)
		if err != nil {
			issues = append(issues, Issue{Code: "answer_area_invalid", Message: "question " + question.QuestionNo + ": " + err.Error()})
			continue
		}
		pageID, ok := pageByNo[pageNo]
		if !ok {
			issues = append(issues, Issue{Code: "submission_page_missing", Message: fmt.Sprintf("question %s expects page %d, but submission page is missing", question.QuestionNo, pageNo)})
			continue
		}
		inputs = append(inputs, CreateSegmentInput{
			TenantID:         user.TenantID,
			SubmissionID:     submissionID,
			SubmissionPageID: pageID,
			QuestionID:       question.ID,
			QuestionNo:       question.QuestionNo,
			BBox:             bbox,
			Source:           "configured_answer_area",
			Status:           "generated",
		})
	}
	segments := []Segment{}
	if len(inputs) > 0 {
		segments, err = h.store.CreateSegments(r.Context(), inputs)
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
	}
	h.auditAction(r, "segment.generated", "submission", submissionID, "generate answer segments from configured answer areas")
	httpx.JSON(w, http.StatusOK, map[string]any{"result": GenerateResult{Valid: len(issues) == 0, Issues: issues, Segments: segments}})
}

func (h *Handler) ListBySubmission(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListBySubmission(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"segments": out})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input UpdateSegmentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		input.Status = &status
	}
	if input.ReviewNotes != nil {
		notes := strings.TrimSpace(*input.ReviewNotes)
		input.ReviewNotes = &notes
	}
	out, err := h.store.Update(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "segment.updated", "answer_segment", out.ID, "update answer segment")
	httpx.JSON(w, http.StatusOK, map[string]any{"segment": out})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "answer_segment_not_found", "answer segment not found")
	case errors.Is(err, ErrNotReady):
		httpx.Error(w, r, http.StatusConflict, "submission_not_ready_for_segmentation", "submission must be ready_for_ocr before segmentation")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_answer_segment", "answer segment input is invalid")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "answer_segment_operation_failed", "answer segment operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func (h *Handler) auditAction(r *http.Request, action string, targetType string, targetID string, reason string) {
	user := mustUser(r)
	_ = h.audit.Audit(r.Context(), auth.AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})
}

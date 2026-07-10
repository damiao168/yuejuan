package submission

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/files"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store     Store
	fileStore files.Store
	audit     auth.Store
}

func NewHandler(store Store, fileStore files.Store, audit auth.Store) *Handler {
	return &Handler{store: store, fileStore: fileStore, audit: audit}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input CreateSubmissionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.StudentID = strings.TrimSpace(input.StudentID)
	input.CandidateNo = strings.TrimSpace(input.CandidateNo)
	input.SourceType = strings.TrimSpace(input.SourceType)
	if input.StudentID != "" && !files.IsUUIDLike(input.StudentID) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_student_id", "student_id must be a UUID")
		return
	}
	out, err := h.store.Create(r.Context(), user.TenantID, r.PathValue("examId"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "submission.created", "submission", out.ID, "create submission")
	httpx.JSON(w, http.StatusCreated, map[string]any{"submission": out})
}

func (h *Handler) ListByExam(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListByExam(r.Context(), user.TenantID, r.PathValue("examId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"submissions": out})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.Get(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"submission": out})
}

func (h *Handler) AddPage(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input AddPageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.FileAssetID = strings.TrimSpace(input.FileAssetID)
	if err := ValidatePageInput(input); err != nil {
		writeStoreError(w, r, err)
		return
	}
	if !h.validatePageFile(w, r, user.TenantID, r.PathValue("id"), input.FileAssetID) {
		return
	}
	out, err := h.store.AddPage(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "submission.page_added", "submission", out.SubmissionID, "add submission page")
	httpx.JSON(w, http.StatusCreated, map[string]any{"page": out})
}

func (h *Handler) ReplacePage(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	pageNo, err := strconv.Atoi(r.PathValue("pageNo"))
	if err != nil || pageNo <= 0 {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_page_no", "pageNo must be a positive integer")
		return
	}
	var input AddPageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.FileAssetID = strings.TrimSpace(input.FileAssetID)
	input.PageNo = pageNo
	if err := ValidatePageInput(input); err != nil {
		writeStoreError(w, r, err)
		return
	}
	if !h.validatePageFile(w, r, user.TenantID, r.PathValue("id"), input.FileAssetID) {
		return
	}
	out, err := h.store.ReplacePage(r.Context(), user.TenantID, r.PathValue("id"), user.ID, pageNo, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "submission.page_replaced", "submission", out.SubmissionID, "replace submission page file")
	httpx.JSON(w, http.StatusOK, map[string]any{"page": out})
}

func (h *Handler) validatePageFile(w http.ResponseWriter, r *http.Request, tenantID string, submissionID string, fileAssetID string) bool {
	asset, err := h.fileStore.Get(r.Context(), tenantID, fileAssetID)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_asset_not_found", "file_asset_id does not exist for current tenant")
		return false
	}
	item, err := h.store.Get(r.Context(), tenantID, submissionID)
	if err != nil {
		writeStoreError(w, r, err)
		return false
	}
	if (asset.SubmissionID != "" && asset.SubmissionID != submissionID) || (asset.ExamID != "" && asset.ExamID != item.ExamID) {
		httpx.Error(w, r, http.StatusBadRequest, "file_asset_scope_mismatch", "file asset does not belong to this submission")
		return false
	}
	return true
}

func (h *Handler) ListPages(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListPages(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"pages": out})
}

func (h *Handler) QualityCheck(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	result, err := h.store.RunQualityCheck(r.Context(), user.TenantID, r.PathValue("id"), user.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "submission.quality_checked", "submission", r.PathValue("id"), "run metadata quality gate")
	httpx.JSON(w, http.StatusOK, map[string]any{"result": result})
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input UpdateStatusInput
	if !decodeJSON(w, r, &input) {
		return
	}
	out, err := h.store.UpdateStatus(r.Context(), user.TenantID, r.PathValue("id"), user.ID, strings.TrimSpace(input.Status))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "submission.status_changed", "submission", out.ID, "change submission status")
	httpx.JSON(w, http.StatusOK, map[string]any{"submission": out})
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
		httpx.Error(w, r, http.StatusNotFound, "submission_not_found", "submission not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_submission", "submission input is invalid")
	case errors.Is(err, ErrDuplicatePage):
		httpx.Error(w, r, http.StatusConflict, "duplicate_page", "submission page already exists")
	case errors.Is(err, ErrInvalidTransition):
		httpx.Error(w, r, http.StatusConflict, "invalid_submission_status_transition", "submission status transition is not allowed")
	case errors.Is(err, ErrSubmissionLocked):
		httpx.Error(w, r, http.StatusConflict, "submission_locked", "submission is locked")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "submission_operation_failed", "submission operation failed")
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

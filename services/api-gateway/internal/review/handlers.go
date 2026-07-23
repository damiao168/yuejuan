package review

import (
	"encoding/json"
	"errors"
	"net/http"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store        Store
	audit        auth.Store
	segmentImage http.HandlerFunc
	fileDownload http.HandlerFunc
}

func NewHandler(store Store, audit auth.Store, segmentImage http.HandlerFunc, fileDownload http.HandlerFunc) *Handler {
	return &Handler{store: store, audit: audit, segmentImage: segmentImage, fileDownload: fileDownload}
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input CreateTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.AssignedTo != "" && !h.authorizeReviewAssignee(w, r, user.TenantID, input.AssignedTo) {
		return
	}
	task, err := h.store.CreateTask(r.Context(), user.TenantID, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.task_created", "review_task", task.ID, "create review task")
	httpx.JSON(w, http.StatusCreated, map[string]any{"task": task})
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	filter := ListFilter{
		Status:     r.URL.Query().Get("status"),
		AssignedTo: r.URL.Query().Get("assigned_to"),
		ExamID:     r.URL.Query().Get("exam_id"),
	}
	if reviewWorkerScoped(user) {
		filter.AssignedTo = user.ID
	}
	tasks, err := h.store.ListTasks(r.Context(), user.TenantID, filter)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	task, err := h.store.GetTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if reviewWorkerScoped(user) && task.AssignedTo != user.ID {
		writeStoreError(w, r, ErrForbidden)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) AssignTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input AssignTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if !h.authorizeReviewAssignee(w, r, user.TenantID, input.AssignedTo) {
		return
	}
	task, err := h.store.AssignTask(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.task_assigned", "review_task", task.ID, "assign review task")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) BatchAssignTasks(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input BatchAssignInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if !h.authorizeReviewAssignee(w, r, user.TenantID, input.AssignedTo) {
		return
	}
	tasks, err := h.store.BatchAssignTasks(r.Context(), user.TenantID, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.tasks_batch_assigned", "review_task", "", "batch assign review tasks")
	httpx.JSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (h *Handler) SubmitGrade(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input SubmitGradeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.store.SubmitGrade(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.human_grade_submitted", "review_task", result.Task.ID, "submit human grade")
	if result.FinalGrade != nil {
		h.auditAction(r, "review.double_mark_auto_finalized", "final_grade", result.FinalGrade.ID, "double mark score difference within threshold")
	}
	if result.ArbitrationTask != nil {
		h.auditAction(r, "arbitration.task_created", "arbitration_task", result.ArbitrationTask.ID, "double mark score difference exceeds threshold")
	}
	response := map[string]any{"task": result.Task, "human_grade": result.Grade}
	if result.QuestionGradeID != "" {
		response["question_grade_id"] = result.QuestionGradeID
	}
	if result.DoubleMarkSession != nil {
		response["double_mark_session"] = result.DoubleMarkSession
	}
	if result.FinalGrade != nil {
		response["final_grade"] = result.FinalGrade
	}
	if result.ArbitrationTask != nil {
		response["arbitration_task"] = result.ArbitrationTask
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) ReturnTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input ReturnTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := h.store.ReturnTask(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.task_returned", "review_task", task.ID, "return review task")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) GetDraft(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeAssignedReviewer(w, r, user) {
		return
	}
	draft, err := h.store.(DraftStore).GetDraft(r.Context(), user.TenantID, r.PathValue("id"), user.ID)
	if errors.Is(err, ErrNotFound) {
		httpx.JSON(w, http.StatusOK, map[string]any{"draft": nil})
		return
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeAssignedReviewer(w, r, user) {
		return
	}
	var input SaveDraftInput
	if !decodeJSON(w, r, &input) {
		return
	}
	draft, err := h.store.(DraftStore).SaveDraft(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.draft_saved", "review_task", draft.ReviewTaskID, "save review draft")
	httpx.JSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) authorizeTaskWorker(w http.ResponseWriter, r *http.Request, user auth.User) bool {
	task, err := h.store.GetTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return false
	}
	if reviewWorkerScoped(user) && task.AssignedTo != user.ID {
		writeStoreError(w, r, ErrForbidden)
		return false
	}
	return true
}

// Drafts are the assigned reviewer's mutable work product. Review managers may
// inspect task context and answer images, but must not read or overwrite a
// reviewer's private draft merely because they can manage the queue.
func (h *Handler) authorizeAssignedReviewer(w http.ResponseWriter, r *http.Request, user auth.User) bool {
	task, err := h.store.GetTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return false
	}
	if task.AssignedTo == "" || task.AssignedTo != user.ID {
		writeStoreError(w, r, ErrForbidden)
		return false
	}
	return true
}

// Queue assignment is deliberately narrower than review:manage: the target
// must be an active grader account in the same tenant.
func (h *Handler) authorizeReviewAssignee(w http.ResponseWriter, r *http.Request, tenantID, assigneeID string) bool {
	return h.authorizeActiveRole(w, r, tenantID, assigneeID, "grader")
}

func (h *Handler) authorizeArbitrationAssignee(w http.ResponseWriter, r *http.Request, tenantID, assigneeID string) bool {
	return h.authorizeActiveRole(w, r, tenantID, assigneeID, "arbitrator")
}

func (h *Handler) authorizeActiveRole(w http.ResponseWriter, r *http.Request, tenantID, userID, roleCode string) bool {
	if userID == "" {
		writeStoreError(w, r, ErrInvalidInput)
		return false
	}
	users, err := h.audit.ListManagedUsers(r.Context(), tenantID)
	if err != nil {
		writeStoreError(w, r, err)
		return false
	}
	for _, candidate := range users {
		if candidate.ID != userID || candidate.Status != "active" {
			continue
		}
		for _, role := range candidate.Roles {
			if role == roleCode {
				return true
			}
		}
		break
	}
	writeStoreError(w, r, ErrForbidden)
	return false
}

func (h *Handler) authorizeReviewManager(w http.ResponseWriter, r *http.Request, user auth.User) bool {
	if reviewManagerScoped(user) {
		return true
	}
	writeStoreError(w, r, ErrForbidden)
	return false
}

func (h *Handler) authorizeArbitrationManager(w http.ResponseWriter, r *http.Request, user auth.User) bool {
	if arbitrationManagerScoped(user) {
		return true
	}
	writeStoreError(w, r, ErrForbidden)
	return false
}

func canViewOriginalReviewImage(user auth.User) bool {
	if !hasAnyRole(user, "platform_admin", "tenant_admin", "school_admin") {
		return false
	}
	return auth.HasPermission(user, "review:manage") ||
		auth.HasPermission(user, "evidence:manage") ||
		auth.HasPermission(user, "tenant:manage")
}

func hasAnyRole(user auth.User, roles ...string) bool {
	for _, current := range user.Roles {
		for _, allowed := range roles {
			if current == allowed {
				return true
			}
		}
	}
	return false
}

func (h *Handler) ClaimNextTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input NextTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	// Pending tasks require an explicit manager assignment; "next" never grants
	// answer access through an implicit self-assignment.
	options := ClaimTaskOptions{}
	task, err := h.store.(WorkbenchStore).ClaimNextTask(r.Context(), user.TenantID, user.ID, input, options)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.task_claimed", "review_task", task.ID, "claim next review task")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeTaskWorker(w, r, user) {
		return
	}
	workspace, err := h.store.(WorkbenchStore).GetWorkspace(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if !canViewOriginalReviewImage(user) {
		workspace.OriginalImageURL = ""
		workspace.OriginalFileID = ""
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"workspace": workspace})
}

func (h *Handler) GetWorkspaceSegmentImage(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeTaskWorker(w, r, user) {
		return
	}
	workspace, err := h.store.(WorkbenchStore).GetWorkspace(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if h.segmentImage == nil {
		httpx.Error(w, r, http.StatusInternalServerError, "review_image_unavailable", "review image handler is unavailable")
		return
	}
	proxyRequest := r.Clone(r.Context())
	proxyRequest.SetPathValue("id", workspace.Task.AnswerSegmentID)
	h.segmentImage(w, proxyRequest)
}

func (h *Handler) GetWorkspaceOriginalImage(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !canViewOriginalReviewImage(user) {
		writeStoreError(w, r, ErrForbidden)
		return
	}
	workspace, err := h.store.(WorkbenchStore).GetWorkspace(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if workspace.OriginalFileID == "" {
		writeStoreError(w, r, ErrNotFound)
		return
	}
	if h.fileDownload == nil {
		httpx.Error(w, r, http.StatusInternalServerError, "review_image_unavailable", "review image handler is unavailable")
		return
	}
	proxyRequest := r.Clone(r.Context())
	proxyRequest.SetPathValue("id", workspace.OriginalFileID)
	h.fileDownload(w, proxyRequest)
}

func (h *Handler) RenewTaskClaim(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if err := h.store.(WorkbenchStore).RenewTaskClaim(r.Context(), user.TenantID, r.PathValue("id"), user.ID); err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"renewed": true})
}

func (h *Handler) ReleaseTaskClaim(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	task, err := h.store.(WorkbenchStore).ReleaseTaskClaim(r.Context(), user.TenantID, r.PathValue("id"), user.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.task_released", "review_task", task.ID, "release review task claim")
	httpx.JSON(w, http.StatusOK, map[string]any{"task": task})
}

func (h *Handler) SetExamDoubleMarkPolicy(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input SetDoubleMarkPolicyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	policy, err := h.store.SetExamDoubleMarkPolicy(r.Context(), user.TenantID, r.PathValue("examId"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.double_mark_policy_set", "exam", policy.ExamID, "set exam double mark policy")
	httpx.JSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func (h *Handler) SetQuestionDoubleMarkPolicy(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input SetDoubleMarkPolicyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	policy, err := h.store.SetQuestionDoubleMarkPolicy(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.double_mark_policy_set", "question", policy.QuestionID, "set question double mark policy")
	httpx.JSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func (h *Handler) ListDoubleMarkPolicies(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	policies, err := h.store.ListDoubleMarkPolicies(r.Context(), user.TenantID, PolicyFilter{
		ExamID:     r.URL.Query().Get("exam_id"),
		QuestionID: r.URL.Query().Get("question_id"),
	})
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"policies": policies})
}

func (h *Handler) CreateDoubleMarkSession(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	var input CreateDoubleMarkSessionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if !h.authorizeReviewAssignee(w, r, user.TenantID, input.FirstReviewerID) {
		return
	}
	if !h.authorizeReviewAssignee(w, r, user.TenantID, input.SecondReviewerID) {
		return
	}
	session, err := h.store.CreateDoubleMarkSession(r.Context(), user.TenantID, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "review.double_mark_session_created", "double_mark_session", session.ID, "create double mark session")
	httpx.JSON(w, http.StatusCreated, map[string]any{"double_mark_session": session})
}

func (h *Handler) ListDoubleMarkSessions(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	sessions, err := h.store.ListDoubleMarkSessions(r.Context(), user.TenantID, DoubleMarkSessionFilter{
		Status:          r.URL.Query().Get("status"),
		AnswerSegmentID: r.URL.Query().Get("answer_segment_id"),
	})
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"double_mark_sessions": sessions})
}

func (h *Handler) GetDoubleMarkSession(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeReviewManager(w, r, user) {
		return
	}
	session, err := h.store.GetDoubleMarkSession(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"double_mark_session": session})
}

func (h *Handler) CreateArbitrationTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeArbitrationManager(w, r, user) {
		return
	}
	var input CreateArbitrationTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.AssignedTo != "" && !h.authorizeArbitrationAssignee(w, r, user.TenantID, input.AssignedTo) {
		return
	}
	task, err := h.store.CreateArbitrationTask(r.Context(), user.TenantID, user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "arbitration.task_created", "arbitration_task", task.ID, "create arbitration task")
	httpx.JSON(w, http.StatusCreated, map[string]any{"arbitration_task": task})
}

func (h *Handler) ListArbitrationTasks(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	filter := ArbitrationFilter{
		Status:     r.URL.Query().Get("status"),
		AssignedTo: r.URL.Query().Get("assigned_to"),
	}
	if arbitrationWorkerScoped(user) {
		filter.AssignedTo = user.ID
	}
	tasks, err := h.store.ListArbitrationTasks(r.Context(), user.TenantID, filter)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"arbitration_tasks": tasks})
}

func (h *Handler) GetArbitrationTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	task, err := h.store.GetArbitrationTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if arbitrationWorkerScoped(user) && task.AssignedTo != user.ID {
		writeStoreError(w, r, ErrForbidden)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"arbitration_task": task})
}

func (h *Handler) AssignArbitrationTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	if !h.authorizeArbitrationManager(w, r, user) {
		return
	}
	var input AssignArbitrationTaskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if !h.authorizeArbitrationAssignee(w, r, user.TenantID, input.AssignedTo) {
		return
	}
	task, err := h.store.AssignArbitrationTask(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "arbitration.task_assigned", "arbitration_task", task.ID, "assign arbitration task")
	httpx.JSON(w, http.StatusOK, map[string]any{"arbitration_task": task})
}

func (h *Handler) SubmitArbitration(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input SubmitArbitrationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	task, err := h.store.GetArbitrationTask(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if task.AssignedTo == "" || task.AssignedTo != user.ID {
		writeStoreError(w, r, ErrForbidden)
		return
	}
	if !h.authorizeArbitrationAssignee(w, r, user.TenantID, user.ID) {
		return
	}
	task, finalGrade, err := h.store.SubmitArbitration(r.Context(), user.TenantID, r.PathValue("id"), user.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "arbitration.submitted", "arbitration_task", task.ID, "submit arbitration final score")
	h.auditAction(r, "final_grade.created", "final_grade", finalGrade.ID, "create final grade from arbitration")
	httpx.JSON(w, http.StatusCreated, map[string]any{"arbitration_task": task, "final_grade": finalGrade})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil {
		return true
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "review_resource_not_found", "review resource not found")
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_review_input", "review input is invalid")
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, r, http.StatusForbidden, "review_action_forbidden", "review action is forbidden")
	case errors.Is(err, ErrInvalidTransition):
		httpx.Error(w, r, http.StatusConflict, "invalid_review_transition", "review task transition is invalid")
	case errors.Is(err, ErrRevisionConflict):
		httpx.Error(w, r, http.StatusConflict, "review_draft_revision_conflict", "review draft was updated in another session")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "review_operation_failed", "review operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func reviewWorkerScoped(user auth.User) bool {
	return !reviewManagerScoped(user)
}

func arbitrationWorkerScoped(user auth.User) bool {
	return !arbitrationManagerScoped(user)
}

func reviewManagerScoped(user auth.User) bool {
	return hasAnyRole(user, "platform_admin", "tenant_admin", "school_admin") && auth.HasPermission(user, "review:manage")
}

func arbitrationManagerScoped(user auth.User) bool {
	return hasAnyRole(user, "platform_admin", "tenant_admin", "school_admin") && auth.HasPermission(user, "arbitration:manage")
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

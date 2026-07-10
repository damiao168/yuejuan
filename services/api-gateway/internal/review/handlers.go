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
	store Store
	audit auth.Store
}

func NewHandler(store Store, audit auth.Store) *Handler {
	return &Handler{store: store, audit: audit}
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input CreateTaskInput
	if !decodeJSON(w, r, &input) {
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
	var input AssignTaskInput
	if !decodeJSON(w, r, &input) {
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
	var input BatchAssignInput
	if !decodeJSON(w, r, &input) {
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

func (h *Handler) SetExamDoubleMarkPolicy(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
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
	var input CreateDoubleMarkSessionInput
	if !decodeJSON(w, r, &input) {
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
	session, err := h.store.GetDoubleMarkSession(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"double_mark_session": session})
}

func (h *Handler) CreateArbitrationTask(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input CreateArbitrationTaskInput
	if !decodeJSON(w, r, &input) {
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
	var input AssignArbitrationTaskInput
	if !decodeJSON(w, r, &input) {
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
	if arbitrationWorkerScoped(user) {
		task, err := h.store.GetArbitrationTask(r.Context(), user.TenantID, r.PathValue("id"))
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
		if task.AssignedTo != user.ID {
			writeStoreError(w, r, ErrForbidden)
			return
		}
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
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
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
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "review_operation_failed", "review operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func reviewWorkerScoped(user auth.User) bool {
	return !auth.HasPermission(user, "review:manage")
}

func arbitrationWorkerScoped(user auth.User) bool {
	return !auth.HasPermission(user, "arbitration:manage")
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

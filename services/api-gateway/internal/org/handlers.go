package org

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	if !isPlatformTenant(r) {
		httpx.Error(w, r, http.StatusForbidden, "platform_tenant_required", "tenant administration requires the platform tenant")
		return
	}
	var input struct {
		Name             string `json:"name"`
		Code             string `json:"code"`
		Status           string `json:"status"`
		AdminUsername    string `json:"admin_username"`
		AdminDisplayName string `json:"admin_display_name"`
		AdminPassword    string `json:"admin_password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.AdminUsername = strings.TrimSpace(input.AdminUsername)
	input.AdminDisplayName = strings.TrimSpace(input.AdminDisplayName)
	if input.Name == "" || input.Code == "" || input.AdminUsername == "" || input.AdminDisplayName == "" || input.AdminPassword == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "school and administrator fields are required")
		return
	}
	if !auth.StrongPassword(input.AdminPassword) {
		httpx.Error(w, r, http.StatusBadRequest, "weak_password", "password must contain upper, lower, number and symbol and be at least 12 characters")
		return
	}
	hash, err := auth.HashPassword(input.AdminPassword)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "password_hash_failed", "failed to create school")
		return
	}
	out, err := h.store.CreateTenant(r.Context(), TenantProvision{
		Name: input.Name, Code: input.Code, Status: input.Status,
		AdminUsername: input.AdminUsername, AdminDisplayName: input.AdminDisplayName,
		PasswordHash: hash,
	})
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "tenant_create_failed", "failed to create tenant")
		return
	}
	h.auditAction(r, "org.tenant_created", "tenant", out.ID, "create tenant")
	httpx.JSON(w, http.StatusCreated, map[string]any{"tenant": out})
}

func (h *Handler) ListTenants(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	canListAll := user.TenantID == auth.PlatformTenantID && hasPermission(user, "tenant:manage")
	out, err := h.store.ListTenants(r.Context(), user.TenantID, canListAll)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "tenant_list_failed", "failed to list tenants")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tenants": out})
}

func (h *Handler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	if !isPlatformTenant(r) {
		httpx.Error(w, r, http.StatusForbidden, "platform_tenant_required", "tenant administration requires the platform tenant")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "status is required")
		return
	}
	out, err := h.store.UpdateTenantStatus(r.Context(), r.PathValue("id"), input.Status)
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "tenant_not_found", "tenant not found")
		return
	}
	h.auditAction(r, "org.tenant_updated", "tenant", out.ID, "update tenant status")
	httpx.JSON(w, http.StatusOK, map[string]any{"tenant": out})
}

func isPlatformTenant(r *http.Request) bool {
	user := mustUser(r)
	return user.TenantID == auth.PlatformTenantID
}

func (h *Handler) CreateSchool(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input School
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name == "" || input.Code == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "name and code are required")
		return
	}
	out, err := h.store.CreateSchool(r.Context(), user.TenantID, input)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "school_create_failed", "failed to create school")
		return
	}
	h.auditAction(r, "org.school_created", "school", out.ID, "create school")
	httpx.JSON(w, http.StatusCreated, map[string]any{"school": out})
}

func (h *Handler) ListSchools(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListSchools(r.Context(), user.TenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "school_list_failed", "failed to list schools")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"schools": out})
}

func (h *Handler) CreateGrade(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input Grade
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.SchoolID == "" || input.Name == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "school_id and name are required")
		return
	}
	out, err := h.store.CreateGrade(r.Context(), user.TenantID, input)
	if err != nil {
		writeStoreError(w, r, err, "grade_create_failed", "failed to create grade")
		return
	}
	h.auditAction(r, "org.grade_created", "grade", out.ID, "create grade")
	httpx.JSON(w, http.StatusCreated, map[string]any{"grade": out})
}

func (h *Handler) ListGrades(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListGrades(r.Context(), user.TenantID, r.URL.Query().Get("school_id"))
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "grade_list_failed", "failed to list grades")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"grades": out})
}

func (h *Handler) CreateClass(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input Class
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.SchoolID == "" || input.GradeID == "" || input.Name == "" || input.Code == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "school_id, grade_id, name and code are required")
		return
	}
	out, err := h.store.CreateClass(r.Context(), user.TenantID, input)
	if err != nil {
		writeStoreError(w, r, err, "class_create_failed", "failed to create class")
		return
	}
	h.auditAction(r, "org.class_created", "class", out.ID, "create class")
	httpx.JSON(w, http.StatusCreated, map[string]any{"class": out})
}

func (h *Handler) ListClasses(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListClasses(r.Context(), user.TenantID, r.URL.Query().Get("grade_id"))
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "class_list_failed", "failed to list classes")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"classes": out})
}

func (h *Handler) CreateStudent(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input Student
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.SchoolID == "" || input.ClassID == "" || input.StudentNo == "" || input.Name == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "school_id, class_id, student_no and name are required")
		return
	}
	out, err := h.store.CreateStudent(r.Context(), user.TenantID, input)
	if err != nil {
		writeStoreError(w, r, err, "student_create_failed", "failed to create student")
		return
	}
	h.auditAction(r, "org.student_created", "student", out.ID, "create student")
	httpx.JSON(w, http.StatusCreated, map[string]any{"student": out})
}

func (h *Handler) ListStudents(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	out, err := h.store.ListStudents(r.Context(), user.TenantID, r.URL.Query().Get("class_id"))
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "student_list_failed", "failed to list students")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"students": out})
}

func (h *Handler) UpdateStudent(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	out, err := h.store.UpdateStudentStatus(r.Context(), user.TenantID, r.PathValue("id"), input.Status)
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "student_not_found", "student not found")
		return
	}
	h.auditAction(r, "org.student_updated", "student", out.ID, "update student status")
	httpx.JSON(w, http.StatusOK, map[string]any{"student": out})
}

func (h *Handler) ImportStudentsCSV(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	reader := csv.NewReader(r.Body)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_csv", "failed to parse csv")
		return
	}
	result := CSVImportResult{Errors: []CSVImportError{}}
	for i, row := range rows {
		if i == 0 && len(row) >= 3 && strings.EqualFold(row[0], "student_no") {
			continue
		}
		if len(row) < 4 {
			result.Errors = append(result.Errors, CSVImportError{Row: i + 1, Message: "expected student_no,name,school_id,class_id"})
			continue
		}
		student := Student{StudentNo: strings.TrimSpace(row[0]), Name: strings.TrimSpace(row[1]), SchoolID: strings.TrimSpace(row[2]), ClassID: strings.TrimSpace(row[3])}
		if student.StudentNo == "" || student.Name == "" || student.SchoolID == "" || student.ClassID == "" {
			result.Errors = append(result.Errors, CSVImportError{Row: i + 1, Message: "student_no, name, school_id and class_id are required"})
			continue
		}
		if _, err := h.store.CreateStudent(r.Context(), user.TenantID, student); err != nil {
			result.Errors = append(result.Errors, CSVImportError{Row: i + 1, Message: err.Error()})
			continue
		}
		result.Created++
	}
	h.auditAction(r, "org.students_imported", "student", "", "import students csv")
	httpx.JSON(w, http.StatusOK, map[string]any{"result": result})
}

func (h *Handler) BindTeacherClass(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	var input struct {
		TeacherID string `json:"teacher_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.TeacherID == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "teacher_id is required")
		return
	}
	classID := r.PathValue("id")
	if err := h.store.BindTeacherClass(r.Context(), user.TenantID, input.TeacherID, classID); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "teacher_bind_failed", "failed to bind teacher to class")
		return
	}
	h.auditAction(r, "org.teacher_bound_to_class", "class", classID, "bind teacher to class")
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "bound"})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error, code string, message string) {
	if errors.Is(err, ErrInvalidParent) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_parent_scope", "parent resource does not belong to tenant")
		return
	}
	httpx.Error(w, r, http.StatusInternalServerError, code, message)
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func hasPermission(user auth.User, permission string) bool {
	for _, current := range user.Permissions {
		if current == permission {
			return true
		}
	}
	return false
}

func (h *Handler) auditAction(r *http.Request, action string, targetType string, targetID string, reason string) {
	user := mustUser(r)
	auth.RecordAudit(r.Context(), h.audit, auth.AuditEvent{
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

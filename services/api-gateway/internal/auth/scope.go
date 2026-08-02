package auth

import (
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

const WorkerTenantHeader = "X-EduGrade-Tenant-ID"

// IsPlatformWorker identifies the platform-owned service account that may
// process background jobs for school tenants. Product users never receive this
// role, even when they are platform administrators.
func IsPlatformWorker(user User) bool {
	return user.TenantID == PlatformTenantID && HasRole(user, "page_processing_worker")
}

// PlatformWorkerTenantScope lets the platform worker operate on the tenant
// returned by a global task claim. The actor id is cleared because platform
// service users do not belong to the target tenant; worker identity remains on
// the durable runtime task and its attempt records.
func PlatformWorkerTenantScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetTenantID := strings.TrimSpace(r.Header.Get(WorkerTenantHeader))
		if targetTenantID == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, ok := UserFromContext(r.Context())
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		if !IsPlatformWorker(user) || targetTenantID == PlatformTenantID {
			httpx.Error(w, r, http.StatusForbidden, "worker_tenant_scope_forbidden", "worker tenant scope is not allowed")
			return
		}
		user.ID = ""
		user.TenantID = targetTenantID
		user.TenantCode = ""
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

// ScopedStudentID returns the student id from either a flat data_scope used by
// memory tests or a role-keyed data_scope returned by the PostgreSQL RBAC join.
func ScopedStudentID(user User) (string, bool) {
	return studentIDFromScope(user.DataScope)
}

func studentIDFromScope(scope map[string]any) (string, bool) {
	if len(scope) == 0 {
		return "", false
	}
	if value, ok := scope["student_id"]; ok {
		if studentID, ok := value.(string); ok && studentID != "" {
			return studentID, true
		}
	}
	for _, value := range scope {
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if studentID, ok := studentIDFromScope(nested); ok {
			return studentID, true
		}
	}
	return "", false
}

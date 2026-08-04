package auth

// IsPlatformWorker identifies the platform-owned service account that may
// process background jobs for school tenants. Product users never receive this
// role, even when they are platform administrators.
func IsPlatformWorker(user User) bool {
	return user.TenantID == PlatformTenantID && HasRole(user, "page_processing_worker")
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

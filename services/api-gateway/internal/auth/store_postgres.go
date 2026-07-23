package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) FindUserByLogin(ctx context.Context, tenantCode string, username string) (UserWithPassword, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
  u.id::text,
  u.tenant_id::text,
  t.code,
  u.username,
  u.display_name,
  u.status,
  u.password_hash,
  COALESCE(array_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '{}') AS roles,
  COALESCE(array_agg(DISTINCT p.code) FILTER (WHERE p.code IS NOT NULL), '{}') AS permissions,
  COALESCE(jsonb_object_agg(r.code, ur.data_scope) FILTER (WHERE r.code IS NOT NULL), '{}') AS data_scope
FROM app_user u
JOIN tenant t ON t.id = u.tenant_id
LEFT JOIN user_role ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id AND ur.deleted_at IS NULL
LEFT JOIN role r ON r.tenant_id = u.tenant_id AND r.id = ur.role_id AND r.deleted_at IS NULL
LEFT JOIN role_permission rp ON rp.tenant_id = u.tenant_id AND rp.role_id = r.id AND rp.deleted_at IS NULL
LEFT JOIN permission p ON p.tenant_id = u.tenant_id AND p.id = rp.permission_id AND p.deleted_at IS NULL
WHERE t.code = $1
  AND u.username = $2
  AND t.status = 'active'
  AND t.deleted_at IS NULL
  AND u.deleted_at IS NULL
GROUP BY u.id, t.code
`, tenantCode, username)

	var user UserWithPassword
	var roles []string
	var permissions []string
	var dataScopeRaw []byte
	if err := row.Scan(&user.ID, &user.TenantID, &user.TenantCode, &user.Username, &user.DisplayName, &user.Status, &user.PasswordHash, pqArray(&roles), pqArray(&permissions), &dataScopeRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserWithPassword{}, ErrInvalidCredentials
		}
		return UserWithPassword{}, err
	}
	user.Roles = roles
	user.Permissions = permissions
	user.DataScope = map[string]any{}
	_ = json.Unmarshal(dataScopeRaw, &user.DataScope)
	if user.Status != "active" {
		return UserWithPassword{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, tenantID string, userID string, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO auth_session (tenant_id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
`, tenantID, userID, tokenHash, expiresAt)
	return err
}

func (s *PostgresStore) FindUserBySession(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
  u.id::text,
  u.tenant_id::text,
  t.code,
  u.username,
  u.display_name,
  u.status,
  COALESCE(array_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '{}') AS roles,
  COALESCE(array_agg(DISTINCT p.code) FILTER (WHERE p.code IS NOT NULL), '{}') AS permissions,
  COALESCE(jsonb_object_agg(r.code, ur.data_scope) FILTER (WHERE r.code IS NOT NULL), '{}') AS data_scope
FROM auth_session s
JOIN app_user u ON u.tenant_id = s.tenant_id AND u.id = s.user_id
JOIN tenant t ON t.id = u.tenant_id
LEFT JOIN user_role ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id AND ur.deleted_at IS NULL
LEFT JOIN role r ON r.tenant_id = u.tenant_id AND r.id = ur.role_id AND r.deleted_at IS NULL
LEFT JOIN role_permission rp ON rp.tenant_id = u.tenant_id AND rp.role_id = r.id AND rp.deleted_at IS NULL
LEFT JOIN permission p ON p.tenant_id = u.tenant_id AND p.id = rp.permission_id AND p.deleted_at IS NULL
WHERE s.token_hash = $1
  AND s.expires_at > $2
  AND s.revoked_at IS NULL
  AND t.status = 'active'
  AND t.deleted_at IS NULL
  AND u.deleted_at IS NULL
GROUP BY u.id, t.code
`, tokenHash, now)

	var user User
	var roles []string
	var permissions []string
	var dataScopeRaw []byte
	if err := row.Scan(&user.ID, &user.TenantID, &user.TenantCode, &user.Username, &user.DisplayName, &user.Status, pqArray(&roles), pqArray(&permissions), &dataScopeRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUnauthenticated
		}
		return User{}, err
	}
	user.Roles = roles
	user.Permissions = permissions
	user.DataScope = map[string]any{}
	_ = json.Unmarshal(dataScopeRaw, &user.DataScope)
	if user.Status != "active" {
		return User{}, ErrUnauthenticated
	}
	return user, nil
}

func (s *PostgresStore) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE auth_session
SET revoked_at = now(), updated_at = now()
WHERE token_hash = $1 AND revoked_at IS NULL
`, tokenHash)
	return err
}

func (s *PostgresStore) ActiveAdminExists(ctx context.Context, tenantCode string, roleCode string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM app_user u
  JOIN tenant t ON t.id = u.tenant_id
  JOIN user_role ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id AND ur.deleted_at IS NULL
  JOIN role r ON r.tenant_id = u.tenant_id AND r.id = ur.role_id AND r.deleted_at IS NULL
  WHERE t.code = $1
    AND r.code = $2
    AND u.status = 'active'
    AND u.deleted_at IS NULL
)
`, tenantCode, roleCode)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PostgresStore) UpsertBootstrapAdmin(ctx context.Context, input BootstrapAdminInput, passwordHash string) (BootstrapAdminResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BootstrapAdminResult{}, err
	}
	defer tx.Rollback()

	var tenantID string
	if err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM tenant
WHERE code = $1 AND deleted_at IS NULL
`, input.TenantCode).Scan(&tenantID); err != nil {
		return BootstrapAdminResult{}, err
	}

	var roleID string
	if err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM role
WHERE tenant_id::text = $1 AND code = $2 AND deleted_at IS NULL
`, tenantID, input.RoleCode).Scan(&roleID); err != nil {
		return BootstrapAdminResult{}, err
	}

	var userID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO app_user (tenant_id, username, display_name, password_hash, status, deleted_at)
VALUES ($1::uuid, $2, $3, $4, 'active', NULL)
ON CONFLICT (tenant_id, username) DO UPDATE
SET display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    status = 'active',
    deleted_at = NULL,
    updated_at = now()
RETURNING id::text
`, tenantID, input.Username, input.DisplayName, passwordHash).Scan(&userID); err != nil {
		return BootstrapAdminResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope, deleted_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, jsonb_build_object('scope', 'platform'), NULL)
ON CONFLICT (tenant_id, user_id, role_id) DO UPDATE
SET data_scope = EXCLUDED.data_scope,
    deleted_at = NULL,
    updated_at = now()
`, tenantID, userID, roleID); err != nil {
		return BootstrapAdminResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO audit_log (tenant_id, actor_id, action, target_type, target_id, after_value, reason)
VALUES ($1::uuid, NULL, 'auth.bootstrap_admin_created', 'user', $2::uuid, $3::jsonb, 'bootstrap initial admin')
`, tenantID, userID, auditJSON(map[string]any{
		"username":    input.Username,
		"role_code":   input.RoleCode,
		"tenant_code": input.TenantCode,
	})); err != nil {
		return BootstrapAdminResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return BootstrapAdminResult{}, err
	}
	return BootstrapAdminResult{
		TenantID:   tenantID,
		TenantCode: input.TenantCode,
		UserID:     userID,
		Username:   input.Username,
		RoleCode:   input.RoleCode,
	}, nil
}

func (s *PostgresStore) Audit(ctx context.Context, event AuditEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO audit_log (tenant_id, actor_id, action, target_type, target_id, before_value, after_value, reason, ip_address, user_agent, request_id)
VALUES ($1, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, '')::uuid, $6::jsonb, $7::jsonb, $8, $9, $10, $11)
`, event.TenantID, event.ActorID, event.Action, event.TargetType, event.TargetID, auditJSON(event.BeforeValue), auditJSON(event.AfterValue), event.Reason, event.IPAddress, event.UserAgent, event.RequestID)
	return err
}

func (s *PostgresStore) ListAudits(ctx context.Context, tenantID string, filter AuditFilter) ([]AuditRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
  id::text,
  tenant_id::text,
  COALESCE(actor_id::text, ''),
  action,
  target_type,
  COALESCE(target_id::text, ''),
  COALESCE(before_value::text, ''),
  COALESCE(after_value::text, ''),
  COALESCE(reason, ''),
  COALESCE(ip_address, ''),
  COALESCE(user_agent, ''),
  COALESCE(request_id, ''),
  created_at
FROM audit_log
WHERE tenant_id = $1
  AND ($2 = '' OR action = $2)
  AND ($3 = '' OR COALESCE(actor_id::text, '') = $3)
  AND ($4 = '' OR target_type = $4)
  AND ($5 = '' OR COALESCE(target_id::text, '') = $5)
  AND ($6 = '' OR COALESCE(target_id::text, '') = $6)
  AND ($7 = '' OR COALESCE(ip_address, '') = $7)
  AND ($8::timestamptz IS NULL OR created_at >= $8::timestamptz)
  AND ($9::timestamptz IS NULL OR created_at <= $9::timestamptz)
ORDER BY created_at DESC
LIMIT $10
`, tenantID, stringsTrim(filter.Action), stringsTrim(filter.ActorID), stringsTrim(filter.TargetType), stringsTrim(filter.TargetID), stringsTrim(filter.ExamID), stringsTrim(filter.IPAddress), nullableTime(filter.CreatedFrom), nullableTime(filter.CreatedTo), normalizedAuditLimit(filter.Limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditRecord{}
	for rows.Next() {
		var record AuditRecord
		beforeValue := ""
		afterValue := ""
		if err := rows.Scan(
			&record.ID,
			&record.TenantID,
			&record.ActorID,
			&record.Action,
			&record.TargetType,
			&record.TargetID,
			&beforeValue,
			&afterValue,
			&record.Reason,
			&record.IPAddress,
			&record.UserAgent,
			&record.RequestID,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		record.BeforeValue = parseAuditJSON(beforeValue)
		record.AfterValue = parseAuditJSON(afterValue)
		out = append(out, record)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListManagedUsers(ctx context.Context, tenantID string) ([]ManagedUser, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT u.id::text, u.username, u.display_name, u.status,
       COALESCE(array_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '{}'),
       u.created_at
FROM app_user u
LEFT JOIN user_role ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id AND ur.deleted_at IS NULL
LEFT JOIN role r ON r.tenant_id = u.tenant_id AND r.id = ur.role_id AND r.deleted_at IS NULL
WHERE u.tenant_id = $1::uuid AND u.deleted_at IS NULL
GROUP BY u.id
ORDER BY u.created_at, u.username
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ManagedUser{}
	for rows.Next() {
		var user ManagedUser
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status, pqArray(&user.Roles), &user.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListAssignableRoles(ctx context.Context, tenantID string) ([]AssignableRole, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT code, name, scope_type, COALESCE(description, '')
FROM role
WHERE tenant_id = $1::uuid AND deleted_at IS NULL AND code NOT IN ('platform_admin', 'tenant_admin')
ORDER BY name, code
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssignableRole{}
	for rows.Next() {
		var role AssignableRole
		if err := rows.Scan(&role.Code, &role.Name, &role.ScopeType, &role.Description); err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateManagedUser(ctx context.Context, tenantID string, tenantCode string, input CreateManagedUserInput, passwordHash string) (ManagedUser, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ManagedUser{}, err
	}
	defer tx.Rollback()

	var roleID string
	if err := tx.QueryRowContext(ctx, `
SELECT id::text FROM role
WHERE tenant_id = $1::uuid AND code = $2 AND code NOT IN ('platform_admin', 'tenant_admin') AND deleted_at IS NULL
`, tenantID, input.RoleCode).Scan(&roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ManagedUser{}, ErrRoleNotFound
		}
		return ManagedUser{}, err
	}

	var user ManagedUser
	err = tx.QueryRowContext(ctx, `
INSERT INTO app_user (tenant_id, username, display_name, password_hash, status)
VALUES ($1::uuid, $2, $3, $4, 'active')
RETURNING id::text, username, display_name, status, created_at
`, tenantID, input.Username, input.DisplayName, passwordHash).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status, &user.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return ManagedUser{}, ErrUsernameExists
		}
		return ManagedUser{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
VALUES ($1::uuid, $2::uuid, $3::uuid, jsonb_build_object('scope', 'tenant'))
`, tenantID, user.ID, roleID); err != nil {
		return ManagedUser{}, err
	}
	if err := tx.Commit(); err != nil {
		return ManagedUser{}, err
	}
	user.Roles = []string{input.RoleCode}
	return user, nil
}

func auditJSON(value map[string]any) any {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return string(data)
}

func parseAuditJSON(value string) map[string]any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil
	}
	return out
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

// pqArray scans a PostgreSQL text array in the simple {"a","b"} form returned by lib/pq-compatible drivers.
func pqArray(target *[]string) any {
	return scannerFunc(func(src any) error {
		if src == nil {
			*target = nil
			return nil
		}
		text := ""
		switch value := src.(type) {
		case string:
			text = value
		case []byte:
			text = string(value)
		default:
			return nil
		}
		text = strings.Trim(text, "{}")
		if text == "" {
			*target = []string{}
			return nil
		}
		parts := strings.Split(text, ",")
		for i := range parts {
			parts[i] = strings.Trim(parts[i], `"`)
		}
		*target = parts
		return nil
	})
}

type scannerFunc func(src any) error

func (f scannerFunc) Scan(src any) error {
	return f(src)
}

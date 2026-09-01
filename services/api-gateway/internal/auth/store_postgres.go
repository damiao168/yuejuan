package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/logger"
)

var sessionTouchFailureLogger = logger.New(os.Stderr, "error")

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

func (s *PostgresStore) CreateSession(ctx context.Context, input CreateSessionInput) (DeviceSession, error) {
	var session DeviceSession
	err := s.db.QueryRowContext(ctx, `
INSERT INTO auth_session (
  tenant_id, user_id, token_hash, session_type, device_id, device_name,
  user_agent_hash, ip_prefix, expires_at, last_seen_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
RETURNING id::text, session_type, device_name, created_at, last_seen_at, expires_at
`, input.TenantID, input.UserID, input.TokenHash, input.SessionType, input.DeviceID,
		input.DeviceName, input.UserAgentHash, input.IPPrefix, input.ExpiresAt,
	).Scan(&session.ID, &session.SessionType, &session.DeviceName, &session.CreatedAt, &session.LastSeenAt, &session.ExpiresAt)
	session.Current = true
	return session, err
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
	cutoff := now.Add(-5 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `
UPDATE auth_session
SET last_seen_at = $2, updated_at = $2
WHERE token_hash = $1
  AND last_seen_at < $3
`, tokenHash, now, cutoff); err != nil {
		sessionTouchFailureLogger.Warn(ctx, "auth_session_touch_failed", map[string]any{"error": err.Error()})
	}
	return user, nil
}

func (s *PostgresStore) ResolveAccessScope(ctx context.Context, user User) (AccessScope, error) {
	scope, err := ResolveDeclaredAccessScope(user)
	if err != nil {
		return AccessScope{}, err
	}
	if scope.IsPlatform || scope.TenantWide {
		return scope, nil
	}

	kinds := declaredRelationshipKinds(user.DataScope)
	if kinds["school"] {
		var schoolIDs, gradeIDs, classIDs, examIDs []string
		err = s.db.QueryRowContext(ctx, `
WITH scoped_school AS (
  SELECT u.school_id::text AS id FROM app_user u
  WHERE u.tenant_id=$1::uuid AND u.id=$2::uuid AND u.school_id IS NOT NULL AND u.deleted_at IS NULL
  UNION SELECT unnest(string_to_array(NULLIF($3,''), ','))
)
SELECT
  COALESCE((SELECT array_agg(DISTINCT id) FROM scoped_school WHERE id<>''), '{}'),
  COALESCE((SELECT array_agg(DISTINCT g.id::text) FROM grade g JOIN scoped_school ss ON ss.id=g.school_id::text WHERE g.tenant_id=$1::uuid AND g.deleted_at IS NULL), '{}'),
  COALESCE((SELECT array_agg(DISTINCT c.id::text) FROM school_class c JOIN scoped_school ss ON ss.id=c.school_id::text WHERE c.tenant_id=$1::uuid AND c.deleted_at IS NULL), '{}'),
  COALESCE((SELECT array_agg(DISTINCT e.id::text) FROM exam e JOIN scoped_school ss ON ss.id=e.school_id::text WHERE e.tenant_id=$1::uuid AND e.deleted_at IS NULL), '{}')
`, user.TenantID, user.ID, strings.Join(scope.SchoolIDs, ",")).Scan(pqArray(&schoolIDs), pqArray(&gradeIDs), pqArray(&classIDs), pqArray(&examIDs))
		if err != nil {
			return AccessScope{}, err
		}
		scope.SchoolIDs = append(scope.SchoolIDs, schoolIDs...)
		scope.GradeIDs = append(scope.GradeIDs, gradeIDs...)
		scope.ClassIDs = append(scope.ClassIDs, classIDs...)
		scope.ExamIDs = append(scope.ExamIDs, examIDs...)
	}
	if kinds["class"] {
		var schoolIDs, gradeIDs, classIDs, examIDs []string
		err = s.db.QueryRowContext(ctx, `
WITH scoped_class AS (
  SELECT tc.class_id FROM teacher_class tc
  WHERE tc.tenant_id=$1::uuid AND tc.teacher_id=$2::uuid AND tc.deleted_at IS NULL
  UNION SELECT unnest(string_to_array(NULLIF($3,''), ','))::uuid
)
SELECT COALESCE(array_agg(DISTINCT c.school_id::text), '{}'),
       COALESCE(array_agg(DISTINCT c.grade_id::text), '{}'),
       COALESCE(array_agg(DISTINCT c.id::text), '{}'),
       COALESCE(array_agg(DISTINCT ec.exam_id::text) FILTER (WHERE ec.exam_id IS NOT NULL), '{}')
FROM scoped_class sc
JOIN school_class c ON c.tenant_id=$1::uuid AND c.id=sc.class_id AND c.deleted_at IS NULL
LEFT JOIN exam_class ec ON ec.tenant_id=c.tenant_id AND ec.class_id=c.id AND ec.deleted_at IS NULL
`, user.TenantID, user.ID, strings.Join(scope.ClassIDs, ",")).Scan(pqArray(&schoolIDs), pqArray(&gradeIDs), pqArray(&classIDs), pqArray(&examIDs))
		if err != nil {
			return AccessScope{}, err
		}
		scope.SchoolIDs = append(scope.SchoolIDs, schoolIDs...)
		scope.GradeIDs = append(scope.GradeIDs, gradeIDs...)
		scope.ClassIDs = append(scope.ClassIDs, classIDs...)
		scope.ExamIDs = append(scope.ExamIDs, examIDs...)
	}
	if kinds["assigned"] || kinds["exam_task"] {
		var reviewTaskIDs, arbitrationTaskIDs, examIDs, submissionIDs []string
		err = s.db.QueryRowContext(ctx, `
SELECT
  COALESCE((SELECT array_agg(rt.id::text) FROM review_task rt WHERE rt.tenant_id=$1::uuid AND rt.assigned_to=$2::uuid AND rt.status IN ('assigned','in_progress','returned') AND rt.deleted_at IS NULL), '{}'),
  COALESCE((SELECT array_agg(at.id::text) FROM arbitration_task at WHERE at.tenant_id=$1::uuid AND at.assigned_to=$2::uuid AND at.status IN ('assigned','in_progress') AND at.deleted_at IS NULL), '{}'),
  COALESCE((SELECT array_agg(DISTINCT x.exam_id) FROM (SELECT rt.exam_id::text exam_id FROM review_task rt WHERE rt.tenant_id=$1::uuid AND rt.assigned_to=$2::uuid AND rt.status IN ('assigned','in_progress','returned') AND rt.deleted_at IS NULL UNION SELECT at.exam_id::text FROM arbitration_task at WHERE at.tenant_id=$1::uuid AND at.assigned_to=$2::uuid AND at.status IN ('assigned','in_progress') AND at.deleted_at IS NULL) x), '{}'),
  COALESCE((SELECT array_agg(DISTINCT x.submission_id) FROM (SELECT rt.submission_id::text submission_id FROM review_task rt WHERE rt.tenant_id=$1::uuid AND rt.assigned_to=$2::uuid AND rt.status IN ('assigned','in_progress','returned') AND rt.deleted_at IS NULL UNION SELECT at.submission_id::text FROM arbitration_task at WHERE at.tenant_id=$1::uuid AND at.assigned_to=$2::uuid AND at.status IN ('assigned','in_progress') AND at.deleted_at IS NULL) x), '{}')
`, user.TenantID, user.ID).Scan(pqArray(&reviewTaskIDs), pqArray(&arbitrationTaskIDs), pqArray(&examIDs), pqArray(&submissionIDs))
		if err != nil {
			return AccessScope{}, err
		}
		scope.ReviewTaskIDs = append(scope.ReviewTaskIDs, reviewTaskIDs...)
		scope.ArbitrationTaskIDs = append(scope.ArbitrationTaskIDs, arbitrationTaskIDs...)
		scope.ExamIDs = append(scope.ExamIDs, examIDs...)
		scope.SubmissionIDs = append(scope.SubmissionIDs, submissionIDs...)
	}
	return scope.normalized(), nil
}

func declaredRelationshipKinds(dataScope map[string]any) map[string]bool {
	out := map[string]bool{}
	var visit func(map[string]any)
	visit = func(scope map[string]any) {
		if kind, ok := scope["scope"].(string); ok {
			out[strings.TrimSpace(kind)] = true
		}
		for _, raw := range scope {
			if nested, ok := raw.(map[string]any); ok {
				visit(nested)
			}
		}
	}
	visit(dataScope)
	return out
}

func (s *PostgresStore) DeleteSession(ctx context.Context, tokenHash string, reason string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE auth_session
SET revoked_at = now(), revoke_reason = NULLIF($2, ''), updated_at = now()
WHERE token_hash = $1 AND revoked_at IS NULL
`, tokenHash, reason)
	return err
}

func (s *PostgresStore) ListSessions(ctx context.Context, tenantID string, userID string, currentTokenHash string, now time.Time) ([]DeviceSession, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_type, device_name, created_at, last_seen_at, expires_at,
       token_hash = $4
FROM auth_session
WHERE tenant_id = $1
  AND user_id = $2
  AND revoked_at IS NULL
  AND expires_at > $3
ORDER BY last_seen_at DESC, created_at DESC
`, tenantID, userID, now, currentTokenHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceSession{}
	for rows.Next() {
		var session DeviceSession
		if err := rows.Scan(&session.ID, &session.SessionType, &session.DeviceName, &session.CreatedAt, &session.LastSeenAt, &session.ExpiresAt, &session.Current); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *PostgresStore) RevokeSession(ctx context.Context, tenantID string, userID string, sessionID string, reason string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE auth_session
SET revoked_at = now(), revoke_reason = NULLIF($4, ''), updated_at = now()
WHERE tenant_id = $1
  AND user_id = $2
  AND id = $3
  AND revoked_at IS NULL
`, tenantID, userID, sessionID, reason)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *PostgresStore) RevokeAllSessions(ctx context.Context, tenantID string, userID string, reason string) (int, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE auth_session
SET revoked_at = now(), revoke_reason = NULLIF($3, ''), updated_at = now()
WHERE tenant_id = $1
  AND user_id = $2
  AND revoked_at IS NULL
`, tenantID, userID, reason)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (s *PostgresStore) UpdatePasswordAndRevokeSessions(ctx context.Context, tenantID, userID, expectedPasswordHash, newPasswordHash string) (bool, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
UPDATE app_user
SET password_hash=$4, updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND password_hash=$3 AND status='active' AND deleted_at IS NULL
`, tenantID, userID, expectedPasswordHash, newPasswordHash)
	if err != nil {
		return false, 0, err
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return false, 0, err
	}
	result, err = tx.ExecContext(ctx, `
UPDATE auth_session
SET revoked_at=now(), revoke_reason='password_changed', updated_at=now()
WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL
`, tenantID, userID)
	if err != nil {
		return false, 0, err
	}
	revoked, err := result.RowsAffected()
	if err != nil {
		return false, 0, err
	}
	if err = tx.Commit(); err != nil {
		return false, 0, err
	}
	return true, int(revoked), nil
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
	if input.TenantCode != "platform" || input.RoleCode != "platform_admin" {
		return BootstrapAdminResult{}, ErrInvalidBootstrapInput
	}
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

func (s *PostgresStore) ProvisionStory060User(ctx context.Context, username, displayName, roleCode, passwordHash string) error {
	expectedScope := map[string]string{"school_admin": "school", "subjective_grading_worker": "service"}
	scope, ok := expectedScope[roleCode]
	if !ok {
		return ErrStory060ProvisioningInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tenantID, roleID, actualScope string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM tenant WHERE code='platform' AND deleted_at IS NULL`).Scan(&tenantID); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id::text,scope_type FROM role WHERE tenant_id=$1::uuid AND code=$2 AND deleted_at IS NULL`, tenantID, roleCode).Scan(&roleID, &actualScope); err != nil {
		return err
	}
	if actualScope != scope {
		return ErrStory060ProvisioningInput
	}
	var userID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO app_user (tenant_id,username,display_name,password_hash,status,deleted_at)
VALUES ($1::uuid,$2,$3,$4,'active',NULL)
ON CONFLICT (tenant_id,username) DO UPDATE SET display_name=EXCLUDED.display_name,password_hash=EXCLUDED.password_hash,status='active',deleted_at=NULL,updated_at=now()
RETURNING id::text
`, tenantID, username, displayName, passwordHash).Scan(&userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_role (tenant_id,user_id,role_id,data_scope,deleted_at)
VALUES ($1::uuid,$2::uuid,$3::uuid,jsonb_build_object('scope',$4),NULL)
ON CONFLICT (tenant_id,user_id,role_id) DO UPDATE SET data_scope=EXCLUDED.data_scope,deleted_at=NULL,updated_at=now()
`, tenantID, userID, roleID, scope); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) Audit(ctx context.Context, event AuditEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO audit_log (tenant_id, actor_id, action, target_type, target_id, before_value, after_value, reason, ip_address, user_agent, request_id)
VALUES ($1, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, '')::uuid, $6::jsonb, $7::jsonb, $8, $9, $10, $11)
`, event.TenantID, event.ActorID, event.Action, event.TargetType, event.TargetID, auditJSON(event.BeforeValue), auditJSON(event.AfterValue), event.Reason, event.IPAddress, event.UserAgent, event.RequestID)
	return err
}

func (s *PostgresStore) ListAudits(ctx context.Context, tenantID string, filter AuditFilter) ([]AuditRecord, error) {
	if filter.ScopeMode != "" && filter.ScopeMode != "tenant" && filter.ScopeMode != "platform" {
		return []AuditRecord{}, nil
	}
	if filter.ScopeMode != "" && filter.ScopeMode != "tenant" && filter.ScopeMode != "platform" {
		return []AuditRecord{}, nil
	}
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
  AND ($11 = '' OR created_at < $10 OR (created_at = $10 AND id::text < $11))
ORDER BY created_at DESC, id::text DESC
LIMIT $12
`, tenantID, stringsTrim(filter.Action), stringsTrim(filter.ActorID), stringsTrim(filter.TargetType), stringsTrim(filter.TargetID), stringsTrim(filter.ExamID), stringsTrim(filter.IPAddress), nullableTime(filter.CreatedFrom), nullableTime(filter.CreatedTo), nullableTime(filter.CursorCreatedAt), stringsTrim(filter.CursorID), normalizedAuditLimit(filter.Limit))
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

func (s *PostgresStore) ListManagedUsers(ctx context.Context, tenantID string, filter ManagedUserFilter) ([]ManagedUser, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT u.id::text, u.username, u.display_name, u.status,
       COALESCE(array_agg(DISTINCT r.code) FILTER (WHERE r.code IS NOT NULL), '{}'),
	   COALESCE(u.school_id::text, ''),
       u.created_at
FROM app_user u
LEFT JOIN user_role ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id AND ur.deleted_at IS NULL
LEFT JOIN role r ON r.tenant_id = u.tenant_id AND r.id = ur.role_id AND r.deleted_at IS NULL
WHERE u.tenant_id = $1::uuid AND u.deleted_at IS NULL
  AND ($2 = '' OR u.id::text = $2)
  AND ($3 = '' OR u.username ILIKE '%' || $3 || '%' OR u.display_name ILIKE '%' || $3 || '%')
  AND ($4 = '' OR EXISTS (
    SELECT 1 FROM user_role fur JOIN role fr ON fr.tenant_id=fur.tenant_id AND fr.id=fur.role_id AND fr.deleted_at IS NULL
    WHERE fur.tenant_id=u.tenant_id AND fur.user_id=u.id AND fur.deleted_at IS NULL AND fr.code=$4
  ))
  AND ($6 = '' OR u.created_at < $5 OR (u.created_at = $5 AND u.id::text < $6))
  AND (NOT $8 OR u.school_id::text = ANY(string_to_array($9, ',')))
GROUP BY u.id
ORDER BY u.created_at DESC, u.id::text DESC
LIMIT NULLIF($7, 0)
`, tenantID, filter.UserID, filter.Query, filter.Role, filter.CursorCreatedAt, filter.CursorID, filter.Limit, filter.RestrictSchools, strings.Join(filter.SchoolIDs, ","))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ManagedUser{}
	for rows.Next() {
		var user ManagedUser
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status, pqArray(&user.Roles), &user.SchoolID, &user.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListAssignableRoles(ctx context.Context, actor User) ([]AssignableRole, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT code, name, scope_type, COALESCE(description, '')
FROM role
WHERE tenant_id = $1::uuid
  AND deleted_at IS NULL
  AND code NOT IN ('platform_admin', 'tenant_admin')
ORDER BY name, code
`, actor.TenantID)
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
		if policy := RolePolicy(role.Code); CanAssignManagedRole(actor, role.Code) && role.ScopeType == policy.CanonicalScope {
			out = append(out, role)
		}
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateManagedUser(ctx context.Context, actor User, actorScope AccessScope, input CreateManagedUserInput, passwordHash string) (ManagedUser, error) {
	policy := RolePolicy(input.RoleCode)
	if !CanAssignManagedRole(actor, input.RoleCode) {
		return ManagedUser{}, ErrRoleAssignment
	}
	if len(input.ClassIDs) > 0 && !policy.ClassBinding {
		return ManagedUser{}, ErrInvalidRoleBinding
	}
	schoolID := strings.TrimSpace(input.SchoolID)
	if schoolID == "" && HasRole(actor, "school_admin") && len(actorScope.SchoolIDs) == 1 {
		schoolID = actorScope.SchoolIDs[0]
	}
	if policy.SchoolRequired && schoolID == "" {
		return ManagedUser{}, ErrInvalidRoleBinding
	}
	if schoolID != "" && !actorScope.AllowsSchool(schoolID) {
		return ManagedUser{}, ErrOrganizationScope
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ManagedUser{}, err
	}
	defer tx.Rollback()

	var roleID, roleScopeType string
	if err := tx.QueryRowContext(ctx, `
SELECT id::text, scope_type FROM role
WHERE tenant_id = $1::uuid AND code = $2 AND deleted_at IS NULL
`, actor.TenantID, input.RoleCode).Scan(&roleID, &roleScopeType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ManagedUser{}, ErrRoleNotFound
		}
		return ManagedUser{}, err
	}
	if roleScopeType != policy.CanonicalScope {
		return ManagedUser{}, ErrRoleAssignment
	}
	if schoolID != "" {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM school WHERE tenant_id=$1::uuid AND id::text=$2 AND deleted_at IS NULL)`, actor.TenantID, schoolID).Scan(&exists); err != nil {
			return ManagedUser{}, err
		}
		if !exists {
			return ManagedUser{}, ErrInvalidRoleBinding
		}
	}
	if len(input.ClassIDs) > 0 {
		var count int
		var minSchoolID, maxSchoolID string
		if err := tx.QueryRowContext(ctx, `
SELECT count(*), COALESCE(min(school_id::text), ''), COALESCE(max(school_id::text), '')
FROM school_class
WHERE tenant_id=$1::uuid AND id::text=ANY(string_to_array($2, ',')) AND deleted_at IS NULL
`, actor.TenantID, strings.Join(input.ClassIDs, ",")).Scan(&count, &minSchoolID, &maxSchoolID); err != nil {
			return ManagedUser{}, err
		}
		if count != len(input.ClassIDs) || minSchoolID == "" || minSchoolID != maxSchoolID || minSchoolID != schoolID {
			return ManagedUser{}, ErrInvalidRoleBinding
		}
		for _, classID := range input.ClassIDs {
			if !actorScope.TenantWide && !actorScope.AllowsSchool(schoolID) && !actorScope.AllowsClass(classID) {
				return ManagedUser{}, ErrOrganizationScope
			}
		}
	}

	var user ManagedUser
	err = tx.QueryRowContext(ctx, `
INSERT INTO app_user (tenant_id, school_id, username, display_name, password_hash, status)
VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5, 'active')
RETURNING id::text, username, display_name, status, COALESCE(school_id::text,''), created_at
`, actor.TenantID, schoolID, input.Username, input.DisplayName, passwordHash).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status, &user.SchoolID, &user.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return ManagedUser{}, ErrUsernameExists
		}
		return ManagedUser{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_role (tenant_id, user_id, role_id, data_scope)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4::jsonb)
`, actor.TenantID, user.ID, roleID, auditJSON(canonicalRoleDataScope(input.RoleCode, schoolID))); err != nil {
		return ManagedUser{}, err
	}
	if len(input.ClassIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO teacher_class (tenant_id, teacher_id, class_id)
SELECT $1::uuid, $2::uuid, value::uuid FROM unnest(string_to_array($3, ',')) AS value
ON CONFLICT (tenant_id, teacher_id, class_id) DO UPDATE SET deleted_at=NULL, updated_at=now()
`, actor.TenantID, user.ID, strings.Join(input.ClassIDs, ",")); err != nil {
			return ManagedUser{}, err
		}
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

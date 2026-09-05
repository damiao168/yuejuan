package modelgovernance

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

func (s *PostgresStore) ListManagedAPIConfigs(ctx context.Context, tenantID string) ([]ManagedAPIConfig, error) {
	rows, err := s.db.QueryContext(ctx, managedAPIConfigSelect+`
WHERE tenant_id=$1::uuid AND deleted_at IS NULL
ORDER BY is_default DESC, updated_at DESC, provider_key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ManagedAPIConfig{}
	for rows.Next() {
		item, scanErr := scanManagedAPIConfig(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) CreateManagedAPIConfig(ctx context.Context, tenantID, actorID string, input ManagedAPIConfigInput) (ManagedAPIConfig, error) {
	if s.credentialCipher == nil {
		return ManagedAPIConfig{}, ErrManagedConfigUnavailable
	}
	input.TenantID = tenantID
	normalized, err := normalizeManagedAPIInput(input, true)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	id := uuid.NewString()
	ciphertext, nonce, err := s.credentialCipher.Encrypt(normalized.APIKey, tenantID, id)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	defer tx.Rollback()
	if normalized.IsDefault {
		if _, err = tx.ExecContext(ctx, `UPDATE managed_model_api_config SET is_default=false,updated_at=now() WHERE tenant_id=$1::uuid AND is_default AND deleted_at IS NULL`, tenantID); err != nil {
			return ManagedAPIConfig{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO managed_model_api_config(
  id,tenant_id,provider_key,display_name,adapter_type,base_url,model_name,model_version,region,
  credential_ciphertext,credential_nonce,credential_hint,status,is_default,created_by
)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid)
RETURNING id::text,tenant_id::text,provider_key,display_name,adapter_type,base_url,model_name,model_version,region,
          true,credential_hint,status,is_default,last_test_status,last_test_message,last_tested_at,created_at,updated_at
`, id, tenantID, normalized.ProviderKey, normalized.DisplayName, normalized.AdapterType,
		normalized.BaseURL, normalized.ModelName, normalized.ModelVersion, normalized.Region,
		ciphertext, nonce, credentialHint(normalized.APIKey), normalized.Status, normalized.IsDefault, actorID)
	item, err := scanManagedAPIConfig(row)
	if err != nil {
		return ManagedAPIConfig{}, mapStoreError(err)
	}
	if err = tx.Commit(); err != nil {
		return ManagedAPIConfig{}, err
	}
	return item, nil
}

func (s *PostgresStore) UpdateManagedAPIConfig(ctx context.Context, tenantID, id string, input ManagedAPIConfigUpdateInput) (ManagedAPIConfig, error) {
	if s.credentialCipher == nil {
		return ManagedAPIConfig{}, ErrManagedConfigUnavailable
	}
	normalized, err := normalizeManagedAPIUpdate(input)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ManagedAPIConfig{}, err
	}
	defer tx.Rollback()
	if normalized.IsDefault {
		if _, err = tx.ExecContext(ctx, `UPDATE managed_model_api_config SET is_default=false,updated_at=now() WHERE tenant_id=$1::uuid AND id<>$2::uuid AND is_default AND deleted_at IS NULL`, tenantID, id); err != nil {
			return ManagedAPIConfig{}, err
		}
	}
	var ciphertext, nonce []byte
	if normalized.APIKey != "" {
		ciphertext, nonce, err = s.credentialCipher.Encrypt(normalized.APIKey, tenantID, id)
		if err != nil {
			return ManagedAPIConfig{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `
UPDATE managed_model_api_config
SET display_name=$3,adapter_type=$4,base_url=$5,model_name=$6,model_version=$7,region=$8,
    credential_ciphertext=CASE WHEN $9::bytea IS NULL THEN credential_ciphertext ELSE $9::bytea END,
    credential_nonce=CASE WHEN $10::bytea IS NULL THEN credential_nonce ELSE $10::bytea END,
    credential_hint=CASE WHEN $9::bytea IS NULL THEN credential_hint ELSE $11 END,
    status=$12,is_default=$13,
    last_test_status=CASE WHEN $9::bytea IS NULL AND base_url=$5 THEN last_test_status ELSE 'untested' END,
    last_test_message=CASE WHEN $9::bytea IS NULL AND base_url=$5 THEN last_test_message ELSE '' END,
    last_tested_at=CASE WHEN $9::bytea IS NULL AND base_url=$5 THEN last_tested_at ELSE NULL END,
    updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL
RETURNING id::text,tenant_id::text,provider_key,display_name,adapter_type,base_url,model_name,model_version,region,
          true,credential_hint,status,is_default,last_test_status,last_test_message,last_tested_at,created_at,updated_at
`, tenantID, id, normalized.DisplayName, normalized.AdapterType, normalized.BaseURL,
		normalized.ModelName, normalized.ModelVersion, normalized.Region,
		nullBytes(ciphertext), nullBytes(nonce), credentialHint(normalized.APIKey), normalized.Status, normalized.IsDefault)
	item, err := scanManagedAPIConfig(row)
	if err != nil {
		return ManagedAPIConfig{}, mapStoreError(err)
	}
	if err = tx.Commit(); err != nil {
		return ManagedAPIConfig{}, err
	}
	return item, nil
}

func (s *PostgresStore) GetManagedAPIConnection(ctx context.Context, tenantID, id string) (ManagedAPIConnection, error) {
	if s.credentialCipher == nil {
		return ManagedAPIConnection{}, ErrManagedConfigUnavailable
	}
	row := s.db.QueryRowContext(ctx, managedAPIConfigSelect+` WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL`, tenantID, id)
	item, err := scanManagedAPIConfig(row)
	if err != nil {
		return ManagedAPIConnection{}, mapStoreError(err)
	}
	var ciphertext, nonce []byte
	if err = s.db.QueryRowContext(ctx, `SELECT credential_ciphertext,credential_nonce FROM managed_model_api_config WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL`, tenantID, id).Scan(&ciphertext, &nonce); err != nil {
		return ManagedAPIConnection{}, mapStoreError(err)
	}
	apiKey, err := s.credentialCipher.Decrypt(ciphertext, nonce, tenantID, id)
	if err != nil {
		return ManagedAPIConnection{}, err
	}
	return ManagedAPIConnection{Config: item, APIKey: apiKey}, nil
}

func (s *PostgresStore) RecordManagedAPIProbe(ctx context.Context, tenantID, id string, result ManagedAPIProbeResult) (ManagedAPIConfig, error) {
	status := "failed"
	if result.OK {
		status = "success"
	}
	message := strings.TrimSpace(result.Message)
	if len(message) > 256 {
		message = message[:256]
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE managed_model_api_config
SET last_test_status=$3,last_test_message=$4,last_tested_at=now(),updated_at=now()
WHERE tenant_id=$1::uuid AND id=$2::uuid AND deleted_at IS NULL
RETURNING id::text,tenant_id::text,provider_key,display_name,adapter_type,base_url,model_name,model_version,region,
          true,credential_hint,status,is_default,last_test_status,last_test_message,last_tested_at,created_at,updated_at
`, tenantID, id, status, message)
	item, err := scanManagedAPIConfig(row)
	if err != nil {
		return ManagedAPIConfig{}, mapStoreError(err)
	}
	return item, nil
}

const managedAPIConfigSelect = `
SELECT id::text,tenant_id::text,provider_key,display_name,adapter_type,base_url,model_name,model_version,region,
       true,credential_hint,status,is_default,last_test_status,last_test_message,last_tested_at,created_at,updated_at
FROM managed_model_api_config
`

func scanManagedAPIConfig(row rowScanner) (ManagedAPIConfig, error) {
	var item ManagedAPIConfig
	if err := row.Scan(
		&item.ID, &item.TenantID, &item.ProviderKey, &item.DisplayName, &item.AdapterType,
		&item.BaseURL, &item.ModelName, &item.ModelVersion, &item.Region,
		&item.CredentialConfigured, &item.CredentialHint, &item.Status, &item.IsDefault,
		&item.LastTestStatus, &item.LastTestMessage, &item.LastTestedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return ManagedAPIConfig{}, err
	}
	return item, nil
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

var _ ManagedAPIConfigStore = (*PostgresStore)(nil)

package modelgovernance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestDefaultManagedAPIResolutionIsTenantScoped(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	input := ManagedAPIConfigInput{ProviderKey: "provider", DisplayName: "School API", AdapterType: "openai_compatible", BaseURL: "https://provider.test/v1", APIKey: "synthetic-secret-123456", ModelName: "school-model", ModelVersion: "v1", Region: "global", Status: "active", IsDefault: true}
	a, err := store.CreateManagedAPIConfig(ctx, "school-a", "actor", input)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := ResolveDefaultManagedAPI(ctx, store, "school-a")
	if err != nil || connection == nil || connection.Config.ID != a.ID {
		t.Fatalf("default not resolved: %v", err)
	}
	connection, err = ResolveDefaultManagedAPI(ctx, store, "school-b")
	if err != nil || connection != nil {
		t.Fatal("school B must not inherit school A's model")
	}
	_, err = store.UpdateManagedAPIConfig(ctx, "school-a", a.ID, ManagedAPIConfigUpdateInput{DisplayName: input.DisplayName, AdapterType: input.AdapterType, BaseURL: input.BaseURL, ModelName: input.ModelName, ModelVersion: input.ModelVersion, Region: input.Region, Status: "disabled", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ResolveDefaultManagedAPI(ctx, store, "school-a"); err == nil {
		t.Fatal("disabled default must not silently fall back")
	}
}

func TestCredentialCipherUsesTenantAndConfigBoundAuthenticatedEncryption(t *testing.T) {
	cipher, err := NewCredentialCipher("test-managed-model-credential-key-with-at-least-32-characters")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, nonce, err := cipher.Encrypt("sk-secret-value-at-least-16", "tenant-a", "config-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ciphertext), "sk-secret") || len(nonce) != 12 {
		t.Fatal("credential was not encrypted with an independent GCM nonce")
	}
	plaintext, err := cipher.Decrypt(ciphertext, nonce, "tenant-a", "config-a")
	if err != nil || plaintext != "sk-secret-value-at-least-16" {
		t.Fatalf("decrypt failed: %q %v", plaintext, err)
	}
	if _, err := cipher.Decrypt(ciphertext, nonce, "tenant-b", "config-a"); err == nil {
		t.Fatal("ciphertext must not decrypt in another tenant")
	}
}

func TestManagedAPIHandlersAssignPerTenantWithoutEchoingKey(t *testing.T) {
	store := NewMemoryStore()
	audits := auth.NewMemoryStore()
	handler := NewHandler(store, audits, NewEnvironmentSecretResolver(t.TempDir()), testBaseline()).
		WithManagedAPIProber(staticManagedProber{result: ManagedAPIProbeResult{OK: true, StatusCode: 200, LatencyMS: 12, Message: "连接成功，密钥可用"}})
	user := auth.User{
		ID: "00000000-0000-0000-0000-000000000010", TenantID: auth.PlatformTenantID,
		Roles: []string{"platform_admin"}, Permissions: []string{"model:provider:manage"},
	}
	tenantID := "00000000-0000-0000-0000-000000000020"
	body := `{
      "tenant_id":"` + tenantID + `","provider_key":"school-provider","display_name":"学校专用 API",
      "adapter_type":"openai_compatible","base_url":"https://models.example.test/v1",
      "model_name":"school-model","model_version":"school-model-v1","region":"cn",
      "api_key":"sk-secret-value-at-least-16","status":"active","is_default":true
    }`
	created := performHandlerRequest(t, user, http.MethodPost, "/api/v1/platform/model-api-configs", body, handler.CreateManagedAPIConfig)
	if created.Code != http.StatusCreated {
		t.Fatalf("create returned %d: %s", created.Code, created.Body.String())
	}
	if strings.Contains(created.Body.String(), "sk-secret") || !strings.Contains(created.Body.String(), `"credential_hint":"•••• t-16"`) {
		t.Fatalf("response exposed a credential or omitted its safe hint: %s", created.Body.String())
	}
	items, err := store.ListManagedAPIConfigs(context.Background(), tenantID)
	if err != nil || len(items) != 1 || items[0].TenantID != tenantID || !items[0].IsDefault {
		t.Fatalf("unexpected tenant assignment: %#v %v", items, err)
	}
	probeRequest := httptest.NewRequest(http.MethodPost,
		"/api/v1/platform/model-api-configs/"+items[0].ID+"/probe?tenant_id="+tenantID, nil)
	probeRequest.SetPathValue("id", items[0].ID)
	probeRequest = probeRequest.WithContext(auth.WithUser(probeRequest.Context(), user))
	probe := httptest.NewRecorder()
	handler.ProbeManagedAPIConfig(probe, probeRequest)
	if probe.Code != http.StatusOK || strings.Contains(probe.Body.String(), "sk-secret") {
		t.Fatalf("probe returned %d or exposed credential: %s", probe.Code, probe.Body.String())
	}
	records, err := audits.ListAudits(context.Background(), user.TenantID, auth.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		raw, _ := json.Marshal(record.AfterValue)
		if strings.Contains(string(raw), "sk-secret") {
			t.Fatalf("audit exposed credential: %#v", record)
		}
	}
}

func TestManagedAPIHandlersRejectCrossTenantSchoolAdmin(t *testing.T) {
	handler := NewHandler(NewMemoryStore(), nil, NewEnvironmentSecretResolver(t.TempDir()), testBaseline())
	user := auth.User{ID: "school-admin", TenantID: "00000000-0000-0000-0000-000000000020", Permissions: []string{"model:provider:manage"}}
	response := performHandlerRequest(t, user, http.MethodGet,
		"/api/v1/platform/model-api-configs?tenant_id=00000000-0000-0000-0000-000000000030", "", handler.ListManagedAPIConfigs)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant list returned %d: %s", response.Code, response.Body.String())
	}
}

type staticManagedProber struct {
	result ManagedAPIProbeResult
}

func (p staticManagedProber) Probe(_ context.Context, _ ManagedAPIConnection) ManagedAPIProbeResult {
	return p.result
}

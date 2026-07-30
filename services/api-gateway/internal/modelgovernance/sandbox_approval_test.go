package modelgovernance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

func TestSandboxApprovalValidationAndEvidence(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	input := validSandboxApprovalInput("provider-1", "deployment-1", now)
	if err := ValidateSandboxApprovalInput(input, now); err != nil {
		t.Fatal(err)
	}
	input.ContractReviewed = false
	if !errors.Is(ValidateSandboxApprovalInput(input, now), ErrInvalidApproval) {
		t.Fatal("incomplete approval facts must fail closed")
	}

	approval := SandboxApproval{
		Protocol:              SandboxProtocolDashScopeNative,
		ApprovalReference:     "approval-061b1c1",
		ApprovedRegion:        "cn-beijing",
		SandboxAccount:        true,
		ContractReviewed:      true,
		RetentionReviewed:     true,
		DataResidencyReviewed: true,
		PricingReviewed:       true,
		SyntheticDataOnly:     true,
		ImageExportReviewed:   true,
		ExpiresAt:             now.Add(time.Hour),
	}
	if !approval.IsActive(now) || approval.Evidence().ApprovalReference != approval.ApprovalReference {
		t.Fatalf("approval did not produce active admission evidence: %#v", approval)
	}
}

func TestMemorySandboxApprovalIsTenantBoundImmutableAndRevocable(t *testing.T) {
	store := NewMemoryStore()
	tenantID := "tenant-1"
	provider, deployment := seedSandboxInventory(t, store, tenantID)
	input := validSandboxApprovalInput(provider.ID, deployment.ID, time.Now().UTC())

	created, err := store.CreateSandboxApproval(context.Background(), tenantID, "actor", input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ProviderKey != provider.Key || created.DeploymentKey != deployment.Key {
		t.Fatalf("approval was not bound to inventory: %#v", created)
	}
	if _, err := store.CreateSandboxApproval(context.Background(), tenantID, "actor", input); !errors.Is(err, ErrConflict) {
		t.Fatalf("second active approval must conflict: %v", err)
	}
	if _, err := store.RevokeSandboxApproval(context.Background(), "tenant-2", "actor", created.ID, "wrong tenant"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant revoke must be not found: %v", err)
	}
	revoked, err := store.RevokeSandboxApproval(context.Background(), tenantID, "actor", created.ID, "sandbox window closed")
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("approval was not revoked: %#v %v", revoked, err)
	}
	if _, err := store.CreateSandboxApproval(context.Background(), tenantID, "actor", input); err != nil {
		t.Fatalf("new approval after explicit revocation failed: %v", err)
	}
	items, err := store.ListSandboxApprovals(context.Background(), tenantID)
	if err != nil || len(items) != 2 {
		t.Fatalf("unexpected approval history: %#v %v", items, err)
	}
}

func TestSandboxApprovalHandlersUseStrictJSONAuditAndTenantScope(t *testing.T) {
	store := NewMemoryStore()
	audits := auth.NewMemoryStore()
	tenantID := "tenant-1"
	provider, deployment := seedSandboxInventory(t, store, tenantID)
	handler := NewHandler(store, audits, NewEnvironmentSecretResolver(t.TempDir()), testBaseline())
	user := auth.User{
		ID:          "actor-1",
		TenantID:    tenantID,
		Permissions: []string{"model:read", "model:provider:manage"},
	}
	input := validSandboxApprovalInput(provider.ID, deployment.ID, time.Now().UTC())
	body, _ := json.Marshal(input)
	response := performHandlerRequest(
		t, user, http.MethodPost, "/api/v1/model-sandbox-approvals", string(body), handler.CreateSandboxApproval,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("create approval returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Approval SandboxApproval `json:"approval"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Approval.ID == "" || strings.Contains(response.Body.String(), input.Reason) {
		t.Fatalf("approval response is invalid or echoed reason: %s", response.Body.String())
	}

	list := performHandlerRequest(
		t, user, http.MethodGet, "/api/v1/model-sandbox-approvals", "", handler.ListSandboxApprovals,
	)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), payload.Approval.ID) {
		t.Fatalf("list approval returned %d: %s", list.Code, list.Body.String())
	}

	revokeRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/model-sandbox-approvals/"+payload.Approval.ID+"/revoke",
		strings.NewReader(`{"reason":"approval withdrawn"}`),
	)
	revokeRequest.SetPathValue("id", payload.Approval.ID)
	revokeRequest = revokeRequest.WithContext(auth.WithUser(revokeRequest.Context(), user))
	revokeResponse := httptest.NewRecorder()
	handler.RevokeSandboxApproval(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK || !strings.Contains(revokeResponse.Body.String(), "revoked_at") {
		t.Fatalf("revoke approval returned %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}

	records, err := audits.ListAudits(context.Background(), tenantID, auth.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(records)
	for _, action := range []string{"model.sandbox_approval_created", "model.sandbox_approval_revoked"} {
		if !bytes.Contains(raw, []byte(action)) {
			t.Fatalf("missing approval audit action %s: %s", action, raw)
		}
	}
}

func TestSandboxApprovalHandlerRejectsUnknownAndIncompleteFacts(t *testing.T) {
	handler := NewHandler(NewMemoryStore(), nil, NewEnvironmentSecretResolver(t.TempDir()), testBaseline())
	user := auth.User{ID: "actor", TenantID: "tenant-1", Permissions: []string{"model:provider:manage"}}
	unknown := `{"provider_id":"p","deployment_id":"d","unknown":true}`
	response := performHandlerRequest(
		t, user, http.MethodPost, "/api/v1/model-sandbox-approvals", unknown, handler.CreateSandboxApproval,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field returned %d: %s", response.Code, response.Body.String())
	}
}

func seedSandboxInventory(t *testing.T, store *MemoryStore, tenantID string) (Provider, Deployment) {
	t.Helper()
	provider, err := store.CreateProvider(context.Background(), tenantID, "", ProviderInput{
		Key:           "dashscope-sandbox",
		DisplayName:   "DashScope sandbox",
		Kind:          ProviderExternal,
		AdapterType:   SandboxProtocolDashScopeNative,
		CredentialRef: "vault://edugrade/dashscope-sandbox",
		Region:        "cn-beijing",
		DataPolicy:    DataPolicy{RetentionMode: "no_store"},
		Status:        "unverified",
	})
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := store.CreateDeployment(context.Background(), tenantID, "", DeploymentInput{
		ProviderID:        provider.ID,
		Key:               "qwen-synthetic-shadow",
		ModelName:         "Qwen synthetic shadow",
		ModelVersion:      "pinned-synthetic-version",
		Region:            provider.Region,
		CapabilityProfile: "synthetic-text-v1",
		Modalities:        []string{"text", "image"},
		PricingPolicy:     map[string]any{"meter": "tokens"},
		Status:            "unverified",
		HealthState:       "unverified",
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider, deployment
}

func validSandboxApprovalInput(providerID string, deploymentID string, now time.Time) SandboxApprovalInput {
	return SandboxApprovalInput{
		ProviderID:            providerID,
		DeploymentID:          deploymentID,
		Protocol:              SandboxProtocolDashScopeNative,
		ApprovalReference:     fmt.Sprintf("approval-%d", now.UnixNano()),
		ApprovedRegion:        "cn-beijing",
		SandboxAccount:        true,
		ContractReviewed:      true,
		RetentionReviewed:     true,
		DataResidencyReviewed: true,
		PricingReviewed:       true,
		SyntheticDataOnly:     true,
		ImageExportReviewed:   false,
		ExpiresAt:             now.Add(30 * 24 * time.Hour),
		Reason:                "approve bounded synthetic sandbox preparation",
	}
}

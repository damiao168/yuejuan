package modelgovernance

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store    Store
	audit    auth.Store
	secrets  SecretReferenceResolver
	baseline LocalBaseline
}

func NewHandler(store Store, audit auth.Store, secrets SecretReferenceResolver, baseline LocalBaseline) *Handler {
	return &Handler{store: store, audit: audit, secrets: secrets, baseline: baseline}
}

func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok || !h.ensureBaseline(w, r, tenantID) {
		return
	}
	items, err := h.store.ListProviders(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_provider_list_failed", "failed to list model providers")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"providers": items})
}

func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var input ProviderInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	tenantID, ok := h.targetTenant(w, r, input.TenantID)
	if !ok {
		return
	}
	provider := ProviderFromInput(input)
	if provider.Status == "" {
		provider.Status = "unverified"
		input.Status = provider.Status
	}
	if strings.TrimSpace(input.DisplayName) == "" || ValidateProvider(provider) != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_provider", "invalid model provider configuration")
		return
	}
	if provider.Kind == ProviderExternal {
		probe, err := h.secrets.Probe(provider.CredentialRef)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_secret_reference", "credential_ref must be a supported secret reference URI")
			return
		}
		if provider.Status != "unverified" && provider.Status != "disabled" {
			httpx.Error(w, r, http.StatusConflict, "external_provider_not_verified", "external providers cannot be activated before native adapter verification")
			return
		}
		if provider.Status != "disabled" && probe.ResolverSupported && !probe.Configured {
			httpx.Error(w, r, http.StatusConflict, "secret_not_configured", "the referenced secret is not configured")
			return
		}
	}
	user := mustUser(r)
	actorID := user.ID
	if tenantID != user.TenantID {
		actorID = ""
	}
	item, err := h.store.CreateProvider(r.Context(), tenantID, actorID, input)
	if err != nil {
		writeStoreError(w, r, err, "model_provider_create_failed", "failed to create model provider")
		return
	}
	h.auditAction(r, "model.provider_created", "model_provider", item.ID, input.Reason(),
		map[string]any{"tenant_id": tenantID, "provider_key": item.Key, "provider_kind": item.Kind, "adapter_type": item.AdapterType, "region": item.Region, "status": item.Status})
	httpx.JSON(w, http.StatusCreated, map[string]any{"provider": item})
}

func (h *Handler) UpdateProviderStatus(w http.ResponseWriter, r *http.Request) {
	var input ProviderStatusInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Reason) == "" || !validProviderStatus(input.Status) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_provider_status", "valid status and reason are required")
		return
	}
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	providers, err := h.store.ListProviders(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_provider_list_failed", "failed to inspect model provider")
		return
	}
	current, exists := providerByID(providers, r.PathValue("id"))
	if !exists {
		httpx.Error(w, r, http.StatusNotFound, "model_provider_not_found", "model provider not found")
		return
	}
	if current.Kind == ProviderExternal && input.Status != "unverified" && input.Status != "disabled" {
		httpx.Error(w, r, http.StatusConflict, "external_provider_not_verified", "external providers cannot be activated before native adapter verification")
		return
	}
	item, err := h.store.UpdateProviderStatus(r.Context(), tenantID, current.ID, input)
	if err != nil {
		writeStoreError(w, r, err, "model_provider_update_failed", "failed to update model provider")
		return
	}
	h.auditAction(r, "model.provider_status_updated", "model_provider", item.ID, input.Reason,
		map[string]any{"tenant_id": tenantID, "provider_key": item.Key, "before_status": current.Status, "status": item.Status})
	httpx.JSON(w, http.StatusOK, map[string]any{"provider": item})
}

func (h *Handler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok || !h.ensureBaseline(w, r, tenantID) {
		return
	}
	items, err := h.store.ListDeployments(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_deployment_list_failed", "failed to list model deployments")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"deployments": items})
}

func (h *Handler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var input DeploymentInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	tenantID, ok := h.targetTenant(w, r, input.TenantID)
	if !ok {
		return
	}
	if input.Status == "" {
		input.Status = "unverified"
	}
	if input.HealthState == "" {
		input.HealthState = "unverified"
	}
	providers, err := h.store.ListProviders(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_provider_list_failed", "failed to inspect model provider")
		return
	}
	provider, exists := providerByID(providers, input.ProviderID)
	if !exists {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_provider", "provider_id does not belong to the target tenant")
		return
	}
	deployment := DeploymentFromInput(input, provider.Key)
	if strings.TrimSpace(input.ModelName) == "" ||
		ValidateDeployment(deployment, provider) != nil ||
		len(input.PricingPolicy) == 0 ||
		strings.TrimSpace(stringValue(input.PricingPolicy["meter"])) == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_deployment", "invalid model deployment configuration")
		return
	}
	if provider.Kind == ProviderExternal &&
		(input.Status != "unverified" || input.HealthState != "unverified") {
		httpx.Error(w, r, http.StatusConflict, "external_deployment_not_verified", "external deployments must remain unverified until native adapter acceptance")
		return
	}
	user := mustUser(r)
	actorID := user.ID
	if tenantID != user.TenantID {
		actorID = ""
	}
	item, err := h.store.CreateDeployment(r.Context(), tenantID, actorID, input)
	if err != nil {
		writeStoreError(w, r, err, "model_deployment_create_failed", "failed to create model deployment")
		return
	}
	h.auditAction(r, "model.deployment_created", "model_deployment", item.ID, "create governed model deployment",
		map[string]any{"tenant_id": tenantID, "provider_key": item.ProviderKey, "deployment_key": item.Key, "region": item.Region, "status": item.Status, "health_state": item.HealthState})
	httpx.JSON(w, http.StatusCreated, map[string]any{"deployment": item})
}

func (h *Handler) UpdateDeploymentState(w http.ResponseWriter, r *http.Request) {
	var input DeploymentStateInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Reason) == "" || !validDeploymentStatus(input.Status) || !validHealthState(input.HealthState) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_deployment_state", "valid status, health_state and reason are required")
		return
	}
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	deployments, err := h.store.ListDeployments(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_deployment_list_failed", "failed to inspect model deployment")
		return
	}
	current, exists := deploymentByID(deployments, r.PathValue("id"))
	if !exists {
		httpx.Error(w, r, http.StatusNotFound, "model_deployment_not_found", "model deployment not found")
		return
	}
	providers, err := h.store.ListProviders(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_provider_list_failed", "failed to inspect model provider")
		return
	}
	provider, exists := providerByID(providers, current.ProviderID)
	if !exists {
		httpx.Error(w, r, http.StatusConflict, "model_provider_not_found", "deployment provider is unavailable")
		return
	}
	externalStateAllowed := input.Status == "unverified" && input.HealthState == "unverified" ||
		input.Status == "disabled" &&
			(input.HealthState == "unverified" || input.HealthState == "unavailable" || input.HealthState == "disabled")
	if provider.Kind == ProviderExternal && !externalStateAllowed {
		httpx.Error(w, r, http.StatusConflict, "external_deployment_not_verified", "external deployments cannot enter routing before native adapter acceptance")
		return
	}
	item, err := h.store.UpdateDeploymentState(r.Context(), tenantID, current.ID, input)
	if err != nil {
		writeStoreError(w, r, err, "model_deployment_update_failed", "failed to update model deployment")
		return
	}
	h.auditAction(r, "model.deployment_state_updated", "model_deployment", item.ID, input.Reason,
		map[string]any{"tenant_id": tenantID, "deployment_key": item.Key, "before_status": current.Status, "status": item.Status, "before_health_state": current.HealthState, "health_state": item.HealthState})
	httpx.JSON(w, http.StatusOK, map[string]any{"deployment": item})
}

func (h *Handler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok || !h.ensureBaseline(w, r, tenantID) {
		return
	}
	item, err := h.store.GetPolicy(r.Context(), tenantID)
	if err != nil {
		writeStoreError(w, r, err, "model_policy_read_failed", "failed to read model policy")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"policy": item})
}

func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	var input PolicyUpdateInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if input.AllowedDeployments == nil {
		input.AllowedDeployments = []string{}
	}
	policy := PolicyFromUpdate(input)
	if strings.TrimSpace(input.Reason) == "" || input.ExpectedVersion < 1 || ValidateTenantPolicy(policy) != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_policy", "valid policy, expected_version and reason are required")
		return
	}
	deployments, err := h.store.ListDeployments(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "model_deployment_list_failed", "failed to validate allowed deployments")
		return
	}
	if !deploymentKeysExist(input.AllowedDeployments, deployments) {
		httpx.Error(w, r, http.StatusBadRequest, "unknown_model_deployment", "allowed_deployments contains an unknown tenant deployment")
		return
	}
	item, err := h.store.UpdatePolicy(r.Context(), tenantID, mustUser(r).ID, input)
	if err != nil {
		writeStoreError(w, r, err, "model_policy_update_failed", "failed to update model policy")
		return
	}
	h.auditAction(r, "model.policy_updated", "tenant_model_policy", item.ID, input.Reason,
		map[string]any{"tenant_id": tenantID, "mode": item.Mode, "external_enabled": item.ExternalEnabled, "text_export_enabled": item.TextExportEnabled, "image_export_enabled": item.ImageExportEnabled, "allowed_deployments": item.AllowedDeployments, "version": item.Version})
	httpx.JSON(w, http.StatusOK, map[string]any{"policy": item})
}

func (h *Handler) ProbeSecret(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CredentialRef string `json:"credential_ref"`
	}
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	probe, err := h.secrets.Probe(input.CredentialRef)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_secret_reference", "invalid secret reference")
		return
	}
	h.auditAction(r, "model.secret_reference_probed", "secret_reference", "", "probe secret reference without reading or returning its value",
		map[string]any{"scheme": probe.Scheme, "resolver_supported": probe.ResolverSupported, "configured": probe.Configured})
	httpx.JSON(w, http.StatusOK, map[string]any{"probe": probe})
}

func (h *Handler) ensureBaseline(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	if err := h.store.EnsureLocalBaseline(r.Context(), tenantID, h.baseline); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "local_model_registry_failed", "failed to register local model baseline")
		return false
	}
	return true
}

func (h *Handler) targetTenant(w http.ResponseWriter, r *http.Request, requested string) (string, bool) {
	user := mustUser(r)
	requested = strings.TrimSpace(requested)
	if requested == "" || requested == user.TenantID {
		return user.TenantID, true
	}
	if user.TenantID == auth.PlatformTenantID && hasPermission(user, "model:provider:manage") {
		return requested, true
	}
	httpx.Error(w, r, http.StatusForbidden, "tenant_scope_forbidden", "target tenant is outside the caller scope")
	return "", false
}

func (h *Handler) auditAction(r *http.Request, action string, targetType string, targetID string, reason string, after map[string]any) {
	user := mustUser(r)
	auth.RecordAudit(r.Context(), h.audit, auth.AuditEvent{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		AfterValue: after,
		Reason:     reason,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		RequestID:  logger.RequestID(r.Context()),
	})
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "request body must contain exactly one json object")
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidProvider), errors.Is(err, ErrInvalidDeployment), errors.Is(err, ErrInvalidPolicy):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_model_governance_request", "invalid model governance request")
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "model_governance_not_found", "model governance resource not found")
	case errors.Is(err, ErrConflict):
		httpx.Error(w, r, http.StatusConflict, "model_governance_conflict", "model governance resource conflict")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, code, message)
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func hasPermission(user auth.User, expected string) bool {
	for _, permission := range user.Permissions {
		if permission == expected {
			return true
		}
	}
	return false
}

func providerByID(items []Provider, id string) (Provider, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return Provider{}, false
}

func deploymentByID(items []Deployment, id string) (Deployment, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return Deployment{}, false
}

func deploymentKeysExist(expected []string, items []Deployment) bool {
	available := make(map[string]bool, len(items))
	for _, item := range items {
		available[item.Key] = true
	}
	for _, key := range expected {
		if !available[key] {
			return false
		}
	}
	return true
}

func validProviderStatus(value string) bool {
	return value == "unverified" || value == "active" || value == "degraded" || value == "rate_limited" || value == "disabled"
}

func validDeploymentStatus(value string) bool {
	return value == "unverified" || value == "shadow_only" || value == "disabled"
}

func validHealthState(value string) bool {
	return value == "unverified" || value == "available" || value == "degraded" || value == "rate_limited" || value == "unavailable" || value == "disabled"
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func (input ProviderInput) Reason() string {
	if input.Kind == ProviderExternal {
		return "register external provider metadata and secret reference"
	}
	return "register local provider metadata"
}

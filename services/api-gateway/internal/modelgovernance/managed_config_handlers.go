package modelgovernance

import (
	"errors"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
)

func (h *Handler) WithManagedAPIProber(prober ManagedAPIProber) *Handler {
	h.managedAPIProber = prober
	return h
}

func (h *Handler) ListManagedAPIConfigs(w http.ResponseWriter, r *http.Request) {
	store, ok := h.managedAPIStore(w, r)
	if !ok {
		return
	}
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if !managedSchoolTenant(w, r, tenantID) {
		return
	}
	items, err := store.ListManagedAPIConfigs(r.Context(), tenantID)
	if err != nil {
		h.writeManagedAPIError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"configs": items})
}

func (h *Handler) CreateManagedAPIConfig(w http.ResponseWriter, r *http.Request) {
	store, ok := h.managedAPIStore(w, r)
	if !ok {
		return
	}
	var input ManagedAPIConfigInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	tenantID, ok := h.targetTenant(w, r, input.TenantID)
	if !ok {
		return
	}
	if !managedSchoolTenant(w, r, tenantID) {
		return
	}
	item, err := store.CreateManagedAPIConfig(r.Context(), tenantID, mustUser(r).ID, input)
	if err != nil {
		h.writeManagedAPIError(w, r, err)
		return
	}
	h.auditAction(r, "model.managed_api_created", "managed_model_api_config", item.ID, "assign encrypted third-party model API to school",
		map[string]any{"tenant_id": tenantID, "provider_key": item.ProviderKey, "adapter_type": item.AdapterType, "base_url": item.BaseURL, "model_version": item.ModelVersion, "is_default": item.IsDefault})
	httpx.JSON(w, http.StatusCreated, map[string]any{"config": item})
}

func (h *Handler) UpdateManagedAPIConfig(w http.ResponseWriter, r *http.Request) {
	store, ok := h.managedAPIStore(w, r)
	if !ok {
		return
	}
	var input ManagedAPIConfigUpdateInput
	if !decodeStrictJSON(w, r, &input) {
		return
	}
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if !managedSchoolTenant(w, r, tenantID) {
		return
	}
	item, err := store.UpdateManagedAPIConfig(r.Context(), tenantID, r.PathValue("id"), input)
	if err != nil {
		h.writeManagedAPIError(w, r, err)
		return
	}
	h.auditAction(r, "model.managed_api_updated", "managed_model_api_config", item.ID, "update school third-party model API assignment",
		map[string]any{"tenant_id": tenantID, "provider_key": item.ProviderKey, "adapter_type": item.AdapterType, "base_url": item.BaseURL, "model_version": item.ModelVersion, "status": item.Status, "is_default": item.IsDefault, "credential_rotated": strings.TrimSpace(input.APIKey) != ""})
	httpx.JSON(w, http.StatusOK, map[string]any{"config": item})
}

func (h *Handler) ProbeManagedAPIConfig(w http.ResponseWriter, r *http.Request) {
	store, ok := h.managedAPIStore(w, r)
	if !ok {
		return
	}
	tenantID, ok := h.targetTenant(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if !managedSchoolTenant(w, r, tenantID) {
		return
	}
	connection, err := store.GetManagedAPIConnection(r.Context(), tenantID, r.PathValue("id"))
	if err != nil {
		h.writeManagedAPIError(w, r, err)
		return
	}
	prober := h.managedAPIProber
	if prober == nil {
		prober = NewHTTPManagedAPIProber(0)
	}
	result := prober.Probe(r.Context(), connection)
	item, recordErr := store.RecordManagedAPIProbe(r.Context(), tenantID, connection.Config.ID, result)
	if recordErr != nil {
		h.writeManagedAPIError(w, r, recordErr)
		return
	}
	h.auditAction(r, "model.managed_api_probed", "managed_model_api_config", item.ID, "test school third-party model API connection without exposing credential",
		map[string]any{"tenant_id": tenantID, "provider_key": item.ProviderKey, "ok": result.OK, "status_code": result.StatusCode, "latency_ms": result.LatencyMS})
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadGateway
	}
	httpx.JSON(w, status, map[string]any{"result": result, "config": item})
}

func managedSchoolTenant(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	if tenantID == "" || tenantID == auth.PlatformTenantID {
		httpx.Error(w, r, http.StatusBadRequest, "school_tenant_required", "请选择要配置的学校")
		return false
	}
	return true
}

func (h *Handler) managedAPIStore(w http.ResponseWriter, r *http.Request) (ManagedAPIConfigStore, bool) {
	store, ok := h.store.(ManagedAPIConfigStore)
	if !ok {
		httpx.Error(w, r, http.StatusServiceUnavailable, "managed_model_api_unavailable", "第三方模型 API 配置暂不可用")
		return nil, false
	}
	return store, true
}

func (h *Handler) writeManagedAPIError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidManagedConfig):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_managed_model_api", "第三方模型 API 配置不完整或不安全")
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "managed_model_api_not_found", "第三方模型 API 配置不存在")
	case errors.Is(err, ErrConflict):
		httpx.Error(w, r, http.StatusConflict, "managed_model_api_conflict", "该学校已经存在相同的供应商配置")
	case errors.Is(err, ErrManagedConfigUnavailable):
		httpx.Error(w, r, http.StatusServiceUnavailable, "managed_model_api_unavailable", "第三方模型 API 密钥服务暂不可用")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "managed_model_api_failed", "第三方模型 API 配置操作失败")
	}
}

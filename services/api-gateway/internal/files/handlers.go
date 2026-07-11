package files

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/httpx"
	"edugrade-enterprise/services/api-gateway/internal/logger"
)

type Handler struct {
	store   Store
	objects ObjectStorage
	audit   auth.Store
	cfg     config.FileConfig
}

func NewHandler(store Store, objects ObjectStorage, audit auth.Store, cfg config.FileConfig) *Handler {
	cfg = normalizeFileConfig(cfg)
	return &Handler{store: store, objects: objects, audit: audit, cfg: cfg}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_multipart", "multipart form is invalid or too large")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_missing", "multipart field file is required")
		return
	}
	defer file.Close()

	if header.Size <= 0 || header.Size > h.cfg.MaxUploadBytes {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_file_size", fmt.Sprintf("file size must be between 1 and %d bytes", h.cfg.MaxUploadBytes))
		return
	}

	originalName, err := CleanFilename(header.Filename)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_filename", "file name is invalid")
		return
	}

	sample := make([]byte, 512)
	n, readErr := file.Read(sample)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		httpx.Error(w, r, http.StatusBadRequest, "file_read_failed", "failed to read uploaded file")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_seek_failed", "failed to inspect uploaded file")
		return
	}
	contentType, err := ValidateFileType(originalName, header.Header.Get("Content-Type"), SniffContentType(sample[:n]), h.cfg.AllowedExtensions)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "unsupported_file_type", "file type is not allowed")
		return
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_hash_failed", "failed to hash uploaded file")
		return
	}
	hashSHA256 := hex.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_seek_failed", "failed to prepare uploaded file")
		return
	}

	ownerType := strings.TrimSpace(r.FormValue("owner_type"))
	if ownerType == "" {
		ownerType = "generic"
	}
	if !ValidateOwnerType(ownerType) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_owner_type", "owner_type is not supported")
		return
	}
	ownerID := strings.TrimSpace(r.FormValue("owner_id"))
	if ownerID != "" && !IsUUIDLike(ownerID) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_owner_id", "owner_id must be a UUID")
		return
	}
	schoolID := strings.TrimSpace(r.FormValue("school_id"))
	examID := strings.TrimSpace(r.FormValue("exam_id"))
	submissionID := strings.TrimSpace(r.FormValue("submission_id"))
	for field, value := range map[string]string{"school_id": schoolID, "exam_id": examID, "submission_id": submissionID} {
		if value != "" && !IsUUIDLike(value) {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_"+field, field+" must be a UUID")
			return
		}
	}
	if !ValidateOwnerReferences(ownerType, ownerID, examID, submissionID) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_owner_reference", "file owner metadata is inconsistent")
		return
	}
	if existing, ok, err := h.store.FindDuplicate(r.Context(), user.TenantID, ownerType, ownerID, hashSHA256); err != nil {
		writeStoreError(w, r, err)
		return
	} else if ok {
		httpx.JSON(w, http.StatusConflict, map[string]any{
			"request_id": logger.RequestID(r.Context()),
			"error": map[string]string{
				"code":    "duplicate_file",
				"message": "same file already exists for this owner",
			},
			"existing_file": existing.Response(),
		})
		return
	}

	storageKey, err := BuildStorageKey(user.TenantID, hashSHA256, originalName)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "storage_key_failed", "failed to create storage key")
		return
	}
	if err := h.objects.Put(r.Context(), h.cfg.Bucket, storageKey, file, header.Size, contentType); err != nil {
		httpx.Error(w, r, http.StatusBadGateway, "object_storage_failed", "failed to write object storage")
		return
	}

	asset, err := h.store.Create(r.Context(), CreateAssetInput{
		TenantID:      user.TenantID,
		SchoolID:      schoolID,
		ExamID:        examID,
		SubmissionID:  submissionID,
		OwnerType:     ownerType,
		OwnerID:       ownerID,
		OriginalName:  originalName,
		ContentType:   contentType,
		SizeBytes:     header.Size,
		HashSHA256:    hashSHA256,
		StorageBucket: h.cfg.Bucket,
		StorageKey:    storageKey,
		Visibility:    "private",
		UploadedBy:    user.ID,
	})
	if err != nil {
		_ = h.objects.Remove(r.Context(), h.cfg.Bucket, storageKey)
		writeStoreError(w, r, err)
		return
	}
	h.auditAction(r, "file.uploaded", "file_asset", asset.ID, "upload private file")
	httpx.JSON(w, http.StatusCreated, map[string]any{"file": asset.Response()})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	asset, err := h.store.Get(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"file": asset.Response()})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	asset, err := h.store.Get(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	body, err := h.objects.Get(r.Context(), asset.StorageBucket, asset.StorageKey)
	if err != nil {
		httpx.Error(w, r, http.StatusBadGateway, "object_storage_failed", "failed to read object storage")
		return
	}
	defer body.Close()

	h.auditAction(r, "file.downloaded", "file_asset", asset.ID, "download private file")
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", asset.SizeBytes))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": asset.OriginalName}))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func normalizeFileConfig(cfg config.FileConfig) config.FileConfig {
	if cfg.Bucket == "" {
		cfg.Bucket = "edugrade-files"
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = 100 * 1024 * 1024
	}
	if len(cfg.AllowedExtensions) == 0 {
		cfg.AllowedExtensions = []string{".pdf", ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".csv", ".docx"}
	}
	return cfg
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := mustUser(r)
	asset, err := h.store.Delete(r.Context(), user.TenantID, r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	cleanup := "removed"
	if err := h.objects.Remove(r.Context(), asset.StorageBucket, asset.StorageKey); err != nil {
		cleanup = "pending"
	}
	h.auditAction(r, "file.deleted", "file_asset", asset.ID, "delete private file")
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "deleted", "object_cleanup": cleanup})
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, "file_not_found", "file asset not found")
	case errors.Is(err, ErrDuplicateFile):
		httpx.Error(w, r, http.StatusConflict, "duplicate_file", "same file already exists for this owner")
	default:
		httpx.Error(w, r, http.StatusInternalServerError, "file_operation_failed", "file operation failed")
	}
}

func mustUser(r *http.Request) auth.User {
	user, _ := auth.UserFromContext(r.Context())
	return user
}

func (h *Handler) auditAction(r *http.Request, action string, targetType string, targetID string, reason string) {
	user := mustUser(r)
	_ = h.audit.Audit(r.Context(), auth.AuditEvent{
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

package capture

import "strings"

var validSourceTypes = map[string]bool{
	"web_upload": true, "scanner_upload": true, "folder_import": true, "desktop_sync": true,
}

func validateCreateBatch(input *CreateBatchInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.ScannerDevice = strings.TrimSpace(input.ScannerDevice)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.Name == "" || len(input.Name) > 120 || !validSourceTypes[input.SourceType] {
		return ErrInvalidInput
	}
	return nil
}

func validateRegisterFile(input *RegisterFileInput, asset FileAssetSnapshot) error {
	input.FileAssetID = strings.TrimSpace(input.FileAssetID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.FileAssetID == "" || input.IdempotencyKey == "" || asset.ID != input.FileAssetID || asset.SizeBytes <= 0 || asset.SHA256 == "" {
		return ErrInvalidInput
	}
	return nil
}

func validRotation(value int) bool {
	return value == 0 || value == 90 || value == 180 || value == 270
}

func canSetBatchStatus(current, next string) bool {
	switch next {
	case "cancelled":
		return current != "completed" && current != "cancelled"
	case "draft":
		return current == "completed" || current == "cancelled"
	case "completed":
		return current == "ready"
	default:
		return false
	}
}

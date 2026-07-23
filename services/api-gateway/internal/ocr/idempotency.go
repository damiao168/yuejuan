package ocr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const maxIdempotencyKeyLength = 200

// PrepareCreateInput normalizes the OCR configuration and supplies a stable
// key when callers do not provide one. A deliberate re-run can use a new key;
// a transport retry with the same request reuses the original source task.
func PrepareCreateInput(submissionID string, input CreateTaskInput) (CreateTaskInput, error) {
	input = NormalizeCreateInput(input)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if err := ValidateCreateInput(input); err != nil {
		return CreateTaskInput{}, err
	}
	if input.IdempotencyKey == "" {
		raw, err := json.Marshal(struct {
			SubmissionID  string  `json:"submission_id"`
			Engine        string  `json:"engine"`
			EngineVersion string  `json:"engine_version"`
			MinConfidence float64 `json:"min_confidence"`
		}{
			SubmissionID: submissionID, Engine: input.Engine,
			EngineVersion: input.EngineVersion, MinConfidence: input.MinConfidence,
		})
		if err != nil {
			return CreateTaskInput{}, err
		}
		sum := sha256.Sum256(raw)
		input.IdempotencyKey = "ocr:" + hex.EncodeToString(sum[:])
	}
	if len(input.IdempotencyKey) > maxIdempotencyKeyLength {
		return CreateTaskInput{}, ErrInvalidInput
	}
	return input, nil
}

func sameCreateRequest(task Task, submissionID string, input CreateTaskInput) bool {
	return task.SubmissionID == submissionID && task.Engine == input.Engine &&
		task.EngineVersion == input.EngineVersion && task.MinConfidence == input.MinConfidence
}

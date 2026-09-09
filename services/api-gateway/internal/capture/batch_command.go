package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func batchCommandHash(examID string, input CreateBatchInput) string {
	data, _ := json.Marshal(struct{ ExamID, Name, SourceType, ScannerDevice string }{examID, input.Name, input.SourceType, input.ScannerDevice})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

type BatchCommandRecovery struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Batch     *Batch `json:"batch,omitempty"`
}

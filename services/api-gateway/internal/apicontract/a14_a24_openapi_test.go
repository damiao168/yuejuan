package apicontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These routes are deliberately listed from the wired handlers, not from a
// planning document. Keeping this compact inventory alongside the spec makes
// it much harder for an implemented safety boundary to silently disappear
// from the generated SDK.
func TestStoryA14ToA26OpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi", "edugrade-api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("OpenAPI contract must be valid JSON: %v", err)
	}
	paths := object(t, document, "paths")
	operations := []struct{ path, method, id string }{
		{"/api/v1/ai-eligibility/policy", "get", "getAIEligibilityPolicy"},
		{"/api/v1/ai-eligibility/policy", "put", "putAIEligibilityPolicy"},
		{"/api/v1/ai-eligibility/decisions/{runItemId}", "get", "getAIEligibilityDecision"},
		{"/api/v1/grading-evaluations", "post", "createGradingEvaluation"},
		{"/api/v1/model-calibrations", "post", "createModelCalibration"},
		{"/api/v1/ai-human-disagreements/{id}/classification", "put", "classifyAIHumanDisagreement"},
		{"/api/v1/exams/{examId}/score-releases", "post", "createScoreRelease"},
		{"/api/v1/regrade-jobs/{jobId}/score-release", "post", "createRegradeScoreRelease"},
		{"/api/v1/exams/{examId}/release-gate/preview", "post", "previewReleaseGate"},
		{"/api/v1/student/exams", "get", "listStudentPublishedExams"},
		{"/api/v1/student/exams/{examId}/question-appeals", "post", "createStudentQuestionAppeal"},
		{"/api/v1/student/exams/{examId}/questions/{questionId}/answer-image", "get", "getStudentPublishedQuestionAnswerImage"},
		{"/api/v1/question-appeals/{id}/events", "get", "listQuestionAppealEvents"},
		{"/api/v1/question-appeals/{id}/context", "get", "getQuestionAppealContext"},
		{"/api/v1/question-appeals/{id}/answer-image", "get", "getQuestionAppealAnswerImage"},
		{"/api/v1/capture/uploads:init", "post", "initCaptureUpload"},
		{"/api/v1/capture/uploads/{id}/chunks", "put", "putCaptureUploadChunk"},
		{"/api/v1/capture/uploads/{id}/complete", "post", "completeCaptureUpload"},
		{"/api/v1/exams/{examId}/processing/summary", "get", "getExamProcessingSummary"},
		{"/api/v1/processing/exceptions", "get", "listProcessingExceptions"},
		{"/api/v1/processing/exceptions/{id}/retry", "post", "retryProcessingException"},
		{"/api/v1/processing/exceptions/{id}/assign", "post", "assignProcessingException"},
		{"/api/v1/processing/exceptions/{id}/resolve", "post", "resolveProcessingException"},
	}
	for _, expected := range operations {
		operation := object(t, object(t, paths, expected.path), expected.method)
		if operation["operationId"] != expected.id {
			t.Fatalf("%s %s operationId = %#v, want %q", expected.method, expected.path, operation["operationId"], expected.id)
		}
		responses := object(t, operation, "responses")
		if _, ok := responses["401"]; !ok {
			t.Fatalf("%s %s must contract authentication", expected.method, expected.path)
		}
		if _, ok := responses["403"]; !ok {
			t.Fatalf("%s %s must contract authorization", expected.method, expected.path)
		}
	}

	schemas := object(t, object(t, document, "components"), "schemas")
	assertRequired(t, object(t, schemas, "AIEligibilityOutputConstraint"), []string{"criteria_evidence_only", "allow_model_final_score", "final_score_authority", "max_severe_error_risk"})
	assertRequired(t, object(t, schemas, "ScoreRelease"), []string{"visibility_policy", "appeal_window", "gate_snapshot"})
	assertRequired(t, object(t, schemas, "CaptureUploadInitRequest"), []string{"sha256", "size", "mime", "exam", "batch", "idempotency_key"})
	assertRequired(t, object(t, schemas, "ProcessingSummary"), []string{"total_pages", "ready_pages", "blocked_pages", "pending_pages", "by_stage", "issues"})
	assertRequired(t, object(t, schemas, "ProcessingException"), []string{"code", "severity", "blocking", "status", "details"})
	projectionTimestamp := object(t, object(t, object(t, schemas, "ProcessingExceptionListResponse"), "properties"), "projected_at")
	if projectionTimestamp["type"] != "string" || projectionTimestamp["format"] != "date-time" {
		t.Fatalf("processing exception list must expose its projection timestamp: %#v", projectionTimestamp)
	}

	studentQuestion := object(t, schemas, "StudentPublishedQuestion")
	studentQuestionProperties := object(t, studentQuestion, "properties")
	for _, forbidden := range []string{"final_grade_id", "source_id", "reviewer_id", "model_reference"} {
		if _, ok := studentQuestionProperties[forbidden]; ok {
			t.Fatalf("student result contract must not disclose %q", forbidden)
		}
	}
	appealDecision := object(t, schemas, "DecideQuestionAppealRequest")
	if _, ok := object(t, appealDecision, "properties")["score"]; ok {
		t.Fatal("question appeal decision must open a governed regrade, not write a score")
	}
	studentAnswerImage := object(t, object(t, paths, "/api/v1/student/exams/{examId}/questions/{questionId}/answer-image"), "get")
	imageResponses := object(t, studentAnswerImage, "responses")
	imageOK := object(t, imageResponses, "200")
	imageContent := object(t, imageOK, "content")
	if _, ok := imageContent["image/png"]; !ok {
		t.Fatal("student answer image must retain a binary image contract")
	}

	chunk := object(t, object(t, paths, "/api/v1/capture/uploads/{id}/chunks"), "put")
	requestBody := object(t, chunk, "requestBody")
	content := object(t, requestBody, "content")
	if _, ok := content["application/octet-stream"]; !ok {
		t.Fatal("capture upload chunk must retain its binary body contract")
	}
	parameters, ok := chunk["parameters"].([]any)
	if !ok || !hasParameter(parameters, "header", "Upload-Offset") || !hasParameter(parameters, "header", "X-Chunk-SHA256") {
		t.Fatal("capture upload chunk must retain offset and chunk hash headers")
	}
}

func hasParameter(values []any, location, name string) bool {
	for _, value := range values {
		parameter, ok := value.(map[string]any)
		if ok && parameter["in"] == location && parameter["name"] == name {
			return true
		}
	}
	return false
}

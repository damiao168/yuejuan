package subjective

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const (
	gradingAgentV2SchemaVersion = "grading-agent-v2"
	gradingAgentV2MaxMediaBytes = 5 * 1024 * 1024
	gradingAgentV2MaxPixels     = 12_000_000
	gradingAgentV2BBoxScale     = 1_000_000
)

func TestGradingAgentV2RequestFixtureHasVerifiedBoundPNG(t *testing.T) {
	request := loadGradingAgentV2Fixture(t, "valid-request.json")
	if err := validateGradingAgentV2RequestFixture(request); err != nil {
		t.Fatal(err)
	}
}

func TestGradingAgentV2InvalidRequestFixturesFailClosed(t *testing.T) {
	for _, name := range []string{"invalid-request-remote-url.json", "invalid-request-whole-page.json"} {
		t.Run(name, func(t *testing.T) {
			if err := validateGradingAgentV2RequestFixture(loadGradingAgentV2Fixture(t, name)); err == nil {
				t.Fatalf("%s unexpectedly passed validation", name)
			}
		})
	}
}

func TestGradingAgentV2ResponseFixturesBindCropEvidence(t *testing.T) {
	request := loadGradingAgentV2Fixture(t, "valid-request.json")
	response := loadGradingAgentV2Fixture(t, "valid-response.json")
	if err := validateGradingAgentV2ResponseFixture(response, request); err != nil {
		t.Fatal(err)
	}
	invalid := loadGradingAgentV2Fixture(t, "invalid-response-crop-hash-mismatch.json")
	if err := validateGradingAgentV2ResponseFixture(invalid, request); err == nil {
		t.Fatal("crop hash mismatch fixture unexpectedly passed validation")
	}
}

func TestGradingAgentV2ErrorFixtureRemainsNonRetryable(t *testing.T) {
	fixture := loadGradingAgentV2Fixture(t, "valid-error.json")
	if err := exactV2Fields(fixture, "schema_version", "request_id", "error"); err != nil {
		t.Fatal(err)
	}
	if fixture["schema_version"] != gradingAgentV2SchemaVersion {
		t.Fatalf("unexpected schema version: %v", fixture["schema_version"])
	}
	body, ok := fixture["error"].(map[string]any)
	if !ok {
		t.Fatal("error body must be an object")
	}
	if err := exactV2Fields(body, "code", "message", "retryable"); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "invalid_request" || body["retryable"] != false {
		t.Fatalf("unsafe error fixture: %#v", body)
	}
}

func TestProductionHTTPAdapterRemainsOnGradingAgentV1(t *testing.T) {
	if gradingAgentSchemaVersion != "grading-agent-v1" {
		t.Fatalf("061B0.3A must not switch the production adapter: %s", gradingAgentSchemaVersion)
	}
}

func loadGradingAgentV2Fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source path")
	}
	path := filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "contracts", "grading-agent", "v2", "fixtures", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("fixture contains trailing JSON: %v", err)
	}
	return value
}

func validateGradingAgentV2RequestFixture(request map[string]any) error {
	if err := exactV2Fields(
		request,
		"schema_version", "request_id", "subject", "grade_level", "question_id", "answer_segment_id",
		"question_type", "question_text", "max_score", "answer_text", "ocr_confidence", "rubric_version",
		"prompt_version", "rubric", "model_policy", "prompt_guard", "media_evidence",
	); err != nil {
		return err
	}
	if request["schema_version"] != gradingAgentV2SchemaVersion {
		return fmt.Errorf("unexpected schema version: %v", request["schema_version"])
	}
	for _, forbidden := range []string{
		"tenant_id", "school_id", "student_id", "student_name", "student_no", "class_id", "exam_id",
		"submission_id", "final_score", "published_score", "original_filename", "bucket", "storage_key", "url",
	} {
		if _, exists := request[forbidden]; exists {
			return fmt.Errorf("request leaks forbidden field %s", forbidden)
		}
	}

	media, ok := request["media_evidence"].(map[string]any)
	if !ok {
		return errors.New("media_evidence must be an object")
	}
	if err := exactV2Fields(
		media,
		"kind", "encoding", "media_type", "sha256", "byte_size", "width_pixels", "height_pixels",
		"normalized_bbox", "binding_hash", "data_base64",
	); err != nil {
		return err
	}
	if media["kind"] != "answer_segment_crop" || media["encoding"] != "base64" || media["media_type"] != "image/png" {
		return errors.New("media evidence literals are not approved")
	}
	size, err := v2JSONInt(media["byte_size"])
	if err != nil || size < 1 || size > gradingAgentV2MaxMediaBytes {
		return errors.New("media byte size is invalid")
	}
	width, err := v2JSONInt(media["width_pixels"])
	if err != nil || width < 1 {
		return errors.New("media width is invalid")
	}
	height, err := v2JSONInt(media["height_pixels"])
	if err != nil || height < 1 || width > gradingAgentV2MaxPixels/height {
		return errors.New("media height or pixel count is invalid")
	}
	bbox, units, err := gradingAgentV2BBox(media["normalized_bbox"])
	if err != nil {
		return err
	}
	if bbox[2]*bbox[3] >= 0.9 {
		return errors.New("whole-page or near-whole-page media evidence is forbidden")
	}
	encoded, ok := media["data_base64"].(string)
	if !ok || encoded == "" {
		return errors.New("media data must be non-empty base64")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return errors.New("media data is not valid base64")
	}
	if len(decoded) != size {
		return errors.New("media byte size does not match")
	}
	if len(decoded) < 24 || !bytes.Equal(decoded[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) || string(decoded[12:16]) != "IHDR" {
		return errors.New("media is not a PNG")
	}
	if int(binary.BigEndian.Uint32(decoded[16:20])) != width || int(binary.BigEndian.Uint32(decoded[20:24])) != height {
		return errors.New("media dimensions do not match decoded PNG")
	}
	digest := sha256.Sum256(decoded)
	if media["sha256"] != hex.EncodeToString(digest[:]) {
		return errors.New("media hash does not match decoded PNG")
	}
	expectedBinding, err := gradingAgentV2BindingHash(request, units)
	if err != nil {
		return err
	}
	if media["binding_hash"] != expectedBinding {
		return errors.New("media binding does not match request")
	}
	return nil
}

func validateGradingAgentV2ResponseFixture(response map[string]any, request map[string]any) error {
	if err := exactV2Fields(
		response,
		"schema_version", "request_id", "status", "delivery", "suggested_score", "max_score", "confidence",
		"matched_points", "missing_points", "deductions", "evidence", "risk_flags", "needs_human_review",
		"student_feedback", "teacher_note", "model_version", "prompt_version", "rubric_version",
		"capability_profile", "mock", "telemetry",
	); err != nil {
		return err
	}
	if response["schema_version"] != gradingAgentV2SchemaVersion ||
		response["request_id"] != request["request_id"] ||
		response["status"] != "suggestion" ||
		response["needs_human_review"] != true ||
		response["mock"] != false {
		return errors.New("response governance fields are invalid")
	}
	media := request["media_evidence"].(map[string]any)
	evidence, ok := response["evidence"].([]any)
	if !ok {
		return errors.New("response evidence must be an array")
	}
	ids := make(map[string]string, len(evidence))
	for _, raw := range evidence {
		item, ok := raw.(map[string]any)
		if !ok {
			return errors.New("response evidence item must be an object")
		}
		location, _ := item["location"].(string)
		switch location {
		case "answer_text":
			if err := exactV2Fields(item, "evidence_id", "rubric_point_id", "text_excerpt", "location", "confidence"); err != nil {
				return err
			}
			excerpt, _ := item["text_excerpt"].(string)
			answer, _ := request["answer_text"].(string)
			if excerpt == "" || !strings.Contains(answer, excerpt) {
				return errors.New("text evidence is not present in answer")
			}
		case "answer_crop":
			if err := exactV2Fields(item, "evidence_id", "rubric_point_id", "location", "crop_sha256", "normalized_bbox", "confidence"); err != nil {
				return err
			}
			if item["crop_sha256"] != media["sha256"] {
				return errors.New("crop evidence hash does not match request")
			}
			bbox, _, err := gradingAgentV2BBox(item["normalized_bbox"])
			if err != nil || bbox[0]+bbox[2] > 1 || bbox[1]+bbox[3] > 1 {
				return errors.New("crop evidence bbox is invalid")
			}
		default:
			return errors.New("response evidence location is unsupported")
		}
		id, idOK := item["evidence_id"].(string)
		pointID, pointOK := item["rubric_point_id"].(string)
		if !idOK || !pointOK || id == "" || pointID == "" {
			return errors.New("response evidence identifiers are invalid")
		}
		if _, exists := ids[id]; exists {
			return errors.New("response evidence id is duplicated")
		}
		ids[id] = pointID
	}
	matched, ok := response["matched_points"].([]any)
	if !ok {
		return errors.New("matched points must be an array")
	}
	for _, raw := range matched {
		point, ok := raw.(map[string]any)
		if !ok {
			return errors.New("matched point must be an object")
		}
		pointID, _ := point["rubric_point_id"].(string)
		links, ok := point["evidence_ids"].([]any)
		if !ok || len(links) == 0 {
			return errors.New("matched point must link evidence")
		}
		for _, link := range links {
			id, _ := link.(string)
			if ids[id] != pointID {
				return errors.New("matched point evidence link is invalid")
			}
		}
	}
	return nil
}

func gradingAgentV2BBox(raw any) ([4]float64, [4]int, error) {
	var values [4]float64
	var units [4]int
	bbox, ok := raw.(map[string]any)
	if !ok {
		return values, units, errors.New("bbox must be an object")
	}
	if err := exactV2Fields(bbox, "x", "y", "width", "height"); err != nil {
		return values, units, err
	}
	for index, field := range []string{"x", "y", "width", "height"} {
		number, ok := bbox[field].(json.Number)
		if !ok {
			return values, units, errors.New("bbox value must be a number")
		}
		value, err := number.Float64()
		if err != nil {
			return values, units, err
		}
		values[index] = value
		units[index] = int(value*gradingAgentV2BBoxScale + 0.5)
		if values[index] != float64(units[index])/gradingAgentV2BBoxScale {
			return values, units, errors.New("bbox supports at most six decimal places")
		}
	}
	if values[0] < 0 || values[1] < 0 || values[2] <= 0 || values[3] <= 0 ||
		values[0]+values[2] > 1 || values[1]+values[3] > 1 {
		return values, units, errors.New("bbox must stay within its source")
	}
	return values, units, nil
}

func gradingAgentV2BindingHash(request map[string]any, bbox [4]int) (string, error) {
	media, ok := request["media_evidence"].(map[string]any)
	if !ok {
		return "", errors.New("media_evidence must be an object")
	}
	parts := []string{gradingAgentV2SchemaVersion}
	for _, field := range []string{"request_id", "question_id", "answer_segment_id"} {
		value, ok := request[field].(string)
		if !ok || value == "" {
			return "", fmt.Errorf("%s must be a string", field)
		}
		parts = append(parts, value)
	}
	cropHash, ok := media["sha256"].(string)
	if !ok || cropHash == "" {
		return "", errors.New("media sha256 must be a string")
	}
	parts = append(parts, cropHash)
	for _, value := range bbox {
		parts = append(parts, strconv.Itoa(value))
	}
	var material strings.Builder
	for _, part := range parts {
		_, _ = fmt.Fprintf(&material, "%d:%s", len([]byte(part)), part)
	}
	digest := sha256.Sum256([]byte(material.String()))
	return hex.EncodeToString(digest[:]), nil
}

func exactV2Fields(value map[string]any, expected ...string) error {
	if len(value) != len(expected) {
		return fmt.Errorf("field count mismatch: got=%d want=%d", len(value), len(expected))
	}
	for _, field := range expected {
		if _, ok := value[field]; !ok {
			return fmt.Errorf("missing field %s", field)
		}
	}
	return nil
}

func v2JSONInt(value any) (int, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, errors.New("value must be a JSON number")
	}
	parsed, err := strconv.Atoi(number.String())
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

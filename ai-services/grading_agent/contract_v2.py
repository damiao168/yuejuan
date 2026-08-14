import base64
import binascii
import hashlib
import math
import re

from .contract import (
    CANONICAL_RISK_FLAGS,
    _exact_fields,
    _fail,
    _fail_evidence,
    _fail_model,
    _is_number,
    _number_evidence,
    _number_model,
    _string,
    validate_request,
)
from .errors import AgentError
from .guardrails import normalize_evidence_text

SCHEMA_VERSION = "grading-agent-v2"
MAX_MEDIA_BYTES = 5 * 1024 * 1024
MAX_MEDIA_PIXELS = 12_000_000
MAX_MEDIA_BASE64_LENGTH = 4 * ((MAX_MEDIA_BYTES + 2) // 3)
MAX_BBOX_AREA = 0.9
BBOX_SCALE = 1_000_000

_PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
_SHA256 = re.compile(r"^[a-f0-9]{64}$")
_INTERNAL_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$")
_REQUEST_FIELDS = {
    "schema_version",
    "request_id",
    "subject",
    "grade_level",
    "question_id",
    "answer_segment_id",
    "question_type",
    "question_text",
    "max_score",
    "answer_text",
    "ocr_confidence",
    "rubric_version",
    "prompt_version",
    "rubric",
    "model_policy",
    "prompt_guard",
    "output_constraint",
    "media_evidence",
}
_MEDIA_FIELDS = {
    "kind",
    "encoding",
    "media_type",
    "sha256",
    "byte_size",
    "width_pixels",
    "height_pixels",
    "normalized_bbox",
    "binding_hash",
    "data_base64",
}
_BBOX_FIELDS = {"x", "y", "width", "height"}
_RESPONSE_FIELDS = {
    "schema_version",
    "request_id",
    "status",
    "delivery",
    "suggested_score",
    "max_score",
    "confidence",
    "matched_points",
    "missing_points",
    "deductions",
    "evidence",
    "risk_flags",
    "needs_human_review",
    "student_feedback",
    "teacher_note",
    "model_version",
    "prompt_version",
    "rubric_version",
    "capability_profile",
    "mock",
    "telemetry",
}
_FORBIDDEN_FIELDS = {
    "tenant_id",
    "school_id",
    "student_id",
    "student_name",
    "student_no",
    "class_id",
    "exam_id",
    "submission_id",
    "final_score",
    "published_score",
    "original_filename",
    "bucket",
    "storage_key",
    "url",
}


def _bbox_units(bbox, request_id):
    _exact_fields(bbox, _BBOX_FIELDS, "media_evidence.normalized_bbox", request_id)
    values = (bbox["x"], bbox["y"], bbox["width"], bbox["height"])
    if any(not _is_number(value) for value in values):
        _fail("media_evidence.normalized_bbox values must be finite numbers", request_id)
    x, y, width, height = values
    if x < 0 or y < 0 or width <= 0 or height <= 0 or x + width > 1 or y + height > 1:
        _fail("media_evidence.normalized_bbox must stay within its source page", request_id)
    if width * height >= MAX_BBOX_AREA:
        _fail("whole-page or near-whole-page media evidence is forbidden", request_id)
    units = tuple(round(value * BBOX_SCALE) for value in values)
    if any(not math.isclose(value, unit / BBOX_SCALE, abs_tol=1e-12) for value, unit in zip(values, units)):
        _fail("media_evidence.normalized_bbox supports at most six decimal places", request_id)
    return units


def compute_media_binding_hash(request):
    """Return the cross-runtime binding for one attested answer crop."""

    media = request["media_evidence"]
    units = _bbox_units(media["normalized_bbox"], request.get("request_id", ""))
    parts = (
        SCHEMA_VERSION,
        request["request_id"],
        request["question_id"],
        request["answer_segment_id"],
        media["sha256"],
        *(str(value) for value in units),
    )
    material = "".join(f"{len(part.encode('utf-8'))}:{part}" for part in parts).encode("utf-8")
    return hashlib.sha256(material).hexdigest()


def validate_media_evidence(media, request):
    request_id = request.get("request_id", "")
    _exact_fields(media, _MEDIA_FIELDS, "media_evidence", request_id)
    if media["kind"] != "answer_segment_crop":
        _fail("media_evidence.kind must be answer_segment_crop", request_id)
    if media["encoding"] != "base64":
        _fail("media_evidence.encoding must be base64", request_id)
    if media["media_type"] != "image/png":
        _fail("media_evidence.media_type must be image/png", request_id)
    if not isinstance(media["sha256"], str) or not _SHA256.fullmatch(media["sha256"]):
        _fail("media_evidence.sha256 must be lowercase hexadecimal SHA-256", request_id)
    if not isinstance(media["binding_hash"], str) or not _SHA256.fullmatch(media["binding_hash"]):
        _fail("media_evidence.binding_hash must be lowercase hexadecimal SHA-256", request_id)

    byte_size = media["byte_size"]
    width = media["width_pixels"]
    height = media["height_pixels"]
    if isinstance(byte_size, bool) or not isinstance(byte_size, int) or not 1 <= byte_size <= MAX_MEDIA_BYTES:
        _fail("media_evidence.byte_size is outside the approved limit", request_id)
    if (
        isinstance(width, bool)
        or isinstance(height, bool)
        or not isinstance(width, int)
        or not isinstance(height, int)
        or width <= 0
        or height <= 0
        or width * height > MAX_MEDIA_PIXELS
    ):
        _fail("media_evidence dimensions are invalid or too large", request_id)

    _bbox_units(media["normalized_bbox"], request_id)
    if (
        not isinstance(media["data_base64"], str)
        or not media["data_base64"]
        or len(media["data_base64"]) > MAX_MEDIA_BASE64_LENGTH
    ):
        _fail("media_evidence.data_base64 must be non-empty and bounded", request_id)
    try:
        decoded = base64.b64decode(media["data_base64"], validate=True)
    except (ValueError, binascii.Error) as exc:
        raise AgentError(
            "invalid_request",
            "media_evidence.data_base64 is not valid base64",
            status=400,
            request_id=request_id,
        ) from exc
    if len(decoded) != byte_size:
        _fail("media_evidence.byte_size does not match decoded content", request_id)
    if len(decoded) < 24 or not decoded.startswith(_PNG_SIGNATURE) or decoded[12:16] != b"IHDR":
        _fail("media_evidence content is not a PNG", request_id)
    decoded_width = int.from_bytes(decoded[16:20], "big")
    decoded_height = int.from_bytes(decoded[20:24], "big")
    if (decoded_width, decoded_height) != (width, height):
        _fail("media_evidence dimensions do not match decoded PNG", request_id)
    if decoded_width * decoded_height > MAX_MEDIA_PIXELS:
        _fail("decoded PNG exceeds the pixel limit", request_id)
    if hashlib.sha256(decoded).hexdigest() != media["sha256"]:
        _fail("media_evidence.sha256 does not match decoded content", request_id)
    if compute_media_binding_hash(request) != media["binding_hash"]:
        _fail("media_evidence.binding_hash does not match this request", request_id)
    return media


def validate_request_v2(payload):
    if not isinstance(payload, dict):
        _fail("request body must be an object")
    request_id = payload.get("request_id") if isinstance(payload.get("request_id"), str) else ""
    leaked = sorted(set(payload) & _FORBIDDEN_FIELDS)
    if leaked:
        _fail(f"request contains forbidden identity, storage, or final-grade fields: {leaked}", request_id)
    _exact_fields(payload, _REQUEST_FIELDS, "request", request_id)
    if payload["schema_version"] != SCHEMA_VERSION:
        _fail("schema_version is unsupported", request_id)
    for field in ("request_id", "question_id", "answer_segment_id"):
        if not isinstance(payload[field], str) or not _INTERNAL_ID.fullmatch(payload[field]):
            _fail(f"{field} must be a bounded internal identifier", request_id)

    # Reuse the already-parsed values without duplicating the bounded Base64
    # string. The v1 validator is read-only, so a shallow inherited envelope is
    # sufficient and keeps peak memory predictable for the future v2 seam.
    inherited = {key: value for key, value in payload.items() if key != "media_evidence"}
    inherited["schema_version"] = "grading-agent-v1"
    validate_request(inherited)
    validate_media_evidence(payload["media_evidence"], payload)
    return payload


def _validate_crop_bbox(bbox, request_id):
    _exact_fields(bbox, _BBOX_FIELDS, "answer_crop.normalized_bbox", request_id)
    values = (bbox["x"], bbox["y"], bbox["width"], bbox["height"])
    if any(not _is_number(value) for value in values):
        _fail_evidence("answer_crop bbox values must be finite numbers", request_id)
    x, y, width, height = values
    if x < 0 or y < 0 or width <= 0 or height <= 0 or x + width > 1 or y + height > 1:
        _fail_evidence("answer_crop bbox must stay within the crop", request_id)


def validate_response_v2(suggestion, request):
    """Validate an already-normalized v2 suggestion without enabling a v2 route."""

    request_id = request["request_id"]
    if not isinstance(suggestion, dict):
        _fail_model("suggestion must be an object", request_id)
    _exact_fields(suggestion, _RESPONSE_FIELDS, "suggestion", request_id)
    if suggestion["schema_version"] != SCHEMA_VERSION:
        _fail_model("suggestion schema version mismatch", request_id)
    if suggestion["request_id"] != request_id:
        _fail_model("suggestion request id mismatch", request_id)
    if suggestion["status"] != "suggestion":
        _fail_model("suggestion status is invalid", request_id)
    if suggestion["delivery"] not in {"teacher_suggestion", "shadow_only"}:
        _fail_model("suggestion delivery is invalid", request_id)
    if suggestion["needs_human_review"] is not True or suggestion["mock"] is not False:
        _fail_model("suggestion governance flags are invalid", request_id)
    if suggestion["rubric_version"] != request["rubric_version"] or suggestion["prompt_version"] != request["prompt_version"]:
        _fail_model("suggestion version mismatch", request_id)
    if suggestion["max_score"] != request["max_score"]:
        _fail_model("suggestion max score mismatch", request_id)
    _number_model(suggestion["confidence"], "confidence", request_id, 0, 1)
    if suggestion["deductions"] != []:
        _fail_model("suggestion deductions are not enabled", request_id)
    risk_flags = suggestion["risk_flags"]
    if (
        not isinstance(risk_flags, list)
        or len(risk_flags) > 20
        or any(not isinstance(flag, str) for flag in risk_flags)
        or len(set(risk_flags)) != len(risk_flags)
        or not set(risk_flags).issubset(CANONICAL_RISK_FLAGS)
    ):
        _fail_model("suggestion contains an unsupported risk flag", request_id)

    rubric_points = {point["id"] for point in request["rubric"]["points"]}
    evidence_by_id = {}
    evidence = suggestion["evidence"]
    if not isinstance(evidence, list) or len(evidence) > 200:
        _fail_evidence("evidence must be a bounded array", request_id)
    for index, item in enumerate(evidence):
        if not isinstance(item, dict):
            _fail_evidence(f"evidence[{index}] must be an object", request_id)
        common = {"evidence_id", "rubric_point_id", "location", "confidence"}
        location = item.get("location")
        expected = common | ({"text_excerpt"} if location == "answer_text" else {"crop_sha256", "normalized_bbox"})
        if set(item) != expected:
            _fail_evidence(f"evidence[{index}] fields are invalid", request_id)
        evidence_id = item["evidence_id"]
        point_id = item["rubric_point_id"]
        _string(evidence_id, f"evidence[{index}].evidence_id", request_id)
        _string(point_id, f"evidence[{index}].rubric_point_id", request_id)
        if evidence_id in evidence_by_id:
            _fail_evidence("suggestion emitted duplicate evidence ids", request_id)
        if point_id not in rubric_points:
            _fail_evidence("suggestion evidence references an unknown rubric point", request_id)
        _number_evidence(item["confidence"], "evidence confidence", request_id, 0, 1)
        if location == "answer_text":
            _string(item["text_excerpt"], f"evidence[{index}].text_excerpt", request_id, 1, 4000)
            excerpt = normalize_evidence_text(item["text_excerpt"])
            if not excerpt or excerpt not in normalize_evidence_text(request["answer_text"]):
                _fail_evidence("evidence excerpt does not occur in the answer", request_id)
        elif location == "answer_crop":
            if item["crop_sha256"] != request["media_evidence"]["sha256"]:
                _fail_evidence("answer_crop hash does not match request media", request_id)
            _validate_crop_bbox(item["normalized_bbox"], request_id)
        else:
            _fail_evidence("evidence location is unsupported", request_id)
        evidence_by_id[evidence_id] = item

    matched = suggestion["matched_points"]
    if not isinstance(matched, list) or len(matched) > 100:
        _fail_model("matched_points must be a bounded array", request_id)
    score = 0
    matched_ids = set()
    for index, point in enumerate(matched):
        expected = {"rubric_point_id", "label", "score", "evidence_ids"}
        if not isinstance(point, dict) or set(point) != expected:
            _fail_model(f"matched_points[{index}] is invalid", request_id)
        point_id = point["rubric_point_id"]
        if point_id not in rubric_points or point_id in matched_ids:
            _fail_model("matched point is unknown or duplicated", request_id)
        matched_ids.add(point_id)
        _number_model(point["score"], "matched point score", request_id, 0, request["max_score"])
        if not isinstance(point["evidence_ids"], list) or not point["evidence_ids"]:
            _fail_evidence("matched point must link evidence", request_id)
        for evidence_id in point["evidence_ids"]:
            linked = evidence_by_id.get(evidence_id)
            if not linked or linked["rubric_point_id"] != point_id:
                _fail_evidence("matched point evidence link is invalid", request_id)
        score += point["score"]
    _number_model(suggestion["suggested_score"], "suggested score", request_id, 0, request["max_score"])
    if not math.isclose(score, suggestion["suggested_score"], abs_tol=1e-6):
        _fail_model("suggested score was not code-recomputed", request_id)
    return suggestion

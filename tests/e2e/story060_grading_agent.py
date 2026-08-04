import json
import os
from urllib import request as urlrequest


API = os.environ.get("EDUGRADE_E2E_API_BASE_URL", "http://api-gateway:8080").rstrip("/")
AGENT = os.environ.get("EDUGRADE_E2E_AGENT_BASE_URL", "http://grading-agent:8100").rstrip("/")
PASSWORD = os.environ["EDUGRADE_E2E_ADMIN_PASSWORD"]
SEGMENT_ID = "00000000-0000-0000-0000-000000000614"


def call_json(url, method="GET", payload=None, headers=None, timeout=30):
    body = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    merged = {"Accept": "application/json", **(headers or {})}
    if body is not None:
        merged["Content-Type"] = "application/json"
    req = urlrequest.Request(url, data=body, headers=merged, method=method)
    with urlrequest.urlopen(req, timeout=timeout) as response:
        return response.status, json.loads(response.read().decode("utf-8"))


status, ready = call_json(f"{AGENT}/ready", timeout=10)
assert status == 200 and ready["status"] == "ready", ready

status, login = call_json(
    f"{API}/api/v1/auth/token",
    method="POST",
    payload={"tenant_code": "platform", "username": "platform_admin", "password": PASSWORD, "client_type": "desktop", "device_name": "STORY-060 E2E"},
)
assert status == 200 and login.get("access_token"), login
headers = {"Authorization": f"Bearer {login['access_token']}"}

status, result = call_json(
    f"{API}/api/v1/answer-segments/{SEGMENT_ID}/subjective-ai-grade",
    method="POST",
    payload={"model_policy": {"model_version": "caller-cannot-select", "prompt_version": "caller-cannot-select", "min_confidence": 0.1}},
    headers=headers,
    timeout=360,
)
assert status == 201, result
grade = result["grade"]
assert grade["status"] == "succeeded", grade.get("failure_reason")
assert grade["mock"] is False
assert grade["needs_human_review"] is True
assert grade["confidence"] == 0
assert grade["model_version"] == "Qwen/Qwen3-4B-GGUF:Q4_K_M"
assert grade["prompt_version"] == "subjective-local-structured-v2"
assert grade["rubric_version"] == "rubric-v3"
assert grade["delivery_mode"] == "teacher_suggestion"
assert grade["capability_profile"] == "local-pilot-v1"
assert grade["adapter_name"] == "local_llama_cpp"
assert 1 <= grade["adapter_attempts"] <= 2
assert grade["suggested_score"] == sum(point["score"] for point in grade["matched_points"])
assert grade["evidence"]
assert all(item["answer_text"] in "因为他对家乡有责任感，也希望帮助村里的孩子继续读书。" for item in grade["evidence"])
assert "tenant_id" not in grade.get("raw_output", {})
assert "student_id" not in grade.get("raw_output", {})

print("STORY-060 real local-model shadow grading API verification passed")

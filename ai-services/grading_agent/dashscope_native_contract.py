import json
import re
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from .errors import AgentError

DASHSCOPE_TEXT_GENERATION_PATH = "/services/aigc/text-generation/generation"
_MODEL_NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$")
_REQUEST_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$")
_MESSAGE_ROLES = frozenset({"system", "user", "assistant"})
FORBIDDEN_EXPORT_KEYS = frozenset(
    {
        "tenant_id",
        "school_id",
        "student_id",
        "student_name",
        "student_no",
        "class_id",
        "exam_id",
        "submission_id",
        "question_id",
        "answer_segment_id",
        "final_score",
        "published_score",
        "answer_image_ref",
    }
)


@dataclass(frozen=True)
class DashScopeNativeResult:
    output: dict
    request_id: str
    input_tokens: int
    output_tokens: int
    total_tokens: int


def _bounded_identifier(value, pattern, field):
    if not isinstance(value, str) or not pattern.fullmatch(value):
        raise ValueError(f"{field} must be a bounded identifier")
    return value


def _validate_messages(messages):
    if not isinstance(messages, Sequence) or isinstance(messages, (str, bytes)) or not messages:
        raise ValueError("messages must be a non-empty sequence")
    normalized = []
    for message in messages:
        if not isinstance(message, Mapping) or set(message) != {"role", "content"}:
            raise ValueError("each message must contain only role and content")
        role = message["role"]
        content = message["content"]
        if role not in _MESSAGE_ROLES:
            raise ValueError("message role is not supported")
        if not isinstance(content, str) or not content.strip() or len(content) > 100_000:
            raise ValueError("message content must be non-empty and bounded")
        normalized.append({"role": role, "content": content})
    if not any("json" in message["content"].lower() for message in normalized):
        raise ValueError("DashScope JSON mode requires a prompt that explicitly requests JSON")
    return normalized


def _forbidden_export_fields(value):
    found = set()
    if isinstance(value, Mapping):
        found.update(key for key in value if key in FORBIDDEN_EXPORT_KEYS)
        for child in value.values():
            found.update(_forbidden_export_fields(child))
    elif isinstance(value, Sequence) and not isinstance(value, (str, bytes)):
        for child in value:
            found.update(_forbidden_export_fields(child))
    return found


def build_dashscope_text_payload(
    *,
    model,
    messages,
    max_completion_tokens,
    temperature=0.0,
    seed=42,
):
    """Build a native DashScope text-generation body without auth or transport."""

    _bounded_identifier(model, _MODEL_NAME, "model")
    if not isinstance(max_completion_tokens, int) or not 1 <= max_completion_tokens <= 16_384:
        raise ValueError("max_completion_tokens must be between 1 and 16384")
    if isinstance(temperature, bool) or not isinstance(temperature, (int, float)) or not 0 <= temperature <= 2:
        raise ValueError("temperature must be between 0 and 2")
    if isinstance(seed, bool) or not isinstance(seed, int) or not 0 <= seed <= 2_147_483_647:
        raise ValueError("seed must be between 0 and 2147483647")
    return {
        "model": model,
        "input": {"messages": _validate_messages(messages)},
        "parameters": {
            "result_format": "message",
            "response_format": {"type": "json_object"},
            "enable_thinking": False,
            "max_completion_tokens": max_completion_tokens,
            "temperature": temperature,
            "seed": seed,
        },
    }


def build_dashscope_grading_payload(
    *,
    grading_request,
    prompt_registry,
    model,
    max_completion_tokens,
    temperature=0.0,
    seed=42,
    repair_reason=None,
):
    """Project a governed grading request onto the native provider whitelist."""

    payload = build_dashscope_text_payload(
        model=model,
        messages=prompt_registry.messages(grading_request, repair_reason),
        max_completion_tokens=max_completion_tokens,
        temperature=temperature,
        seed=seed,
    )
    leaked = sorted(_forbidden_export_fields(payload))
    if leaked:
        raise ValueError(f"DashScope payload contains forbidden export fields: {', '.join(leaked)}")
    return payload


def parse_dashscope_text_response(response):
    """Parse a non-streaming native response and retain only governed facts."""

    if not isinstance(response, Mapping):
        raise AgentError("model_output_invalid", "provider response was not an object", status=502)
    try:
        request_id = _bounded_identifier(response["request_id"], _REQUEST_ID, "request_id")
        choice = response["output"]["choices"][0]
        finish_reason = choice["finish_reason"]
        content = choice["message"]["content"]
        usage = response["usage"]
        input_tokens = usage["input_tokens"]
        output_tokens = usage["output_tokens"]
        total_tokens = usage["total_tokens"]
    except (KeyError, IndexError, TypeError, ValueError) as exc:
        raise AgentError(
            "model_output_invalid",
            "provider response omitted required structured facts",
            status=502,
        ) from exc
    if finish_reason != "stop":
        raise AgentError(
            "model_output_invalid",
            "provider response did not complete normally",
            status=502,
            request_id=request_id,
        )
    if not isinstance(content, str) or not content.strip():
        raise AgentError(
            "model_output_invalid",
            "provider response contained no structured content",
            status=502,
            request_id=request_id,
        )
    token_counts = (input_tokens, output_tokens, total_tokens)
    if any(isinstance(value, bool) or not isinstance(value, int) or value < 0 for value in token_counts):
        raise AgentError(
            "model_output_invalid",
            "provider response contained invalid usage facts",
            status=502,
            request_id=request_id,
        )
    try:
        output = json.loads(content)
    except json.JSONDecodeError as exc:
        raise AgentError(
            "model_output_invalid",
            "provider response was not valid JSON",
            status=502,
            request_id=request_id,
        ) from exc
    if not isinstance(output, dict):
        raise AgentError(
            "model_output_invalid",
            "provider response JSON was not an object",
            status=502,
            request_id=request_id,
        )
    return DashScopeNativeResult(
        output=output,
        request_id=request_id,
        input_tokens=input_tokens,
        output_tokens=output_tokens,
        total_tokens=total_tokens,
    )


def map_dashscope_error(http_status, body):
    """Map native provider errors to the existing public grading-agent error set."""

    if isinstance(http_status, bool) or not isinstance(http_status, int) or not isinstance(body, Mapping):
        raise TypeError("provider error fixture is invalid")
    request_id = body.get("request_id", "")
    if request_id:
        try:
            request_id = _bounded_identifier(request_id, _REQUEST_ID, "request_id")
        except ValueError:
            request_id = ""
    code = body.get("code", "")
    if not isinstance(code, str):
        code = ""
    normalized_code = code.lower()
    if http_status == 408 or "timeout" in normalized_code:
        return AgentError(
            "model_timeout",
            "external model request timed out",
            status=504,
            retryable=True,
            request_id=request_id,
        )
    retryable = http_status == 429 or http_status >= 500 or normalized_code.startswith("throttling")
    return AgentError(
        "model_unavailable",
        "external model request failed",
        status=503,
        retryable=retryable,
        request_id=request_id,
    )

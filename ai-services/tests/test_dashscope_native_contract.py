import copy
import json
import unittest
from pathlib import Path

from grading_agent.app import GradingAgentApplication
from grading_agent.dashscope_native_contract import (
    DASHSCOPE_TEXT_GENERATION_PATH,
    FORBIDDEN_EXPORT_KEYS,
    build_dashscope_grading_payload,
    build_dashscope_text_payload,
    map_dashscope_error,
    parse_dashscope_text_response,
)
from grading_agent.errors import AgentError
from grading_agent.model import PromptRegistry
from helpers import FakeModel, settings, valid_request

FIXTURES = Path(__file__).parent / "fixtures" / "dashscope-native"


def fixture(name):
    return json.loads(FIXTURES.joinpath(name).read_text(encoding="utf-8"))


class DashScopeNativeContractTests(unittest.TestCase):
    def test_native_text_fixture_matches_the_offline_builder(self):
        expected = fixture("request-text.json")
        payload = build_dashscope_text_payload(
            model=expected["model"],
            messages=expected["input"]["messages"],
            max_completion_tokens=expected["parameters"]["max_completion_tokens"],
            temperature=expected["parameters"]["temperature"],
            seed=expected["parameters"]["seed"],
        )
        self.assertEqual(payload, expected)
        self.assertEqual(DASHSCOPE_TEXT_GENERATION_PATH, "/services/aigc/text-generation/generation")
        self.assertNotIn("compatible-mode", DASHSCOPE_TEXT_GENERATION_PATH)

    def test_governed_projection_omits_internal_identity_and_image_fields(self):
        current = settings()
        prompt_registry = PromptRegistry(current.prompt_root, current.prompt_version)
        request = valid_request()
        payload = build_dashscope_grading_payload(
            grading_request=request,
            prompt_registry=prompt_registry,
            model="qwen-plus-2024-11-25",
            max_completion_tokens=384,
        )
        serialized = json.dumps(payload, ensure_ascii=False)
        for key in FORBIDDEN_EXPORT_KEYS:
            self.assertNotIn(f'"{key}"', serialized)
        for value in (request["request_id"], request["question_id"], request["answer_segment_id"]):
            self.assertNotIn(value, serialized)

    def test_json_mode_requires_an_explicit_json_instruction(self):
        with self.assertRaisesRegex(ValueError, "explicitly requests JSON"):
            build_dashscope_text_payload(
                model="qwen-plus-2024-11-25",
                messages=[{"role": "user", "content": "Return a synthetic result."}],
                max_completion_tokens=384,
            )

    def test_success_fixture_preserves_request_id_usage_and_structured_output(self):
        result = parse_dashscope_text_response(fixture("response-text.json"))
        self.assertEqual(result.request_id, "fixture-dashscope-request-0001")
        self.assertEqual(result.input_tokens, 240)
        self.assertEqual(result.output_tokens, 180)
        self.assertEqual(result.total_tokens, 420)
        self.assertEqual(result.output["suggested_score"], 4)

    def test_success_fixture_output_passes_the_existing_grading_contract(self):
        result = parse_dashscope_text_response(fixture("response-text.json"))
        app = GradingAgentApplication(settings(), model=FakeModel([result.output]))
        suggestion, replayed = app.grade(valid_request(), "subjective-grade-0001")
        self.assertFalse(replayed)
        self.assertEqual(suggestion["suggested_score"], 4)
        self.assertTrue(suggestion["needs_human_review"])

    def test_truncated_or_non_json_success_is_rejected(self):
        response = fixture("response-text.json")
        response["output"]["choices"][0]["finish_reason"] = "length"
        with self.assertRaisesRegex(AgentError, "did not complete normally"):
            parse_dashscope_text_response(response)
        response = fixture("response-text.json")
        response["output"]["choices"][0]["message"]["content"] = "not-json"
        with self.assertRaisesRegex(AgentError, "not valid JSON"):
            parse_dashscope_text_response(response)

    def test_invalid_usage_is_rejected(self):
        response = fixture("response-text.json")
        response["usage"]["input_tokens"] = -1
        with self.assertRaisesRegex(AgentError, "invalid usage facts"):
            parse_dashscope_text_response(response)

    def test_throttling_maps_to_sanitized_retryable_unavailable(self):
        error_fixture = fixture("error-throttled.json")
        error = map_dashscope_error(error_fixture["http_status"], error_fixture["body"])
        self.assertEqual(error.code, "model_unavailable")
        self.assertEqual(error.status, 503)
        self.assertTrue(error.retryable)
        self.assertEqual(error.request_id, "fixture-dashscope-request-0002")
        self.assertNotIn(error_fixture["body"]["message"], error.message)

    def test_authentication_failure_is_sanitized_and_not_retryable(self):
        error_fixture = fixture("error-auth.json")
        error = map_dashscope_error(error_fixture["http_status"], error_fixture["body"])
        self.assertEqual(error.code, "model_unavailable")
        self.assertEqual(error.status, 503)
        self.assertFalse(error.retryable)
        self.assertEqual(error.request_id, "fixture-dashscope-request-0003")
        self.assertNotIn(error_fixture["body"]["message"], error.message)

    def test_timeout_code_maps_to_existing_timeout_contract(self):
        error = map_dashscope_error(
            500,
            {
                "code": "InternalError.Timeout",
                "message": "synthetic timeout fixture",
                "request_id": "fixture-dashscope-request-0004",
            },
        )
        self.assertEqual(error.code, "model_timeout")
        self.assertEqual(error.status, 504)
        self.assertTrue(error.retryable)

    def test_fixture_parser_does_not_mutate_provider_response(self):
        response = fixture("response-text.json")
        original = copy.deepcopy(response)
        parse_dashscope_text_response(response)
        self.assertEqual(response, original)


if __name__ == "__main__":
    unittest.main()

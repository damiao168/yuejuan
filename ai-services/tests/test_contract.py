import unittest

from grading_agent.capabilities import CapabilityMatrix
from grading_agent.contract import normalize_model_output, validate_request
from grading_agent.errors import AgentError

from helpers import settings, valid_raw_output, valid_request


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.settings = settings()
        self.matrix = CapabilityMatrix.load(self.settings.contract_root)

    def _normalize(self, request, raw):
        route = self.matrix.route(request["grade_level"], request["subject"], request["question_type"])
        return normalize_model_output(
            raw,
            request,
            route,
            self.settings.model_version,
            self.settings.prompt_version,
            self.matrix.profile_id,
            {"adapter": "local_llama_cpp", "attempts": 1, "repair_attempted": False, "prior_error_codes": [], "elapsed_ms": 1},
        )

    def test_valid_request_and_output_are_promoted_without_identity(self):
        request = valid_request()
        validate_request(request)
        suggestion = self._normalize(request, valid_raw_output())
        self.assertEqual(suggestion["suggested_score"], 4)
        self.assertEqual(suggestion["confidence"], 0)
        self.assertTrue(suggestion["needs_human_review"])
        self.assertNotIn("tenant_id", suggestion)
        self.assertNotIn("student_id", suggestion)

    def test_forbidden_identity_field_is_rejected(self):
        request = valid_request()
        request["student_name"] = "not allowed"
        with self.assertRaisesRegex(AgentError, "forbidden"):
            validate_request(request)

    def test_unapproved_capability_fails_closed(self):
        request = valid_request()
        request["subject"] = "math"
        validate_request(request)
        with self.assertRaises(AgentError) as caught:
            self.matrix.route(request["grade_level"], request["subject"], request["question_type"], request["request_id"])
        self.assertEqual(caught.exception.code, "capability_not_supported")

    def test_evidence_excerpt_must_exist_in_answer(self):
        request = valid_request()
        raw = valid_raw_output()
        raw["evidence"][0]["text_excerpt"] = "invented evidence"
        with self.assertRaises(AgentError) as caught:
            self._normalize(request, raw)
        self.assertEqual(caught.exception.code, "evidence_verification_failed")

    def test_model_total_is_ignored_and_recomputed_from_points(self):
        request = valid_request()
        raw = valid_raw_output()
        raw["suggested_score"] = 1
        self.assertEqual(self._normalize(request, raw)["suggested_score"], 4)

    def test_low_ocr_and_injection_force_canonical_risks(self):
        request = valid_request()
        request["ocr_confidence"] = 0.5
        request["prompt_guard"]["suspected_injection"] = True
        suggestion = self._normalize(request, valid_raw_output())
        self.assertIn("ocr_low_confidence", suggestion["risk_flags"])
        self.assertIn("prompt_injection_suspected", suggestion["risk_flags"])


if __name__ == "__main__":
    unittest.main()

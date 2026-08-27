import json
import shutil
import tempfile
import unittest
from pathlib import Path

from grading_agent.model import (
    LocalLlamaCppAdapter,
    PromptRegistry,
    grading_output_schema,
)
from helpers import settings, valid_raw_output, valid_request


class ModelAdapterTests(unittest.TestCase):
    def test_llama_cpp_request_uses_strict_schema_and_untrusted_answer_boundary(self):
        calls = []

        def transport(url, payload, headers, timeout):
            calls.append((url, payload, headers, timeout))
            return {
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                valid_raw_output(), ensure_ascii=False
                            )
                        }
                    }
                ]
            }

        adapter = LocalLlamaCppAdapter(
            settings(model_api_key="model-secret"), transport=transport
        )
        raw = adapter.request(valid_request())
        self.assertEqual(raw["matched_points"][0]["rubric_point_id"], "p1")
        url, payload, headers, _timeout = calls[0]
        self.assertTrue(url.endswith("/chat/completions"))
        self.assertEqual(headers["Authorization"], "Bearer model-secret")
        self.assertTrue(payload["response_format"]["json_schema"]["strict"])
        self.assertFalse(payload["chat_template_kwargs"]["enable_thinking"])
        self.assertIn("untrusted_student_answer", payload["messages"][1]["content"])

    def test_dynamic_schema_restricts_rubric_point_ids(self):
        schema = grading_output_schema(valid_request())
        point_enum = schema["properties"]["matched_points"]["items"]["properties"][
            "rubric_point_id"
        ]["enum"]
        self.assertEqual(point_enum, ["p1", "p2"])

    def test_prompt_registry_rejects_unversioned_content_change(self):
        source = Path(settings().prompt_root)
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary)
            shutil.copytree(source, target, dirs_exist_ok=True)
            target.joinpath("base_grading.md").write_text("changed", encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                PromptRegistry(target, settings().prompt_version)

    def test_prompt_registry_routes_by_subject_and_question_type(self):
        registry = PromptRegistry(settings().prompt_root, settings().prompt_version)
        math_request = valid_request()
        math_request.update({"subject": "math", "question_type": "calculation"})
        system = registry.messages(math_request)[0]["content"]
        self.assertIn("【数学·计算与证明题】", system)
        self.assertNotIn("【物理·计算题】", system)

        chinese_request = valid_request()
        chinese_request.update({"subject": "chinese", "question_type": "essay"})
        system = registry.messages(chinese_request)[0]["content"]
        self.assertIn("【语文·作文题】", system)
        self.assertNotIn("【英语·书面表达】", system)

    def test_prompt_registry_fails_closed_for_unsupported_pair(self):
        registry = PromptRegistry(settings().prompt_root, settings().prompt_version)
        request = valid_request()
        request.update({"subject": "history", "question_type": "calculation"})
        with self.assertRaisesRegex(Exception, "no governed prompt is registered"):
            registry.messages(request)


if __name__ == "__main__":
    unittest.main()

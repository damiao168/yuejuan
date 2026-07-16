import json
from pathlib import Path
import tempfile
import unittest

from grading_agent.model import LocalLlamaCppAdapter, PromptRegistry, grading_output_schema

from helpers import settings, valid_raw_output, valid_request


class ModelAdapterTests(unittest.TestCase):
    def test_llama_cpp_request_uses_strict_schema_and_untrusted_answer_boundary(self):
        calls = []

        def transport(url, payload, headers, timeout):
            calls.append((url, payload, headers, timeout))
            return {"choices": [{"message": {"content": json.dumps(valid_raw_output(), ensure_ascii=False)}}]}

        adapter = LocalLlamaCppAdapter(settings(model_api_key="model-secret"), transport=transport)
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
        point_enum = schema["properties"]["matched_points"]["items"]["properties"]["rubric_point_id"]["enum"]
        self.assertEqual(point_enum, ["p1", "p2"])

    def test_prompt_registry_rejects_unversioned_content_change(self):
        source = Path(settings().prompt_root)
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary)
            for item in source.iterdir():
                target.joinpath(item.name).write_bytes(item.read_bytes())
            target.joinpath("base_grading.md").write_text("changed", encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                PromptRegistry(target, settings().prompt_version)


if __name__ == "__main__":
    unittest.main()

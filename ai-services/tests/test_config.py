import os
import unittest
from unittest.mock import patch

from grading_agent.config import Settings


class SettingsTests(unittest.TestCase):
    @patch.dict(
        os.environ,
        {
            "EDUGRADE_GRADING_AGENT_TOKEN": "test-service-token-with-at-least-32-characters",
            "EDUGRADE_GRADING_MODEL_TEMPERATURE": "NaN",
        },
        clear=True,
    )
    def test_non_finite_float_configuration_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "TEMPERATURE"):
            Settings.from_env()

    @patch.dict(
        os.environ,
        {"EDUGRADE_GRADING_AGENT_TOKEN": " " * 32},
        clear=True,
    )
    def test_whitespace_service_token_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "TOKEN"):
            Settings.from_env()

    @patch.dict(
        os.environ,
        {
            "EDUGRADE_GRADING_AGENT_TOKEN": "test-service-token-with-at-least-32-characters",
            "EDUGRADE_GRADING_MODEL_BASE_URL": "not-a-url",
        },
        clear=True,
    )
    def test_invalid_model_url_is_rejected_at_startup(self):
        with self.assertRaisesRegex(ValueError, "MODEL_BASE_URL"):
            Settings.from_env()

    @patch.dict(
        os.environ,
        {
            "EDUGRADE_GRADING_AGENT_TOKEN": "test-service-token-with-at-least-32-characters",
            "EDUGRADE_GRADING_ADAPTER_TYPE": "dashscope_native",
            "EDUGRADE_GRADING_MODEL_BASE_URL": "https://dashscope.aliyuncs.com/api/v1",
            "EDUGRADE_GRADING_MODEL_API_KEY": "synthetic-key-with-16-characters",
        },
        clear=True,
    )
    def test_dashscope_native_accepts_only_the_native_https_endpoint(self):
        current = Settings.from_env()
        self.assertEqual(current.adapter_type, "dashscope_native")

    @patch.dict(
        os.environ,
        {
            "EDUGRADE_GRADING_AGENT_TOKEN": "test-service-token-with-at-least-32-characters",
            "EDUGRADE_GRADING_ADAPTER_TYPE": "dashscope_native",
            "EDUGRADE_GRADING_MODEL_BASE_URL": "https://example.com/api/v1",
            "EDUGRADE_GRADING_MODEL_API_KEY": "synthetic-key-with-16-characters",
        },
        clear=True,
    )
    def test_dashscope_native_rejects_compatible_or_proxy_hosts(self):
        with self.assertRaisesRegex(ValueError, "dashscope_native requires"):
            Settings.from_env()


if __name__ == "__main__":
    unittest.main()

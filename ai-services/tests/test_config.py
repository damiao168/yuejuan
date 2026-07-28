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


if __name__ == "__main__":
    unittest.main()

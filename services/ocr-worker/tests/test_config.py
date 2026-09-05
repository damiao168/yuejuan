import os
import unittest
from unittest.mock import patch

from ocr_worker.config import load_settings

BASE_ENV = {
    "EDUGRADE_API_BASE_URL": "http://api-gateway:8080",
    "EDUGRADE_OCR_WORKER_TENANT_CODE": "demo",
    "EDUGRADE_OCR_WORKER_USERNAME": "ocr-worker",
    "EDUGRADE_OCR_WORKER_PASSWORD": "test-only",
}


class ConfigTests(unittest.TestCase):
    @patch.dict(os.environ, BASE_ENV, clear=True)
    def test_reliable_lease_defaults_are_valid(self):
        settings = load_settings()

        self.assertEqual(settings.batch_size, 1)
        self.assertEqual(settings.lease_seconds, 300)
        self.assertEqual(settings.heartbeat_interval, 10)
        self.assertEqual(settings.heartbeat_timeout, 3)
        self.assertTrue(settings.ocr_runtime_enabled)
        self.assertEqual(settings.cpu_threads, 4)
        self.assertEqual(settings.enable_mkldnn, "auto")
        self.assertTrue(settings.use_textline_orientation)
        self.assertEqual(settings.text_det_limit_side_len, 64)
        self.assertEqual(settings.text_recognition_batch_size, 1)

    @patch.dict(os.environ, {**BASE_ENV, "EDUGRADE_OCR_CPU_THREADS": "0"}, clear=True)
    def test_cpu_threads_are_bounded(self):
        with self.assertRaisesRegex(ValueError, "CPU_THREADS"):
            load_settings()

    @patch.dict(os.environ, {**BASE_ENV, "EDUGRADE_OCR_ENABLE_MKLDNN": "sometimes"}, clear=True)
    def test_mkldnn_mode_is_explicit(self):
        with self.assertRaisesRegex(ValueError, "MKLDNN"):
            load_settings()

    @patch.dict(os.environ, {**BASE_ENV, "EDUGRADE_OCR_TEXT_RECOGNITION_BATCH_SIZE": "0"}, clear=True)
    def test_recognition_batch_size_is_positive(self):
        with self.assertRaisesRegex(ValueError, "RECOGNITION_BATCH_SIZE"):
            load_settings()

    @patch.dict(os.environ, {**BASE_ENV, "EDUGRADE_OCR_BATCH_SIZE": "2"}, clear=True)
    def test_batch_size_greater_than_one_fails_before_claim(self):
        with self.assertRaisesRegex(ValueError, "must be 1"):
            load_settings()

    @patch.dict(
        os.environ,
        {
            **BASE_ENV,
            "EDUGRADE_OCR_LEASE_SECONDS": "30",
            "EDUGRADE_OCR_HEARTBEAT_INTERVAL": "28",
            "EDUGRADE_OCR_HEARTBEAT_TIMEOUT": "3",
        },
        clear=True,
    )
    def test_heartbeat_timeout_must_finish_before_lease_expires(self):
        with self.assertRaisesRegex(ValueError, "interval plus timeout"):
            load_settings()

    @patch.dict(os.environ, {**BASE_ENV, "EDUGRADE_OCR_MIN_CONFIDENCE": "NaN"}, clear=True)
    def test_non_finite_confidence_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "MIN_CONFIDENCE"):
            load_settings()

    @patch.dict(os.environ, {**BASE_ENV, "EDUGRADE_OCR_POLL_INTERVAL": "-1"}, clear=True)
    def test_non_positive_poll_interval_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "POLL_INTERVAL"):
            load_settings()

    @patch.dict(
        os.environ,
        {**BASE_ENV, "EDUGRADE_OCR_RUNTIME_ENABLED": "false", "EDUGRADE_MATH_RUNTIME_ENABLED": "false"},
        clear=True,
    )
    def test_at_least_one_runtime_must_be_enabled(self):
        with self.assertRaisesRegex(ValueError, "at least one"):
            load_settings()


if __name__ == "__main__":
    unittest.main()

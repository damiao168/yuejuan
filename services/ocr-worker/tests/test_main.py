import unittest
from types import SimpleNamespace
from unittest.mock import Mock, patch

from ocr_worker import __main__ as worker_main


def settings():
    return SimpleNamespace(
        api_base_url="http://api-gateway:8080",
        tenant_code="demo",
        username="ocr-worker",
        password="test-only",
        device="cpu",
        engine="paddleocr",
        engine_version="pp-ocrv5",
        model_version="ppocr-v5-server",
        worker_id="worker-1",
        batch_size=1,
        preprocess_profile="default",
        lease_seconds=300,
        heartbeat_interval=10,
        heartbeat_timeout=3,
        poll_interval=5,
    )


class MainTests(unittest.TestCase):
    def test_engine_is_ready_before_login_or_claim(self):
        events = []
        engine = Mock(model_version="ppocr-v5-server", device="cpu")
        engine.initialize.side_effect = lambda: events.append("engine-ready")
        client = Mock(token=None)
        client.login.side_effect = lambda: events.append("login")
        runner = Mock()

        def stop_after_first_claim():
            events.append("claim")
            raise KeyboardInterrupt

        runner.process_once.side_effect = stop_after_first_claim

        with (
            patch.object(worker_main, "load_settings", return_value=settings()),
            patch.object(worker_main, "PaddleOCREngine", return_value=engine),
            patch.object(worker_main, "EduGradeClient", return_value=client),
            patch.object(worker_main, "OCRRunner", return_value=runner),
            self.assertRaises(KeyboardInterrupt),
        ):
            worker_main.main()

        self.assertEqual(events, ["engine-ready", "login", "claim"])

    def test_engine_initialization_failure_prevents_authentication(self):
        engine = Mock(model_version="ppocr-v5-server", device="cpu")
        engine.initialize.side_effect = RuntimeError("model unavailable")
        client_class = Mock()

        with (
            patch.object(worker_main, "load_settings", return_value=settings()),
            patch.object(worker_main, "PaddleOCREngine", return_value=engine),
            patch.object(worker_main, "EduGradeClient", client_class),
            self.assertRaisesRegex(RuntimeError, "model unavailable"),
        ):
            worker_main.main()

        client_class.assert_not_called()


if __name__ == "__main__":
    unittest.main()

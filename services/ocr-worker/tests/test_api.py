import json
import unittest
from unittest.mock import patch

from ocr_worker.api import APIError, EduGradeClient


class FakeResponse:
    def __enter__(self):
        return self

    def __exit__(self, _exc_type, _exc, _traceback):
        return None

    def read(self):
        return b"{}"


class APITests(unittest.TestCase):
    @patch("ocr_worker.api.request.urlopen", return_value=FakeResponse())
    def test_heartbeat_renews_configured_lease_with_short_timeout(self, urlopen):
        client = EduGradeClient(
            base_url="http://api-gateway:8080",
            tenant_code="demo",
            username="ocr-worker",
            password="test-only",
            token="worker-token",
        )

        client.heartbeat_task("runtime-1", "lease-1", "worker-1", 300, 3.0)

        req = urlopen.call_args.args[0]
        payload = json.loads(req.data.decode("utf-8"))
        self.assertEqual(payload["lease_seconds"], 300)
        self.assertEqual(payload["state"], "running")
        self.assertEqual(urlopen.call_args.kwargs["timeout"], 3.0)

    def test_download_rejects_cross_origin_url_before_sending_worker_token(self):
        client = EduGradeClient("http://api-gateway:8080", "demo", "worker", "secret", token="worker-token")

        with self.assertRaisesRegex(APIError, "configured API origin"):
            client.download("https://attacker.invalid/collect")

    @patch("ocr_worker.api.request.urlopen", return_value=FakeResponse())
    def test_target_tenant_header_is_sent_for_claimed_task_requests(self, urlopen):
        client = EduGradeClient(
            base_url="http://api-gateway:8080",
            tenant_code="platform",
            username="ocr-worker",
            password="test-only",
            token="worker-token",
        )

        client.start_task("task-1", "tenant-1")

        req = urlopen.call_args.args[0]
        self.assertEqual(req.headers["X-edugrade-tenant-id"], "tenant-1")


if __name__ == "__main__":
    unittest.main()

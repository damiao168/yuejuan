import json
import unittest
from unittest.mock import patch

from ocr_worker.api import EduGradeClient


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


if __name__ == "__main__":
    unittest.main()

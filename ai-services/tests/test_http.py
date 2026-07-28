import json
import threading
import unittest
from urllib import error as urlerror
from urllib import request as urlrequest

from grading_agent.app import GradingAgentApplication
from grading_agent.server import GradingAgentHTTPServer
from helpers import FakeModel, settings, valid_request


class HTTPTests(unittest.TestCase):
    def setUp(self):
        self.settings = settings()
        self.app = GradingAgentApplication(self.settings, model=FakeModel())
        self.server = GradingAgentHTTPServer(("127.0.0.1", 0), self.app)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base_url = f"http://127.0.0.1:{self.server.server_port}"

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def test_health_and_readiness(self):
        with urlrequest.urlopen(f"{self.base_url}/health", timeout=2) as response:
            health = json.loads(response.read())
        with urlrequest.urlopen(f"{self.base_url}/ready", timeout=2) as response:
            ready = json.loads(response.read())
        self.assertEqual(health["mode"], "shadow")
        self.assertEqual(ready["status"], "ready")

    def test_grade_requires_service_auth_and_idempotency(self):
        body = json.dumps(valid_request(), ensure_ascii=False).encode("utf-8")
        unauthorized = urlrequest.Request(
            f"{self.base_url}/grading/grade",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with self.assertRaises(urlerror.HTTPError) as caught:
            urlrequest.urlopen(unauthorized, timeout=2)
        self.assertEqual(caught.exception.code, 401)

        grading_request = valid_request()
        authorized = urlrequest.Request(
            f"{self.base_url}/grading/grade",
            data=json.dumps(grading_request, ensure_ascii=False).encode("utf-8"),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.settings.service_token}",
                "Idempotency-Key": grading_request["request_id"],
            },
            method="POST",
        )
        with urlrequest.urlopen(authorized, timeout=2) as response:
            suggestion = json.loads(response.read())
            replay = response.headers["Idempotent-Replay"]
        self.assertEqual(suggestion["status"], "suggestion")
        self.assertEqual(replay, "false")


if __name__ == "__main__":
    unittest.main()

import unittest

from ocr_worker.api import APIError, AuthenticationError
from ocr_worker.engine import OCRBlock
from ocr_worker.runner import OCRRunner, WorkerConfig


class FakeAPI:
    def __init__(self):
        self.started = []
        self.completed = []
        self.failed = []
        self.downloads = {}
        self.pending_error = None
        self.heartbeats = []
        self.runtime_failed = []

    def claim_tasks(self, worker_instance_id, limit, lease_seconds):
        if self.pending_error:
            raise self.pending_error
        return [{"id": "runtime-1", "source_id": "task-1", "lease_token": "lease-1", "payload": {"ocr_task_id": "task-1"}}]

    def heartbeat_task(self, runtime_task_id, lease_token, worker_instance_id):
        self.heartbeats.append((runtime_task_id, lease_token, worker_instance_id))

    def start_task(self, task_id):
        self.started.append(task_id)

    def get_task_input(self, task_id):
        return {"pages": [{"id": "page-1", "download_url": "/files/file-1/download", "file_asset_id": "file-1"}]}

    def download(self, url):
        value = self.downloads.get(url, b"image")
        if isinstance(value, Exception):
            raise value
        return value

    def complete_task(self, task_id, payload):
        self.completed.append((task_id, payload))

    def fail_task(self, task_id, message, runtime_task_id, lease_token, retryable):
        self.failed.append((task_id, message))
        self.runtime_failed.append((runtime_task_id, lease_token, message, retryable, 0))


class FakeEngine:
    def __init__(self, blocks):
        self.blocks = blocks

    def recognize(self, image_bytes):
        return self.blocks


class RunnerTests(unittest.TestCase):
    def test_low_confidence_result_is_completed_for_human_review_gate(self):
        api = FakeAPI()
        engine = FakeEngine([OCRBlock(text="student answer", bbox=[1, 2, 30, 10], confidence=0.52)])
        runner = OCRRunner(api=api, engine=engine, config=WorkerConfig(worker_id="worker-a", batch_size=1))

        processed = runner.process_once()

        self.assertEqual(processed, 1)
        self.assertEqual(api.started, ["task-1"])
        self.assertEqual(api.heartbeats, [("runtime-1", "lease-1", "worker-a")])
        self.assertEqual(api.failed, [])
        task_id, payload = api.completed[0]
        self.assertEqual(task_id, "task-1")
        self.assertEqual(payload["worker_id"], "worker-a")
        self.assertEqual(payload["results"][0]["confidence"], 0.52)
        self.assertEqual(payload["runtime_task_id"], "runtime-1")

    def test_empty_ocr_result_fails_task(self):
        api = FakeAPI()
        runner = OCRRunner(api=api, engine=FakeEngine([]), config=WorkerConfig(worker_id="worker-a", batch_size=1))

        processed = runner.process_once()

        self.assertEqual(processed, 1)
        self.assertEqual(api.completed, [])
        self.assertEqual(api.failed, [("task-1", "empty_ocr_result")])
        self.assertEqual(api.runtime_failed[0][2:4], ("empty_ocr_result", False))

    def test_download_failure_fails_task(self):
        api = FakeAPI()
        api.downloads["/files/file-1/download"] = APIError("download failed")
        engine = FakeEngine([OCRBlock(text="never used", bbox=[1, 2, 30, 10], confidence=0.9)])
        runner = OCRRunner(api=api, engine=engine, config=WorkerConfig(worker_id="worker-a", batch_size=1))

        processed = runner.process_once()

        self.assertEqual(processed, 1)
        self.assertEqual(api.completed, [])
        self.assertEqual(api.failed, [("task-1", "download_failed")])
        self.assertEqual(api.runtime_failed[0][2:4], ("download_failed", True))

    def test_authentication_failure_stops_poll_cycle(self):
        api = FakeAPI()
        api.pending_error = AuthenticationError("unauthorized")
        engine = FakeEngine([OCRBlock(text="unused", bbox=[1, 2, 30, 10], confidence=0.9)])
        runner = OCRRunner(api=api, engine=engine, config=WorkerConfig(worker_id="worker-a", batch_size=1))

        with self.assertRaises(AuthenticationError):
            runner.process_once()
        self.assertEqual(api.started, [])
        self.assertEqual(api.completed, [])
        self.assertEqual(api.failed, [])

    def test_engine_failure_is_reported_as_retryable(self):
        class FailingEngine:
            def recognize(self, image_bytes):
                raise RuntimeError("engine unavailable")

        api = FakeAPI()
        runner = OCRRunner(api=api, engine=FailingEngine(), config=WorkerConfig(worker_id="worker-a", batch_size=1))

        processed = runner.process_once()

        self.assertEqual(processed, 1)
        self.assertEqual(api.completed, [])
        self.assertEqual(api.runtime_failed[0][2:4], ("ocr_engine_failed", True))

    def test_invalid_nan_result_is_not_submitted(self):
        api = FakeAPI()
        engine = FakeEngine([OCRBlock(text="invalid", bbox=[1, 2, 30, 10], confidence=float("nan"))])
        runner = OCRRunner(api=api, engine=engine, config=WorkerConfig(worker_id="worker-a", batch_size=1))

        runner.process_once()

        self.assertEqual(api.completed, [])
        self.assertEqual(api.runtime_failed[0][2:4], ("empty_ocr_result", False))


if __name__ == "__main__":
    unittest.main()

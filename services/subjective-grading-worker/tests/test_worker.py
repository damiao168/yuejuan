from __future__ import annotations

from subjective_grading_worker.config import Settings, validate_settings
from subjective_grading_worker.runner import Runner


class FakeAPI:
    def __init__(self, task):
        self.task = task
        self.calls = []

    def claim(self, worker_id, lease_seconds):
        self.calls.append(("claim", worker_id, lease_seconds))
        return [self.task]

    def heartbeat(self, task_id, token, worker_id, lease_seconds, timeout):
        self.calls.append(("heartbeat", task_id, token, worker_id))

    def execute(self, run_id, task_id, lease):
        self.calls.append(("execute", run_id, task_id, lease))
        return {"output": {"request_id": "request-1", "model_version": "model-v1"}}

    def complete(self, run_id, task_id, lease, output, duration_ms):
        self.calls.append(("complete", run_id, task_id, lease, output["request_id"]))

    def fail(self, run_id, task_id, lease, code, retryable, duration_ms):
        self.calls.append(("fail", run_id, task_id, lease, code, retryable))


def settings() -> Settings:
    return Settings("http://127.0.0.1:8088", "demo", "worker", "secret", heartbeat_interval=1, heartbeat_timeout=0.5)


def test_runner_executes_and_completes_domain_result():
    api = FakeAPI({"id": "task-1", "source_id": "run-1", "source_type": "subjective_grading_run", "lease_token": "lease-1"})
    assert Runner(api=api, settings=settings()).process_once() == 1
    assert any(call[0] == "execute" for call in api.calls)
    assert any(call[0] == "complete" for call in api.calls)
    assert not any(call[0] == "fail" for call in api.calls)


def test_runner_rejects_invalid_runtime_source():
    api = FakeAPI({"id": "task-1", "source_id": "run-1", "source_type": "ocr_task", "lease_token": "lease-1"})
    Runner(api=api, settings=settings()).process_once()
    assert any(call[0] == "fail" and call[4] == "invalid_subjective_runtime_payload" for call in api.calls)


def test_settings_reject_unsafe_lease_heartbeat():
    invalid = Settings("http://127.0.0.1:8088", "demo", "worker", "secret", lease_seconds=30, heartbeat_interval=29, heartbeat_timeout=2)
    try:
        validate_settings(invalid)
    except ValueError as exc:
        assert "heartbeat" in str(exc)
    else:
        raise AssertionError("expected heartbeat validation failure")

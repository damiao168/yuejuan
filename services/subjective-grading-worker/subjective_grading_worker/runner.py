from __future__ import annotations

import threading
import time
from typing import Any, Self

from .api import APIError
from .config import Settings


class Runner:
    def __init__(self, *, api: Any, settings: Settings) -> None:
        self.api = api
        self.settings = settings

    def process_once(self) -> int:
        processed = 0
        for task in self.api.claim(self.settings.worker_id, self.settings.lease_seconds):
            self._process(task)
            processed += 1
        return processed

    def _process(self, task: dict[str, Any]) -> None:
        started = time.monotonic()
        runtime_id = str(task.get("id", ""))
        lease = str(task.get("lease_token", ""))
        run_id = str(task.get("source_id", ""))
        activate_task = getattr(self.api, "activate_task", None)
        if callable(activate_task):
            activate_task(task, self.settings.worker_id)
        if task.get("source_type") != "subjective_grading_run" or not runtime_id or not lease or not run_id:
            fail_task = getattr(self.api, "fail_task", None)
            if not runtime_id or not lease:
                raise APIError("subjective grading task is missing its runtime capability")
            if callable(fail_task):
                fail_task(runtime_id, lease, "invalid_subjective_runtime_payload", False)
            else:
                self.api.fail(run_id, runtime_id, lease, "invalid_subjective_runtime_payload", False, 0)
            return
        with _Heartbeat(self.api, runtime_id, lease, self.settings) as heartbeat:
            try:
                heartbeat.raise_if_failed()
                response = self.api.execute(run_id, runtime_id, lease)
                output = response.get("output")
                if not isinstance(output, dict):
                    raise APIError("subjective agent response did not include output")
                heartbeat.raise_if_failed()
                self.api.complete(run_id, runtime_id, lease, output, int((time.monotonic() - started) * 1000))
            except APIError:
                heartbeat.stop()
                self.api.fail(run_id, runtime_id, lease, "subjective_agent_failed", True, int((time.monotonic() - started) * 1000))


class _Heartbeat:
    def __init__(self, api: Any, task_id: str, token: str, settings: Settings) -> None:
        self.api, self.task_id, self.token, self.settings = api, task_id, token, settings
        self._stop = threading.Event()
        self._error: Exception | None = None
        self._thread: threading.Thread | None = None

    def __enter__(self) -> Self:
        self.api.heartbeat(self.task_id, self.token, self.settings.worker_id, self.settings.lease_seconds, self.settings.heartbeat_timeout)
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()
        return self

    def __exit__(self, *_: object) -> None:
        self.stop()

    def stop(self) -> None:
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=self.settings.heartbeat_timeout + 0.5)
            self._thread = None
        self.raise_if_failed()

    def raise_if_failed(self) -> None:
        if self._error:
            raise self._error

    def _run(self) -> None:
        while not self._stop.wait(self.settings.heartbeat_interval):
            try:
                self.api.heartbeat(self.task_id, self.token, self.settings.worker_id, self.settings.lease_seconds, self.settings.heartbeat_timeout)
            except Exception as exc:  # noqa: BLE001
                self._error = exc
                return

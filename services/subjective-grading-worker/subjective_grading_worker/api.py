from __future__ import annotations

import json
from dataclasses import dataclass
from urllib import error, request
from urllib.parse import urlsplit
from typing import Any


class APIError(RuntimeError):
    pass


@dataclass
class EduGradeClient:
    base_url: str
    tenant_code: str
    username: str
    password: str
    token: str | None = None

    def login(self) -> None:
        response = self._request("POST", "/api/v1/auth/login", {"tenant_code": self.tenant_code, "username": self.username, "password": self.password}, auth=False)
        self.token = str(response.get("access_token", ""))
        if not self.token:
            raise APIError("login response did not include access_token")

    def claim(self, worker_id: str, lease_seconds: int) -> list[dict[str, Any]]:
        response = self._request("POST", "/api/v1/internal/worker/tasks/claim", {"queue_name": "subjective-grading", "worker_service": "subjective-grading-worker", "worker_instance_id": worker_id, "limit": 1, "lease_seconds": lease_seconds})
        tasks = response.get("tasks", [])
        return tasks if isinstance(tasks, list) else []

    def heartbeat(self, task_id: str, token: str, worker_id: str, lease_seconds: int, timeout: float) -> None:
        self._request("POST", f"/api/v1/internal/worker/tasks/{task_id}/heartbeat", {"lease_token": token, "worker_service": "subjective-grading-worker", "worker_instance_id": worker_id, "state": "running", "lease_seconds": lease_seconds}, timeout=timeout)

    def execute(self, run_id: str, task_id: str, lease_token: str) -> dict[str, Any]:
        return self._request("POST", f"/api/v1/internal/subjective-grading/runs/{run_id}/execute", {"task_id": task_id, "lease_token": lease_token})

    def complete(self, run_id: str, task_id: str, lease_token: str, output: dict[str, Any], duration_ms: int) -> None:
        self._request("POST", f"/api/v1/internal/subjective-grading/runs/{run_id}/result", {"task_id": task_id, "lease_token": lease_token, "result_schema_version": "subjective-grade-result-v1", "duration_ms": duration_ms, "output": output})

    def fail(self, run_id: str, task_id: str, lease_token: str, code: str, retryable: bool, duration_ms: int) -> None:
        self._request("POST", f"/api/v1/internal/subjective-grading/runs/{run_id}/failure", {"task_id": task_id, "lease_token": lease_token, "retryable": retryable, "error_code": code, "error_detail": {}, "duration_ms": duration_ms})

    def _request(self, method: str, path: str, payload: dict[str, Any], *, auth: bool = True, timeout: float = 60) -> dict[str, Any]:
        url = _trusted_url(self.base_url, path)
        req = request.Request(url, data=json.dumps(payload).encode(), headers={"Accept": "application/json", "Content-Type": "application/json", **({"Authorization": f"Bearer {self.token}"} if auth and self.token else {})}, method=method)
        try:
            with request.urlopen(req, timeout=timeout) as response:
                return json.loads(response.read().decode() or "{}")
        except error.HTTPError as exc:
            raise APIError(f"api request failed: {exc.code}") from exc
        except (OSError, ValueError) as exc:
            raise APIError("api request failed") from exc


def _trusted_url(base_url: str, path: str) -> str:
    base = urlsplit(base_url)
    target = urlsplit(f"{base_url.rstrip('/')}/{path.lstrip('/')}")
    if (target.scheme, target.hostname, target.port) != (base.scheme, base.hostname, base.port) or target.username or target.password or target.fragment:
        raise APIError("service URL must remain on configured API origin")
    return target.geturl()

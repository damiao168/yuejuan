from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any
from urllib import error, request
from urllib.parse import urlsplit


class APIError(RuntimeError):
    pass


class AuthenticationError(APIError):
    pass


@dataclass
class EduGradeClient:
    base_url: str
    tenant_code: str
    username: str
    password: str
    token: str | None = None

    def login(self) -> None:
        response = self._request(
            "POST",
            "/api/v1/auth/login",
            {
                "tenant_code": self.tenant_code,
                "username": self.username,
                "password": self.password,
            },
            require_auth=False,
        )
        token = response.get("access_token")
        if not isinstance(token, str) or not token:
            raise AuthenticationError("login response did not include access_token")
        self.token = token

    def list_pending(self, limit: int) -> list[dict[str, Any]]:
        response = self._request("GET", f"/api/v1/ocr-tasks/pending?limit={limit}")
        tasks = response.get("tasks", [])
        return tasks if isinstance(tasks, list) else []

    def claim_tasks(self, worker_instance_id: str, limit: int, lease_seconds: int) -> list[dict[str, Any]]:
        response = self._request(
            "POST",
            "/api/v1/internal/worker/tasks/claim",
            {
                "queue_name": "ocr",
                "worker_service": "ocr-worker",
                "worker_instance_id": worker_instance_id,
                "limit": limit,
                "lease_seconds": lease_seconds,
            },
        )
        tasks = response.get("tasks", [])
        return tasks if isinstance(tasks, list) else []

    def heartbeat_task(
        self,
        runtime_task_id: str,
        lease_token: str,
        worker_instance_id: str,
        lease_seconds: int,
        timeout_seconds: float,
        tenant_id: str | None = None,
    ) -> None:
        self._request(
            "POST",
            f"/api/v1/internal/worker/tasks/{runtime_task_id}/heartbeat",
            {
                "lease_token": lease_token,
                "worker_service": "ocr-worker",
                "worker_instance_id": worker_instance_id,
                "state": "running",
                "lease_seconds": lease_seconds,
            },
            timeout=timeout_seconds,
            tenant_id=tenant_id,
        )

    def start_task(self, task_id: str, tenant_id: str | None = None) -> None:
        self._request("POST", f"/api/v1/ocr-tasks/{task_id}/start", {}, tenant_id=tenant_id)

    def get_task_input(self, task_id: str, tenant_id: str | None = None) -> dict[str, Any]:
        return self._request("GET", f"/api/v1/ocr-tasks/{task_id}/input", tenant_id=tenant_id)

    def complete_task(self, task_id: str, payload: dict[str, Any], tenant_id: str | None = None) -> None:
        self._request("POST", f"/api/v1/ocr-tasks/{task_id}/results", payload, tenant_id=tenant_id)

    def fail_task(
        self,
        task_id: str,
        message: str,
        runtime_task_id: str,
        lease_token: str,
        retryable: bool,
        tenant_id: str | None = None,
    ) -> None:
        self._request(
            "POST",
            f"/api/v1/ocr-tasks/{task_id}/fail",
            {
                "error_message": message,
                "runtime_task_id": runtime_task_id,
                "runtime_lease_token": lease_token,
                "retryable": retryable,
            },
            tenant_id=tenant_id,
        )

    def download(self, url: str, tenant_id: str | None = None) -> bytes:
        req = self._build_request("GET", url, None, tenant_id=tenant_id)
        try:
            with request.urlopen(req, timeout=60) as response:
                return response.read()
        except error.HTTPError as exc:
            if exc.code in (401, 403):
                raise AuthenticationError(f"download unauthorized: {exc.code}") from exc
            raise APIError(f"download failed: {exc.code}") from exc
        except OSError as exc:
            raise APIError("download failed") from exc

    def _request(
        self,
        method: str,
        path: str,
        payload: dict[str, Any] | None = None,
        require_auth: bool = True,
        timeout: float = 60,
        tenant_id: str | None = None,
    ) -> dict[str, Any]:
        req = self._build_request(
            method,
            path,
            payload if method != "GET" else None,
            require_auth=require_auth,
            tenant_id=tenant_id,
        )
        try:
            with request.urlopen(req, timeout=timeout) as response:
                raw = response.read()
        except error.HTTPError as exc:
            if exc.code in (401, 403):
                raise AuthenticationError(f"unauthorized: {exc.code}") from exc
            raise APIError(f"api request failed: {exc.code}") from exc
        except OSError as exc:
            raise APIError("api request failed") from exc
        if not raw:
            return {}
        return json.loads(raw.decode("utf-8"))

    def _build_request(
        self,
        method: str,
        path_or_url: str,
        payload: dict[str, Any] | None,
        require_auth: bool = True,
        tenant_id: str | None = None,
    ) -> request.Request:
        url = _trusted_service_url(self.base_url, path_or_url)
        body = None if payload is None else json.dumps(payload).encode("utf-8")
        headers = {"Accept": "application/json"}
        if payload is not None:
            headers["Content-Type"] = "application/json"
        if require_auth and self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        if tenant_id:
            headers["X-EduGrade-Tenant-ID"] = tenant_id
        return request.Request(url, data=body, headers=headers, method=method)


def _trusted_service_url(base_url: str, path_or_url: str) -> str:
    candidate = path_or_url if path_or_url.startswith(("http://", "https://")) else f"{base_url.rstrip('/')}/{path_or_url.lstrip('/')}"
    try:
        base = urlsplit(base_url)
        target = urlsplit(candidate)
        default_ports = {"http": 80, "https": 443}
        base_origin = (base.scheme.lower(), (base.hostname or "").lower(), base.port or default_ports.get(base.scheme.lower()))
        target_origin = (
            target.scheme.lower(),
            (target.hostname or "").lower(),
            target.port or default_ports.get(target.scheme.lower()),
        )
    except ValueError as exc:
        raise APIError("service URL is invalid") from exc
    if (
        target.scheme.lower() not in default_ports
        or target_origin != base_origin
        or target.username is not None
        or target.password is not None
        or target.fragment
    ):
        raise APIError("service URL must remain on the configured API origin")
    return candidate

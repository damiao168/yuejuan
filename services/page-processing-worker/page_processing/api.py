from __future__ import annotations

import json
import math
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from email.utils import parsedate_to_datetime
from typing import Any
from urllib import error, request
from urllib.parse import urlsplit


class APIError(RuntimeError):
    def __init__(self, message: str, *, status_code: int | None = None, retry_after: float | None = None) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.retry_after = retry_after


@dataclass
class Client:
    base_url: str
    tenant_code: str
    username: str
    password: str
    token: str | None = None

    def login(self) -> None:
        payload = self._json("POST", "/api/v1/auth/login", {"tenant_code": self.tenant_code, "username": self.username, "password": self.password}, auth=False)
        token = payload.get("access_token")
        if not isinstance(token, str) or not token:
            raise APIError("login_missing_access_token")
        self.token = token

    def claim(self, worker_id: str, limit: int, lease_seconds: int) -> list[dict[str, Any]]:
        payload = self._json("POST", "/api/v1/internal/worker/tasks/claim", {"queue_name": "page-processing", "worker_service": "page-processing", "worker_instance_id": worker_id, "limit": limit, "lease_seconds": lease_seconds})
        tasks = payload.get("tasks", [])
        return tasks if isinstance(tasks, list) else []

    def heartbeat(self, task: dict[str, Any], worker_id: str, lease_seconds: int, timeout: float | None = None) -> None:
        self._json("POST", f"/api/v1/internal/worker/tasks/{task['id']}/heartbeat", {"lease_token": task["lease_token"], "worker_service": "page-processing", "worker_instance_id": worker_id, "state": "running", "progress": {"phase": "processing"}, "lease_seconds": lease_seconds}, timeout=timeout)

    def download(self, path: str) -> bytes:
        req = self._request("GET", path, None)
        try:
            with request.urlopen(req, timeout=120) as response:
                return response.read()
        except (error.HTTPError, OSError) as exc:
            raise APIError("source_download_failed") from exc

    def upload_page(self, task: dict[str, Any], page_index: int, png: bytes) -> dict[str, Any]:
        payload = task.get("payload") or {}
        return self.upload_asset(task, "capture_page_decoded", str(payload["capture_file_id"]), f"page-{page_index:04d}.png", png)

    def upload_asset(self, task: dict[str, Any], owner_type: str, owner_id: str, filename: str, png: bytes) -> dict[str, Any]:
        payload = task.get("payload") or {}
        boundary = "----edugrade-" + uuid.uuid4().hex
        fields = {
            "owner_type": owner_type,
            "owner_id": owner_id,
            "exam_id": str(payload["exam_id"]),
        }
        body = _multipart(boundary, fields, filename, png)
        req = self._request("POST", "/api/v1/files", None)
        req.data = body
        req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
        req.add_header("Content-Length", str(len(body)))
        try:
            with request.urlopen(req, timeout=180) as response:
                result = json.loads(response.read().decode("utf-8"))
        except error.HTTPError as exc:
            if exc.code == 409:
                try:
                    duplicate = json.loads(exc.read().decode("utf-8"))
                    existing = duplicate.get("existing_file") or {}
                except (UnicodeDecodeError, json.JSONDecodeError):
                    existing = {}
                if existing.get("id") and existing.get("hash_sha256"):
                    return existing
            raise APIError(f"decoded_page_upload_failed:{exc.code}") from exc
        file_info = result.get("file") or {}
        if not file_info.get("id") or not file_info.get("hash_sha256"):
            raise APIError("decoded_page_upload_invalid_response")
        return file_info

    def complete(self, capture_file_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/capture/files/{capture_file_id}/result", payload)

    def fail(self, capture_file_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/capture/files/{capture_file_id}/fail", payload)

    def complete_registration(self, run_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/page-registration-runs/{run_id}/result", payload)

    def fail_registration(self, run_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/page-registration-runs/{run_id}/fail", payload)

    def complete_correction(self, correction_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/page-registration-corrections/{correction_id}/result", payload)

    def fail_correction(self, correction_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/page-registration-corrections/{correction_id}/failure", payload)

    def complete_omr(self, run_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/omr-runs/{run_id}/result", payload)

    def fail_omr(self, run_id: str, payload: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/omr-runs/{run_id}/failure", payload)

    def fail_task(self, task: dict[str, Any], error_code: str, detail: dict[str, Any]) -> None:
        self._json("POST", f"/api/v1/internal/worker/tasks/{task['id']}/fail", {"lease_token": task["lease_token"], "retryable": True, "error_code": error_code, "error_detail": detail, "duration_ms": 0})

    def _json(self, method: str, path: str, payload: dict[str, Any], auth: bool = True, timeout: float | None = None) -> dict[str, Any]:
        req = self._request(method, path, json.dumps(payload).encode("utf-8"), auth=auth)
        req.add_header("Content-Type", "application/json")
        try:
            with request.urlopen(req, timeout=120 if timeout is None else timeout) as response:
                raw = response.read()
        except error.HTTPError as exc:
            raise APIError(
                f"api_request_failed:{exc.code}",
                status_code=exc.code,
                retry_after=_retry_after_seconds(exc.headers),
            ) from exc
        except (error.URLError, OSError, TimeoutError) as exc:
            raise APIError("api_request_failed:transport") from exc
        return json.loads(raw.decode("utf-8")) if raw else {}

    def _request(self, method: str, path: str, body: bytes | None, auth: bool = True) -> request.Request:
        url = _trusted_service_url(self.base_url, path)
        headers = {"Accept": "application/json"}
        if auth and self.token:
            headers["Authorization"] = "Bearer " + self.token
        return request.Request(url, data=body, headers=headers, method=method)


def _multipart(boundary: str, fields: dict[str, str], filename: str, data: bytes) -> bytes:
    chunks: list[bytes] = []
    for name, value in fields.items():
        chunks.extend([f"--{boundary}\r\n".encode(), f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode(), value.encode(), b"\r\n"])
    chunks.extend([f"--{boundary}\r\n".encode(), f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'.encode(), b"Content-Type: image/png\r\n\r\n", data, b"\r\n", f"--{boundary}--\r\n".encode()])
    return b"".join(chunks)


def _retry_after_seconds(headers: Any) -> float | None:
    """Parse Retry-After as either delta seconds or an HTTP date."""
    if headers is None:
        return None
    raw = headers.get("Retry-After")
    if raw is None:
        return None
    value = str(raw).strip()
    if not value:
        return None
    try:
        seconds = float(value)
    except ValueError:
        seconds = math.nan
    if math.isfinite(seconds) and seconds >= 0:
        return seconds
    try:
        retry_at = parsedate_to_datetime(value)
    except (TypeError, ValueError, OverflowError):
        return None
    if retry_at.tzinfo is None:
        retry_at = retry_at.replace(tzinfo=timezone.utc)
    return max(0.0, (retry_at - datetime.now(timezone.utc)).total_seconds())


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

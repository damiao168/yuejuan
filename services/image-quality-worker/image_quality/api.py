from __future__ import annotations

import json
import uuid
from dataclasses import dataclass
from typing import Any
from urllib import error, request


class APIError(RuntimeError):
    pass


class AuthenticationError(APIError):
    pass


@dataclass
class EduGradeImageQualityClient:
    base_url: str
    tenant_code: str
    username: str
    password: str
    token: str | None = None

    def login(self) -> None:
        response = self._request(
            "POST",
            "/api/v1/auth/login",
            {"tenant_code": self.tenant_code, "username": self.username, "password": self.password},
            require_auth=False,
        )
        token = response.get("access_token")
        if not isinstance(token, str) or not token:
            raise AuthenticationError("login response did not include access_token")
        self.token = token

    def claim_jobs(self, worker_instance_id: str, limit: int, lease_seconds: int) -> list[dict[str, Any]]:
        response = self._request(
            "POST",
            "/api/v1/internal/image-quality/jobs/claim",
            {"worker_instance_id": worker_instance_id, "limit": limit, "lease_seconds": lease_seconds},
        )
        jobs = response.get("jobs", [])
        return jobs if isinstance(jobs, list) else []

    def download(self, url: str) -> bytes:
        req = self._build_request("GET", url, None)
        try:
            with request.urlopen(req, timeout=60) as response:
                return response.read()
        except error.HTTPError as exc:
            if exc.code in (401, 403):
                raise AuthenticationError(f"download unauthorized: {exc.code}") from exc
            raise APIError(f"download failed: {exc.code}") from exc
        except OSError as exc:
            raise APIError("download failed") from exc

    def request_normalized_asset(self, run_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", f"/api/v1/internal/image-quality/runs/{run_id}/normalized-assets", payload)

    def upload_normalized(self, slot: dict[str, Any], job: dict[str, Any], normalized_png: bytes, sha256: str) -> str:
        upload_url = str(slot.get("upload_url") or "/api/v1/files")
        boundary = "----edugrade-" + uuid.uuid4().hex
        fields = {
            "owner_type": "submission_page_normalized",
            "owner_id": str(job["submission_page_id"]),
            "submission_id": str(job["submission_id"]),
        }
        body = _multipart_body(boundary, fields, "file", "normalized-page.png", "image/png", normalized_png)
        req = self._build_request("POST", upload_url, None)
        req.data = body
        req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
        req.add_header("Content-Length", str(len(body)))
        try:
            with request.urlopen(req, timeout=120) as response:
                raw = response.read()
        except error.HTTPError as exc:
            raise APIError(f"normalized upload failed: {exc.code}") from exc
        payload = json.loads(raw.decode("utf-8"))
        file_info = payload.get("file") or {}
        if file_info.get("hash_sha256") not in (sha256, "sha256:" + sha256):
            raise APIError("normalized upload hash mismatch")
        file_id = file_info.get("id")
        if not isinstance(file_id, str) or not file_id:
            raise APIError("normalized upload response missing file id")
        return file_id

    def submit_result(self, run_id: str, payload: dict[str, Any]) -> None:
        self._request("POST", f"/api/v1/internal/image-quality/runs/{run_id}/result", payload)

    def submit_failure(self, run_id: str, payload: dict[str, Any]) -> None:
        self.submit_result(run_id, payload)

    def _request(self, method: str, path: str, payload: dict[str, Any] | None = None, require_auth: bool = True) -> dict[str, Any]:
        req = self._build_request(method, path, payload if method != "GET" else None, require_auth=require_auth)
        try:
            with request.urlopen(req, timeout=60) as response:
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

    def _build_request(self, method: str, path_or_url: str, payload: dict[str, Any] | None, require_auth: bool = True) -> request.Request:
        url = path_or_url if path_or_url.startswith("http") else f"{self.base_url}{path_or_url}"
        body = None if payload is None else json.dumps(payload).encode("utf-8")
        headers = {"Accept": "application/json"}
        if payload is not None:
            headers["Content-Type"] = "application/json"
        if require_auth and self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        return request.Request(url, data=body, headers=headers, method=method)


def _multipart_body(boundary: str, fields: dict[str, str], file_field: str, filename: str, content_type: str, data: bytes) -> bytes:
    chunks: list[bytes] = []
    for name, value in fields.items():
        chunks.extend(
            [
                f"--{boundary}\r\n".encode(),
                f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode(),
                value.encode(),
                b"\r\n",
            ]
        )
    chunks.extend(
        [
            f"--{boundary}\r\n".encode(),
            f'Content-Disposition: form-data; name="{file_field}"; filename="{filename}"\r\n'.encode(),
            f"Content-Type: {content_type}\r\n\r\n".encode(),
            data,
            b"\r\n",
            f"--{boundary}--\r\n".encode(),
        ]
    )
    return b"".join(chunks)

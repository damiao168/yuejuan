from __future__ import annotations

import math
import os
import socket
from dataclasses import dataclass


@dataclass
class EngineConfig:
    batch_size: int = 1
    lease_seconds: int = 300
    worker_instance_id: str = "image-quality-worker"
    poll_interval: float = 5.0

    def __post_init__(self) -> None:
        # Jobs are processed sequentially and the image-quality API does not
        # expose a source-lease renewal endpoint. Claiming ahead would let later
        # jobs expire while the first image is downloaded and uploaded.
        if self.batch_size != 1:
            raise ValueError("EDUGRADE_IMAGE_QUALITY_BATCH_SIZE must be 1")
        if self.lease_seconds < 30 or self.lease_seconds > 3600:
            raise ValueError("EDUGRADE_IMAGE_QUALITY_LEASE_SECONDS must be between 30 and 3600")
        if not self.worker_instance_id.strip():
            raise ValueError("EDUGRADE_IMAGE_QUALITY_WORKER_ID must not be empty")
        if not math.isfinite(self.poll_interval) or self.poll_interval <= 0:
            raise ValueError("EDUGRADE_IMAGE_QUALITY_POLL_INTERVAL must be greater than 0")


@dataclass
class APIConfig:
    base_url: str
    tenant_code: str
    username: str
    password: str

    def __post_init__(self) -> None:
        if not self.base_url.startswith(("http://", "https://")):
            raise ValueError("EDUGRADE_API_BASE_URL must use http or https")
        for name, value in (
            ("EDUGRADE_IMAGE_QUALITY_TENANT_CODE", self.tenant_code),
            ("EDUGRADE_IMAGE_QUALITY_USERNAME", self.username),
            ("EDUGRADE_IMAGE_QUALITY_PASSWORD", self.password),
        ):
            if not value.strip():
                raise ValueError(f"{name} must not be empty")


def load_engine_config() -> EngineConfig:
    return EngineConfig(
        batch_size=_int_env("EDUGRADE_IMAGE_QUALITY_BATCH_SIZE", 5),
        lease_seconds=_int_env("EDUGRADE_IMAGE_QUALITY_LEASE_SECONDS", 300),
        worker_instance_id=os.getenv("EDUGRADE_IMAGE_QUALITY_WORKER_ID") or f"{socket.gethostname()}-image-quality",
        poll_interval=float(os.getenv("EDUGRADE_IMAGE_QUALITY_POLL_INTERVAL", "5")),
    )


def load_api_config() -> APIConfig:
    return APIConfig(
        base_url=os.getenv("EDUGRADE_API_BASE_URL", "http://localhost:8080").rstrip("/"),
        tenant_code=os.getenv("EDUGRADE_IMAGE_QUALITY_TENANT_CODE", "demo"),
        username=os.getenv("EDUGRADE_IMAGE_QUALITY_USERNAME", ""),
        password=os.getenv("EDUGRADE_IMAGE_QUALITY_PASSWORD", ""),
    )


def _int_env(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or not raw.strip():
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if value <= 0:
        raise ValueError(f"{name} must be greater than 0")
    return value

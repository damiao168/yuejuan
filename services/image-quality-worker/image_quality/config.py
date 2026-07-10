from __future__ import annotations

import os
import socket
from dataclasses import dataclass


@dataclass
class EngineConfig:
    batch_size: int = 5
    lease_seconds: int = 300
    worker_instance_id: str = "image-quality-worker"
    poll_interval: float = 5.0


@dataclass
class APIConfig:
    base_url: str
    tenant_code: str
    username: str
    password: str


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
    try:
        value = int(os.getenv(name, ""))
    except ValueError:
        return default
    return value if value > 0 else default


from __future__ import annotations

import math
import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    api_base_url: str
    tenant_code: str
    username: str
    password: str
    worker_id: str = "subjective-grading-worker"
    poll_interval: float = 5.0
    lease_seconds: int = 300
    heartbeat_interval: float = 10.0
    heartbeat_timeout: float = 3.0
    # The API Gateway permits 750s for governed AI work and 780s for the
    # enclosing HTTP response. The outer worker must wait longer than both.
    execute_timeout: float = 810.0
    health_file: str = "/tmp/edugrade-subjective-grading-worker.ready"
    health_max_age: float = 120.0


def load_settings() -> Settings:
    settings = Settings(
        api_base_url=os.environ["EDUGRADE_API_BASE_URL"].rstrip("/"),
        tenant_code=os.environ["EDUGRADE_SUBJECTIVE_WORKER_TENANT_CODE"],
        username=os.environ["EDUGRADE_SUBJECTIVE_WORKER_USERNAME"],
        password=os.environ["EDUGRADE_SUBJECTIVE_WORKER_PASSWORD"],
        worker_id=os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_ID", "subjective-grading-worker"),
        poll_interval=float(os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_POLL_INTERVAL", "5")),
        lease_seconds=int(os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_LEASE_SECONDS", "300")),
        heartbeat_interval=float(os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_HEARTBEAT_INTERVAL", "10")),
        heartbeat_timeout=float(os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_HEARTBEAT_TIMEOUT", "3")),
        execute_timeout=float(os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_EXECUTE_TIMEOUT", "810")),
        health_file=os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_HEALTH_FILE", "/tmp/edugrade-subjective-grading-worker.ready"),
        health_max_age=float(os.environ.get("EDUGRADE_SUBJECTIVE_WORKER_HEALTH_MAX_AGE", "120")),
    )
    validate_settings(settings)
    return settings


def validate_settings(settings: Settings) -> None:
    if not settings.api_base_url.startswith(("http://", "https://")):
        raise ValueError("EDUGRADE_API_BASE_URL must use http or https")
    for name, value in (("tenant_code", settings.tenant_code), ("username", settings.username), ("password", settings.password), ("worker_id", settings.worker_id)):
        if not value.strip():
            raise ValueError(f"{name} must not be empty")
    if not math.isfinite(settings.poll_interval) or settings.poll_interval <= 0:
        raise ValueError("poll_interval must be greater than 0")
    if settings.lease_seconds < 30 or settings.lease_seconds > 3600:
        raise ValueError("lease_seconds must be between 30 and 3600")
    if not math.isfinite(settings.heartbeat_interval) or settings.heartbeat_interval <= 0 or not math.isfinite(settings.heartbeat_timeout) or settings.heartbeat_timeout <= 0:
        raise ValueError("heartbeat values must be greater than 0")
    if settings.heartbeat_interval + settings.heartbeat_timeout >= settings.lease_seconds:
        raise ValueError("heartbeat interval plus timeout must be shorter than the lease")
    if not math.isfinite(settings.execute_timeout) or settings.execute_timeout <= 0:
        raise ValueError("execute_timeout must be greater than 0")
    if not settings.health_file.strip():
        raise ValueError("health_file must not be empty")
    if not math.isfinite(settings.health_max_age) or settings.health_max_age <= 0:
        raise ValueError("health_max_age must be greater than 0")

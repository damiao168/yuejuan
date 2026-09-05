from __future__ import annotations

import math
import os
import socket
from dataclasses import dataclass


@dataclass(frozen=True)
class Config:
    base_url: str
    tenant_code: str
    username: str
    password: str
    worker_id: str
    batch_size: int = 1
    lease_seconds: int = 600
    poll_interval: float = 4.0
    max_pages: int = 500
    max_page_pixels: int = 50_000_000
    max_total_pixels: int = 1_000_000_000
    heartbeat_interval: float = 30.0
    heartbeat_timeout: float = 10.0

    def __post_init__(self) -> None:
        if not self.base_url.startswith(("http://", "https://")):
            raise ValueError("base_url must use http or https")
        if not self.tenant_code.strip() or not self.worker_id.strip():
            raise ValueError("tenant_code and worker_id must not be empty")
        # A heartbeat starts only when a task is processed. Claiming multiple
        # tasks up front can therefore expire later leases during a long decode.
        if self.batch_size != 1:
            raise ValueError("batch_size must be 1 for the sequential page-processing worker")
        # Match the runtime's own bounds (internal/workerruntime/validation.go):
        # it silently clamps out-of-range leases to 300s, so validating the
        # heartbeat cadence against an unclamped value would compare against a
        # lease the server never grants.
        if self.lease_seconds < 30 or self.lease_seconds > 3600:
            raise ValueError("lease_seconds must be between 30 and 3600")
        if not math.isfinite(self.poll_interval) or self.poll_interval <= 0:
            raise ValueError("poll_interval must be greater than zero")
        for name, value in (
            ("max_pages", self.max_pages),
            ("max_page_pixels", self.max_page_pixels),
            ("max_total_pixels", self.max_total_pixels),
        ):
            if not isinstance(value, int) or isinstance(value, bool) or value <= 0:
                raise ValueError(f"{name} must be a positive integer")
        if (
            not math.isfinite(self.heartbeat_interval)
            or not math.isfinite(self.heartbeat_timeout)
            or self.heartbeat_interval <= 0
            or self.heartbeat_timeout <= 0
        ):
            raise ValueError("heartbeat interval and timeout must be greater than zero")
        if self.heartbeat_interval + self.heartbeat_timeout >= self.lease_seconds:
            raise ValueError("heartbeat interval plus timeout must be shorter than lease_seconds")


def load_config() -> Config:
    return Config(
        base_url=os.getenv("EDUGRADE_API_BASE_URL", "http://localhost:8080").rstrip("/"),
        tenant_code=os.getenv("EDUGRADE_PAGE_PROCESSING_TENANT_CODE", "demo"),
        username=os.getenv("EDUGRADE_PAGE_PROCESSING_USERNAME", ""),
        password=os.getenv("EDUGRADE_PAGE_PROCESSING_PASSWORD", ""),
        worker_id=os.getenv("EDUGRADE_PAGE_PROCESSING_WORKER_ID") or f"{socket.gethostname()}-page-processing",
        batch_size=_positive_int("EDUGRADE_PAGE_PROCESSING_BATCH_SIZE", 1),
        lease_seconds=_positive_int("EDUGRADE_PAGE_PROCESSING_LEASE_SECONDS", 600),
        poll_interval=float(os.getenv("EDUGRADE_PAGE_PROCESSING_POLL_INTERVAL", "4")),
        max_pages=_positive_int("EDUGRADE_PAGE_PROCESSING_MAX_PAGES", 500),
        max_page_pixels=_positive_int("EDUGRADE_PAGE_PROCESSING_MAX_PAGE_PIXELS", 50_000_000),
        max_total_pixels=_positive_int("EDUGRADE_PAGE_PROCESSING_MAX_TOTAL_PIXELS", 1_000_000_000),
        heartbeat_interval=_positive_float("EDUGRADE_PAGE_PROCESSING_HEARTBEAT_INTERVAL", 30.0),
        heartbeat_timeout=_positive_float("EDUGRADE_PAGE_PROCESSING_HEARTBEAT_TIMEOUT", 10.0),
    )


def _positive_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or not raw.strip():
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if value <= 0:
        raise ValueError(f"{name} must be greater than zero")
    return value


def _positive_float(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or not raw.strip():
        return default
    try:
        value = float(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number") from exc
    if not math.isfinite(value) or value <= 0:
        raise ValueError(f"{name} must be a finite number greater than zero")
    return value

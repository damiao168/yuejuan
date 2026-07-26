from __future__ import annotations

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
    batch_size: int = 2
    lease_seconds: int = 600
    poll_interval: float = 4.0
    max_pages: int = 500
    max_page_pixels: int = 50_000_000
    max_total_pixels: int = 1_000_000_000
    heartbeat_interval: float = 30.0
    heartbeat_timeout: float = 10.0

    def __post_init__(self) -> None:
        # Match the runtime's own bounds (internal/workerruntime/validation.go):
        # it silently clamps out-of-range leases to 300s, so validating the
        # heartbeat cadence against an unclamped value would compare against a
        # lease the server never grants.
        if self.lease_seconds < 30 or self.lease_seconds > 3600:
            raise ValueError("lease_seconds must be between 30 and 3600")
        if self.heartbeat_interval <= 0 or self.heartbeat_timeout <= 0:
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
        batch_size=_positive_int("EDUGRADE_PAGE_PROCESSING_BATCH_SIZE", 2),
        lease_seconds=_positive_int("EDUGRADE_PAGE_PROCESSING_LEASE_SECONDS", 600),
        poll_interval=float(os.getenv("EDUGRADE_PAGE_PROCESSING_POLL_INTERVAL", "4")),
        max_pages=_positive_int("EDUGRADE_PAGE_PROCESSING_MAX_PAGES", 500),
        max_page_pixels=_positive_int("EDUGRADE_PAGE_PROCESSING_MAX_PAGE_PIXELS", 50_000_000),
        max_total_pixels=_positive_int("EDUGRADE_PAGE_PROCESSING_MAX_TOTAL_PIXELS", 1_000_000_000),
        heartbeat_interval=_positive_float("EDUGRADE_PAGE_PROCESSING_HEARTBEAT_INTERVAL", 30.0),
        heartbeat_timeout=_positive_float("EDUGRADE_PAGE_PROCESSING_HEARTBEAT_TIMEOUT", 10.0),
    )


def _positive_int(name: str, default: int) -> int:
    try:
        value = int(os.getenv(name, ""))
    except ValueError:
        return default
    return value if value > 0 else default


def _positive_float(name: str, default: float) -> float:
    try:
        value = float(os.getenv(name, ""))
    except ValueError:
        return default
    return value if value > 0 else default

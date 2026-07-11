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
    )


def _positive_int(name: str, default: int) -> int:
    try:
        value = int(os.getenv(name, ""))
    except ValueError:
        return default
    return value if value > 0 else default

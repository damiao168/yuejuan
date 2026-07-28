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
    engine: str = "paddleocr"
    engine_version: str = "pp-ocrv5"
    model_version: str = "ppocr-v5-server"
    min_confidence: float = 0.8
    poll_interval: float = 5.0
    batch_size: int = 1
    device: str = "cpu"
    preprocess_profile: str = "default"
    worker_id: str = "ocr-worker"
    lease_seconds: int = 300
    heartbeat_interval: float = 10.0
    heartbeat_timeout: float = 3.0


def load_settings() -> Settings:
    settings = Settings(
        api_base_url=os.environ["EDUGRADE_API_BASE_URL"].rstrip("/"),
        tenant_code=os.environ["EDUGRADE_OCR_WORKER_TENANT_CODE"],
        username=os.environ["EDUGRADE_OCR_WORKER_USERNAME"],
        password=os.environ["EDUGRADE_OCR_WORKER_PASSWORD"],
        engine=os.environ.get("EDUGRADE_OCR_ENGINE", "paddleocr"),
        engine_version=os.environ.get("EDUGRADE_OCR_ENGINE_VERSION", "pp-ocrv5"),
        model_version=os.environ.get("EDUGRADE_OCR_MODEL_VERSION", "ppocr-v5-server"),
        min_confidence=float(os.environ.get("EDUGRADE_OCR_MIN_CONFIDENCE", "0.8")),
        poll_interval=float(os.environ.get("EDUGRADE_OCR_POLL_INTERVAL", "5")),
        batch_size=int(os.environ.get("EDUGRADE_OCR_BATCH_SIZE", "1")),
        device=os.environ.get("EDUGRADE_OCR_DEVICE", "cpu"),
        preprocess_profile=os.environ.get("EDUGRADE_OCR_PREPROCESS_PROFILE", "default"),
        worker_id=os.environ.get("EDUGRADE_OCR_WORKER_ID", "ocr-worker"),
        lease_seconds=int(os.environ.get("EDUGRADE_OCR_LEASE_SECONDS", "300")),
        heartbeat_interval=float(os.environ.get("EDUGRADE_OCR_HEARTBEAT_INTERVAL", "10")),
        heartbeat_timeout=float(os.environ.get("EDUGRADE_OCR_HEARTBEAT_TIMEOUT", "3")),
    )
    _validate_settings(settings)
    return settings


def _validate_settings(settings: Settings) -> None:
    if not settings.api_base_url.startswith(("http://", "https://")):
        raise ValueError("EDUGRADE_API_BASE_URL must use http or https")
    for name, value in (
        ("EDUGRADE_OCR_WORKER_TENANT_CODE", settings.tenant_code),
        ("EDUGRADE_OCR_WORKER_USERNAME", settings.username),
        ("EDUGRADE_OCR_WORKER_PASSWORD", settings.password),
        ("EDUGRADE_OCR_WORKER_ID", settings.worker_id),
        ("EDUGRADE_OCR_ENGINE", settings.engine),
        ("EDUGRADE_OCR_ENGINE_VERSION", settings.engine_version),
        ("EDUGRADE_OCR_MODEL_VERSION", settings.model_version),
        ("EDUGRADE_OCR_PREPROCESS_PROFILE", settings.preprocess_profile),
    ):
        if not value.strip():
            raise ValueError(f"{name} must not be empty")
    if not math.isfinite(settings.min_confidence) or not 0 <= settings.min_confidence <= 1:
        raise ValueError("EDUGRADE_OCR_MIN_CONFIDENCE must be between 0 and 1")
    if not math.isfinite(settings.poll_interval) or settings.poll_interval <= 0:
        raise ValueError("EDUGRADE_OCR_POLL_INTERVAL must be greater than 0")
    # OCR processing is sequential. Claiming more than one task would leave
    # later tasks without a heartbeat while the first model inference runs.
    if settings.batch_size != 1:
        raise ValueError("EDUGRADE_OCR_BATCH_SIZE must be 1 for the sequential OCR worker")
    if settings.lease_seconds < 30 or settings.lease_seconds > 3600:
        raise ValueError("EDUGRADE_OCR_LEASE_SECONDS must be between 30 and 3600")
    if not math.isfinite(settings.heartbeat_interval) or settings.heartbeat_interval <= 0:
        raise ValueError("EDUGRADE_OCR_HEARTBEAT_INTERVAL must be greater than 0")
    if not math.isfinite(settings.heartbeat_timeout) or settings.heartbeat_timeout <= 0:
        raise ValueError("EDUGRADE_OCR_HEARTBEAT_TIMEOUT must be greater than 0")
    if settings.heartbeat_interval >= settings.lease_seconds:
        raise ValueError("EDUGRADE_OCR_HEARTBEAT_INTERVAL must be shorter than the lease")
    if settings.heartbeat_interval + settings.heartbeat_timeout >= settings.lease_seconds:
        raise ValueError("OCR heartbeat interval plus timeout must be shorter than the lease")

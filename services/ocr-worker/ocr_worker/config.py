from __future__ import annotations

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


def load_settings() -> Settings:
    return Settings(
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
    )

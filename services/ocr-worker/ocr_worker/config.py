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
    model_version: str = "ppocr-v5-mobile"
    min_confidence: float = 0.8
    poll_interval: float = 5.0
    batch_size: int = 1
    device: str = "cpu"
    preprocess_profile: str = "default"
    worker_id: str = "ocr-worker"
    lease_seconds: int = 300
    heartbeat_interval: float = 10.0
    heartbeat_timeout: float = 3.0
    ocr_runtime_enabled: bool = True
    math_runtime_enabled: bool = False
    math_verify_base_url: str = "http://math-verification-worker:8092"
    math_verify_token: str = ""
    formula_model_version: str = "PP-FormulaNet_plus-M"
    cpu_threads: int = 4
    enable_mkldnn: str = "auto"
    enable_hpi: bool = False
    use_textline_orientation: bool = True
    text_det_limit_type: str = "min"
    text_det_limit_side_len: int = 64
    text_recognition_batch_size: int = 1


def load_settings() -> Settings:
    settings = Settings(
        api_base_url=os.environ["EDUGRADE_API_BASE_URL"].rstrip("/"),
        tenant_code=os.environ["EDUGRADE_OCR_WORKER_TENANT_CODE"],
        username=os.environ["EDUGRADE_OCR_WORKER_USERNAME"],
        password=os.environ["EDUGRADE_OCR_WORKER_PASSWORD"],
        engine=os.environ.get("EDUGRADE_OCR_ENGINE", "paddleocr"),
        engine_version=os.environ.get("EDUGRADE_OCR_ENGINE_VERSION", "pp-ocrv5"),
        model_version=os.environ.get("EDUGRADE_OCR_MODEL_VERSION", "ppocr-v5-mobile"),
        min_confidence=float(os.environ.get("EDUGRADE_OCR_MIN_CONFIDENCE", "0.8")),
        poll_interval=float(os.environ.get("EDUGRADE_OCR_POLL_INTERVAL", "5")),
        batch_size=int(os.environ.get("EDUGRADE_OCR_BATCH_SIZE", "1")),
        device=os.environ.get("EDUGRADE_OCR_DEVICE", "cpu"),
        preprocess_profile=os.environ.get("EDUGRADE_OCR_PREPROCESS_PROFILE", "default"),
        worker_id=os.environ.get("EDUGRADE_OCR_WORKER_ID", "ocr-worker"),
        lease_seconds=int(os.environ.get("EDUGRADE_OCR_LEASE_SECONDS", "300")),
        heartbeat_interval=float(os.environ.get("EDUGRADE_OCR_HEARTBEAT_INTERVAL", "10")),
        heartbeat_timeout=float(os.environ.get("EDUGRADE_OCR_HEARTBEAT_TIMEOUT", "3")),
        ocr_runtime_enabled=os.environ.get("EDUGRADE_OCR_RUNTIME_ENABLED", "true").lower() in {"1", "true", "yes"},
        math_runtime_enabled=os.environ.get("EDUGRADE_MATH_RUNTIME_ENABLED", "false").lower() in {"1", "true", "yes"},
        math_verify_base_url=os.environ.get("EDUGRADE_MATH_VERIFY_BASE_URL", "http://math-verification-worker:8092"),
        math_verify_token=os.environ.get("EDUGRADE_MATH_VERIFY_TOKEN", ""),
        formula_model_version=os.environ.get("EDUGRADE_FORMULA_MODEL_VERSION", "PP-FormulaNet_plus-M"),
        cpu_threads=int(os.environ.get("EDUGRADE_OCR_CPU_THREADS", "4")),
        enable_mkldnn=os.environ.get("EDUGRADE_OCR_ENABLE_MKLDNN", "auto").strip().lower(),
        enable_hpi=os.environ.get("EDUGRADE_OCR_ENABLE_HPI", "false").lower() in {"1", "true", "yes"},
        use_textline_orientation=os.environ.get("EDUGRADE_OCR_USE_TEXTLINE_ORIENTATION", "true").lower() in {"1", "true", "yes"},
        text_det_limit_type=os.environ.get("EDUGRADE_OCR_TEXT_DET_LIMIT_TYPE", "min").strip().lower(),
        text_det_limit_side_len=int(os.environ.get("EDUGRADE_OCR_TEXT_DET_LIMIT_SIDE_LEN", "64")),
        text_recognition_batch_size=int(os.environ.get("EDUGRADE_OCR_TEXT_RECOGNITION_BATCH_SIZE", "1")),
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
    if settings.cpu_threads < 1 or settings.cpu_threads > 64:
        raise ValueError("EDUGRADE_OCR_CPU_THREADS must be between 1 and 64")
    if settings.enable_mkldnn not in {"auto", "true", "false"}:
        raise ValueError("EDUGRADE_OCR_ENABLE_MKLDNN must be auto, true, or false")
    if settings.text_det_limit_type not in {"max", "min"}:
        raise ValueError("EDUGRADE_OCR_TEXT_DET_LIMIT_TYPE must be max or min")
    if settings.text_det_limit_side_len < 32 or settings.text_det_limit_side_len > 4096:
        raise ValueError("EDUGRADE_OCR_TEXT_DET_LIMIT_SIDE_LEN must be between 32 and 4096")
    if settings.text_recognition_batch_size < 1 or settings.text_recognition_batch_size > 64:
        raise ValueError("EDUGRADE_OCR_TEXT_RECOGNITION_BATCH_SIZE must be between 1 and 64")
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
    if not settings.ocr_runtime_enabled and not settings.math_runtime_enabled:
        raise ValueError("at least one OCR worker runtime must be enabled")
    if settings.math_runtime_enabled:
        if not settings.math_verify_base_url.startswith(("http://", "https://")):
            raise ValueError("EDUGRADE_MATH_VERIFY_BASE_URL must use http or https")
        if len(settings.math_verify_token) < 32:
            raise ValueError("EDUGRADE_MATH_VERIFY_TOKEN must contain at least 32 characters")
        if settings.formula_model_version not in {"PP-FormulaNet_plus-M", "PP-FormulaNet_plus-L"}:
            raise ValueError("EDUGRADE_FORMULA_MODEL_VERSION is unsupported")

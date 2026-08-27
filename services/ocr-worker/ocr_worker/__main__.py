from __future__ import annotations

import logging
import time

from .api import APIError, AuthenticationError, EduGradeClient
from .config import load_settings
from .engine import PaddleOCREngine
from .math_runner import MathUnderstandingRunner, MathVerificationClient
from .recognition_router import PaddleFormulaNetEngine, RecognitionRouter
from .runner import OCRRunner, WorkerConfig


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    log = logging.getLogger("edugrade.ocr_worker")
    settings = load_settings()
    engine = PaddleOCREngine(device=settings.device, model_version=settings.model_version)
    log.info("initializing OCR engine before worker registration")
    engine.initialize()
    log.info("OCR engine ready: model=%s device=%s", engine.model_version, engine.device)
    client = EduGradeClient(
        base_url=settings.api_base_url,
        tenant_code=settings.tenant_code,
        username=settings.username,
        password=settings.password,
    )
    worker_config = WorkerConfig(
        worker_id=settings.worker_id,
        batch_size=settings.batch_size,
        engine=settings.engine,
        engine_version=settings.engine_version,
        preprocess_profile=settings.preprocess_profile,
        lease_seconds=settings.lease_seconds,
        heartbeat_interval=settings.heartbeat_interval,
        heartbeat_timeout=settings.heartbeat_timeout,
    )
    runner = None
    if getattr(settings, "ocr_runtime_enabled", True):
        runner = OCRRunner(api=client, engine=engine, config=worker_config)
    math_runner = None
    if getattr(settings, "math_runtime_enabled", False):
        formula_engine = None
        try:
            formula_engine = PaddleFormulaNetEngine(model_version=settings.formula_model_version, device=settings.device)
            formula_engine.initialize()
        except Exception:
            formula_engine = None
            log.exception("formula engine unavailable; mathematical formula regions will require human review")
        math_runner = MathUnderstandingRunner(
            api=client,
            router=RecognitionRouter(text_engine=engine, formula_engine=formula_engine),
            verifier=MathVerificationClient(settings.math_verify_base_url, settings.math_verify_token),
            config=worker_config,
        )
        log.info("math understanding runtime ready: formula_model=%s available=%s", settings.formula_model_version, formula_engine is not None)
    while True:
        try:
            if client.token is None:
                client.login()
                log.info("worker authenticated")
            processed = runner.process_once() if runner is not None else 0
            if math_runner is not None:
                processed += math_runner.process_once()
            if processed == 0:
                time.sleep(settings.poll_interval)
        except AuthenticationError:
            client.token = None
            log.warning("worker authentication expired; login will be retried")
            time.sleep(settings.poll_interval)
        except APIError as exc:
            log.warning("worker API request failed; polling will continue", extra={"error_type": type(exc).__name__})
            time.sleep(settings.poll_interval)
        except Exception:
            log.exception("unexpected OCR worker error; polling will continue")
            time.sleep(settings.poll_interval)


if __name__ == "__main__":
    main()

from __future__ import annotations

import logging
import time

from .api import APIError, AuthenticationError, EduGradeClient
from .config import load_settings
from .engine import PaddleOCREngine
from .runner import OCRRunner, WorkerConfig


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    log = logging.getLogger("edugrade.ocr_worker")
    settings = load_settings()
    client = EduGradeClient(
        base_url=settings.api_base_url,
        tenant_code=settings.tenant_code,
        username=settings.username,
        password=settings.password,
    )
    engine = PaddleOCREngine(device=settings.device, model_version=settings.model_version)
    runner = OCRRunner(
        api=client,
        engine=engine,
        config=WorkerConfig(
            worker_id=settings.worker_id,
            batch_size=settings.batch_size,
            model_version=settings.model_version,
            preprocess_profile=settings.preprocess_profile,
            lease_seconds=settings.lease_seconds,
        ),
    )
    while True:
        try:
            if client.token is None:
                client.login()
                log.info("worker authenticated")
            processed = runner.process_once()
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

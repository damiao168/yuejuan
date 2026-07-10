from __future__ import annotations

import logging
import time

from image_quality.api import APIError, AuthenticationError, EduGradeImageQualityClient
from image_quality.config import load_api_config, load_engine_config
from image_quality.runner import ImageQualityRunner


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    log = logging.getLogger("edugrade.image_quality_worker")
    api_config = load_api_config()
    engine_config = load_engine_config()
    client = EduGradeImageQualityClient(
        base_url=api_config.base_url,
        tenant_code=api_config.tenant_code,
        username=api_config.username,
        password=api_config.password,
    )
    runner = ImageQualityRunner(client, engine_config)
    while True:
        try:
            if client.token is None:
                client.login()
                log.info("worker authenticated")
            processed = runner.run_once()
            if processed == 0:
                time.sleep(engine_config.poll_interval)
        except AuthenticationError:
            client.token = None
            log.warning("worker authentication expired; login will be retried")
            time.sleep(engine_config.poll_interval)
        except APIError as exc:
            log.warning("worker API request failed; polling will continue", extra={"error_type": type(exc).__name__})
            time.sleep(engine_config.poll_interval)
        except Exception:
            log.exception("unexpected image-quality worker error; polling will continue")
            time.sleep(engine_config.poll_interval)


if __name__ == "__main__":
    main()

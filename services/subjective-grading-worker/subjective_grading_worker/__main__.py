from __future__ import annotations

import time

from .api import EduGradeClient
from .config import load_settings
from .runner import Runner


def main() -> None:
    settings = load_settings()
    api = EduGradeClient(settings.api_base_url, settings.tenant_code, settings.username, settings.password)
    api.login()
    runner = Runner(api=api, settings=settings)
    while True:
        runner.process_once()
        time.sleep(settings.poll_interval)


if __name__ == "__main__":
    main()

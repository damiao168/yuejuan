from __future__ import annotations

import argparse
import json
from collections.abc import Sequence

from page_processing.api import Client
from page_processing.config import load_config
from page_processing.runner import Runner


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run the EduGrade page-processing worker.")
    parser.add_argument(
        "--once",
        action="store_true",
        help="claim and process at most one task batch, print its claimed-task count, then exit",
    )
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    config = load_config()
    if not config.username or not config.password:
        raise SystemExit("page-processing worker credentials are required")
    client = Client(config.base_url, config.tenant_code, config.username, config.password)
    client.login()
    runner = Runner(client, config)
    if args.once:
        claimed_tasks = runner.run_once()
        print(json.dumps({"claimed_tasks": claimed_tasks, "mode": "once"}, sort_keys=True))
        return 0
    runner.run_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

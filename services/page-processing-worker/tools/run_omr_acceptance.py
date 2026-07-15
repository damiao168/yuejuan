from __future__ import annotations

import argparse
from pathlib import Path

from page_processing.omr_acceptance import write_synthetic_acceptance_report


def main() -> int:
    parser = argparse.ArgumentParser(description="Run the STORY-056 synthetic OMR acceptance suite.")
    parser.add_argument("--output", required=True, help="Directory for summary.json and summary.md")
    args = parser.parse_args()
    report = write_synthetic_acceptance_report(Path(args.output))
    print(f"wrote {report['total_submissions']} synthetic OMR cases to {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

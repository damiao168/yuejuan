from __future__ import annotations

import pytest
from page_processing.config import Config


def config(**overrides: object) -> Config:
    values = {
        "base_url": "http://api",
        "tenant_code": "demo",
        "username": "worker",
        "password": "secret",
        "worker_id": "worker-1",
    }
    values.update(overrides)
    return Config(**values)


def test_sequential_worker_rejects_claiming_multiple_tasks() -> None:
    with pytest.raises(ValueError, match="batch_size must be 1"):
        config(batch_size=2)


@pytest.mark.parametrize("poll_interval", [0, -1, float("nan")])
def test_poll_interval_must_be_finite_and_positive(poll_interval: float) -> None:
    with pytest.raises(ValueError, match="poll_interval"):
        config(poll_interval=poll_interval)

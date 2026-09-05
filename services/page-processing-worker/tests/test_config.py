from __future__ import annotations

import pytest
from page_processing.config import Config, load_config


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


def test_minimal_environment_uses_valid_sequential_batch_size(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("EDUGRADE_PAGE_PROCESSING_BATCH_SIZE", raising=False)

    loaded = load_config()

    assert loaded.batch_size == 1


@pytest.mark.parametrize("poll_interval", [0, -1, float("nan")])
def test_poll_interval_must_be_finite_and_positive(poll_interval: float) -> None:
    with pytest.raises(ValueError, match="poll_interval"):
        config(poll_interval=poll_interval)

from __future__ import annotations

import pytest
from image_quality.config import APIConfig, EngineConfig


def test_sequential_worker_rejects_claiming_multiple_jobs() -> None:
    with pytest.raises(ValueError, match="must be 1"):
        EngineConfig(batch_size=2)


@pytest.mark.parametrize("poll_interval", [0, -1, float("nan")])
def test_poll_interval_must_be_finite_and_positive(poll_interval: float) -> None:
    with pytest.raises(ValueError, match="POLL_INTERVAL"):
        EngineConfig(poll_interval=poll_interval)


def test_api_credentials_are_required_at_startup() -> None:
    with pytest.raises(ValueError, match="USERNAME"):
        APIConfig("http://api", "demo", "", "secret")

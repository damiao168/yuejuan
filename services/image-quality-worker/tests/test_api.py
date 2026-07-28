from __future__ import annotations

import pytest
from image_quality.api import APIError, EduGradeImageQualityClient


def test_download_rejects_cross_origin_url_before_sending_worker_token() -> None:
    client = EduGradeImageQualityClient("http://api-gateway:8080", "demo", "worker", "secret", token="worker-token")

    with pytest.raises(APIError, match="configured API origin"):
        client.download("https://attacker.invalid/collect")

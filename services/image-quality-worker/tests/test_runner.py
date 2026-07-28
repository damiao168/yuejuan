from __future__ import annotations

import io
import json

from image_quality.config import EngineConfig
from image_quality.runner import ImageQualityRunner
from PIL import Image, ImageDraw


def test_runner_claims_downloads_processes_uploads_and_submits() -> None:
    client = FakeClient(clear_png())
    runner = ImageQualityRunner(client, EngineConfig(batch_size=1, lease_seconds=300, worker_instance_id="worker-a"))

    processed = runner.run_once()

    assert processed == 1
    assert client.upload_requested
    assert client.result_submitted is not None
    assert client.result_submitted["quality_status"] == "passed"
    assert client.result_submitted["normalized_file_asset_id"] == "file-normalized-1"
    assert "source_to_normalized_matrix" in client.result_submitted["normalization_transform"]
    assert "student_id" not in json.dumps(client.result_submitted)
    assert "candidate_no" not in json.dumps(client.result_submitted)


def test_runner_reports_terminal_failure_for_unreadable_image() -> None:
    client = FakeClient(b"not-an-image")
    runner = ImageQualityRunner(client, EngineConfig(batch_size=1, lease_seconds=300, worker_instance_id="worker-a"))

    processed = runner.run_once()

    assert processed == 1
    assert client.failure_submitted is not None
    assert client.failure_submitted["processing_status"] == "terminal_error"
    assert client.failure_submitted["error_code"] == "image_quality_processing_failed"


class FakeClient:
    def __init__(self, image_bytes: bytes) -> None:
        self.image_bytes = image_bytes
        self.upload_requested = False
        self.result_submitted: dict | None = None
        self.failure_submitted: dict | None = None

    def claim_jobs(self, worker_instance_id: str, limit: int, lease_seconds: int) -> list[dict]:
        assert worker_instance_id == "worker-a"
        assert limit == 1
        assert lease_seconds == 300
        return [
            {
                "run_id": "quality-run-1",
                "runtime_task_id": "runtime-task-1",
                "submission_id": "submission-1",
                "exam_id": "exam-1",
                "submission_page_id": "page-1",
                "source_file_asset_id": "file-original-1",
                "source_sha256": "source-hash-1",
                "download_url": "/api/v1/files/file-original-1/download",
                "lease_token": "lease-token-1",
                "attempt_no": 1,
                "profile": {
                    "name": "opencv-default",
                    "version": "v1",
                    "config_hash": "sha256:opencv-default-v1",
                },
            }
        ]

    def download(self, url: str) -> bytes:
        assert url == "/api/v1/files/file-original-1/download"
        return self.image_bytes

    def request_normalized_asset(self, run_id: str, payload: dict) -> dict:
        assert run_id == "quality-run-1"
        assert payload["lease_token"] == "lease-token-1"
        assert payload["content_type"] == "image/png"
        assert payload["sha256"]
        return {"upload_url": "/api/v1/files", "upload_method": "POST", "expected_sha256": payload["sha256"]}

    def upload_normalized(self, slot: dict, job: dict, normalized_png: bytes, sha256: str) -> str:
        assert slot["expected_sha256"] == sha256
        assert normalized_png.startswith(b"\x89PNG")
        self.upload_requested = True
        return "file-normalized-1"

    def submit_result(self, run_id: str, payload: dict) -> None:
        assert run_id == "quality-run-1"
        self.result_submitted = payload

    def submit_failure(self, run_id: str, payload: dict) -> None:
        assert run_id == "quality-run-1"
        self.failure_submitted = payload


def clear_png() -> bytes:
    image = Image.new("RGB", (600, 800), "white")
    draw = ImageDraw.Draw(image)
    draw.rectangle((30, 30, 570, 770), outline="black", width=5)
    for y in range(100, 700, 70):
        draw.line((80, y, 520, y), fill="black", width=4)
    output = io.BytesIO()
    image.save(output, format="PNG")
    return output.getvalue()

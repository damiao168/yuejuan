from __future__ import annotations

import hashlib
import math
import time
from dataclasses import dataclass
from typing import Any

from .api import APIError, AuthenticationError
from .engine import OCREngine


@dataclass(frozen=True)
class WorkerConfig:
    worker_id: str
    batch_size: int = 1
    model_version: str = "ppocr-v5-server"
    config_hash: str = ""
    preprocess_profile: str = "default"
    lease_seconds: int = 300


class OCRRunner:
    def __init__(self, *, api: Any, engine: OCREngine, config: WorkerConfig) -> None:
        self.api = api
        self.engine = engine
        self.config = config

    def process_once(self) -> int:
        tasks = self.api.claim_tasks(self.config.worker_id, self.config.batch_size, self.config.lease_seconds)
        processed = 0
        for task in tasks:
            self._process_task(task)
            processed += 1
        return processed

    def _process_task(self, runtime_task: dict[str, Any]) -> None:
        start = time.monotonic()
        runtime_task_id = str(runtime_task["id"])
        lease_token = str(runtime_task["lease_token"])
        task_id = str(runtime_task["source_id"])
        self.api.heartbeat_task(runtime_task_id, lease_token, self.config.worker_id)
        self.api.start_task(task_id)
        task_input = self.api.get_task_input(task_id)
        image_hash = hashlib.sha256()
        results: list[dict[str, Any]] = []
        for page in task_input.get("pages", []):
            page_id = str(page["id"])
            try:
                image_bytes = self.api.download(str(page["download_url"]))
            except AuthenticationError:
                raise
            except APIError:
                self.api.fail_task(task_id, "download_failed", runtime_task_id, lease_token, True)
                return
            image_hash.update(image_bytes)
            try:
                blocks = self.engine.recognize(image_bytes)
            except Exception:
                self.api.fail_task(task_id, "ocr_engine_failed", runtime_task_id, lease_token, True)
                return
            for block in blocks:
                if not _valid_block(block.text, block.bbox, block.confidence):
                    continue
                results.append(
                    {
                        "submission_page_id": page_id,
                        "text": block.text,
                        "bbox": block.bbox,
                        "confidence": block.confidence,
                    }
                )
        if not results:
            self.api.fail_task(task_id, "empty_ocr_result", runtime_task_id, lease_token, False)
            return
        duration_ms = int((time.monotonic() - start) * 1000)
        payload = {
            "worker_id": self.config.worker_id,
            "model_version": self.config.model_version,
            "config_hash": self._config_hash(),
            "input_hash": image_hash.hexdigest(),
            "duration_ms": duration_ms,
            "preprocess_profile": self.config.preprocess_profile,
            "runtime_task_id": runtime_task_id,
            "runtime_lease_token": lease_token,
            "results": results,
        }
        self.api.complete_task(task_id, payload)

    def _config_hash(self) -> str:
        if self.config.config_hash:
            return self.config.config_hash
        raw = f"{self.config.model_version}|{self.config.preprocess_profile}".encode("utf-8")
        return hashlib.sha256(raw).hexdigest()


def _valid_block(text: str, bbox: list[float], confidence: float) -> bool:
    if not text.strip() or len(bbox) != 4 or not math.isfinite(confidence) or confidence < 0 or confidence > 1:
        return False
    if not all(math.isfinite(value) for value in bbox):
        return False
    return bbox[0] >= 0 and bbox[1] >= 0 and bbox[2] > 0 and bbox[3] > 0

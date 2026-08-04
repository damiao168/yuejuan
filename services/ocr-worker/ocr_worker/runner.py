from __future__ import annotations

import hashlib
import math
import threading
import time
from dataclasses import dataclass
from typing import Any, Self

from .api import APIError, AuthenticationError
from .engine import OCREngine


@dataclass(frozen=True)
class WorkerConfig:
    worker_id: str
    batch_size: int = 1
    engine: str = "paddleocr"
    engine_version: str = "pp-ocrv5"
    config_hash: str = ""
    preprocess_profile: str = "default"
    lease_seconds: int = 300
    heartbeat_interval: float = 10.0
    heartbeat_timeout: float = 3.0

    def __post_init__(self) -> None:
        if self.batch_size != 1:
            raise ValueError("the sequential OCR worker must claim exactly one task")
        if not self.engine or not self.engine_version:
            raise ValueError("engine and engine_version must be configured")
        if self.lease_seconds < 30 or self.lease_seconds > 3600:
            raise ValueError("lease_seconds must be between 30 and 3600")
        if (
            not math.isfinite(self.heartbeat_interval)
            or not math.isfinite(self.heartbeat_timeout)
            or self.heartbeat_interval <= 0
            or self.heartbeat_timeout <= 0
        ):
            raise ValueError("heartbeat interval and timeout must be greater than zero")
        if self.heartbeat_interval >= self.lease_seconds:
            raise ValueError("heartbeat_interval must be shorter than lease_seconds")
        if self.heartbeat_interval + self.heartbeat_timeout >= self.lease_seconds:
            raise ValueError("heartbeat interval plus timeout must be shorter than the lease")


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
        tenant_id = str(runtime_task["tenant_id"])
        activate_task = getattr(self.api, "activate_task", None)
        if callable(activate_task):
            activate_task(runtime_task, self.config.worker_id)
        incompatibility = self._runtime_incompatibility(runtime_task)
        if incompatibility is not None:
            self.api.fail_task(task_id, incompatibility, runtime_task_id, lease_token, False, tenant_id)
            return
        with _LeaseHeartbeat(
            api=self.api,
            runtime_task_id=runtime_task_id,
            lease_token=lease_token,
            worker_id=self.config.worker_id,
            interval=self.config.heartbeat_interval,
            lease_seconds=self.config.lease_seconds,
            request_timeout=self.config.heartbeat_timeout,
            tenant_id=tenant_id,
        ) as heartbeat:
            self.api.start_task(task_id, tenant_id)
            task_input = self.api.get_task_input(task_id, tenant_id)
            image_hash = hashlib.sha256()
            results: list[dict[str, Any]] = []
            for page in task_input.get("pages", []):
                heartbeat.raise_if_failed()
                page_id = str(page["id"])
                try:
                    image_bytes = self.api.download(str(page["download_url"]), tenant_id)
                except AuthenticationError:
                    raise
                except APIError:
                    heartbeat.stop()
                    self.api.fail_task(task_id, "download_failed", runtime_task_id, lease_token, True, tenant_id)
                    return
                image_hash.update(image_bytes)
                try:
                    blocks = self.engine.recognize(image_bytes)
                except Exception:  # noqa: BLE001 - Paddle backends raise heterogeneous runtime exceptions.
                    heartbeat.stop()
                    self.api.fail_task(task_id, "ocr_engine_failed", runtime_task_id, lease_token, True, tenant_id)
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
            heartbeat.raise_if_failed()
            if not results:
                heartbeat.stop()
                self.api.fail_task(task_id, "empty_ocr_result", runtime_task_id, lease_token, False, tenant_id)
                return
            duration_ms = int((time.monotonic() - start) * 1000)
            payload = {
                "worker_id": self.config.worker_id,
                "model_version": self.engine.model_version,
                "config_hash": self._config_hash(),
                "input_hash": image_hash.hexdigest(),
                "duration_ms": duration_ms,
                "preprocess_profile": self.config.preprocess_profile,
                "runtime_task_id": runtime_task_id,
                "runtime_lease_token": lease_token,
                "results": results,
            }
            heartbeat.stop()
            self.api.complete_task(task_id, payload, tenant_id)

    def _config_hash(self) -> str:
        if self.config.config_hash:
            return self.config.config_hash
        raw = (
            f"{self.config.engine}|{self.config.engine_version}|{self.engine.model_version}|"
            f"{self.engine.device}|{self.config.preprocess_profile}"
        ).encode()
        return hashlib.sha256(raw).hexdigest()

    def _runtime_incompatibility(self, runtime_task: dict[str, Any]) -> str | None:
        payload = runtime_task.get("payload")
        if not isinstance(payload, dict):
            return "invalid_ocr_runtime_payload"
        engine = payload.get("engine")
        engine_version = payload.get("engine_version")
        if not isinstance(engine, str) or not engine.strip():
            return "invalid_ocr_runtime_payload"
        if not isinstance(engine_version, str) or not engine_version.strip():
            return "invalid_ocr_runtime_payload"
        if engine != self.config.engine:
            return "unsupported_ocr_engine"
        if engine_version != self.config.engine_version:
            return "unsupported_ocr_engine_version"
        return None


class _LeaseHeartbeat:
    def __init__(
        self,
        *,
        api: Any,
        runtime_task_id: str,
        lease_token: str,
        worker_id: str,
        interval: float,
        lease_seconds: int,
        request_timeout: float,
        tenant_id: str,
    ) -> None:
        self.api = api
        self.runtime_task_id = runtime_task_id
        self.lease_token = lease_token
        self.worker_id = worker_id
        self.interval = interval
        self.lease_seconds = lease_seconds
        self.request_timeout = request_timeout
        self.tenant_id = tenant_id
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None
        self._error: Exception | None = None

    def __enter__(self) -> Self:
        self._send()
        self._thread = threading.Thread(target=self._run, name=f"ocr-heartbeat-{self.worker_id}", daemon=True)
        self._thread.start()
        return self

    def __exit__(self, _exc_type: object, _exc: object, _traceback: object) -> None:
        try:
            self.stop()
        except Exception:
            if _exc_type is None:
                raise

    def stop(self) -> None:
        self._stop.set()
        thread = self._thread
        if thread is None:
            return
        thread.join(timeout=self.request_timeout + 0.5)
        if thread.is_alive():
            raise RuntimeError("heartbeat request did not stop within its bounded timeout")
        self._thread = None
        self.raise_if_failed()

    def raise_if_failed(self) -> None:
        if self._error is not None:
            raise self._error

    def _run(self) -> None:
        while not self._stop.wait(self.interval):
            try:
                self._send()
            except Exception as exc:  # noqa: BLE001 - heartbeat transports may raise non-API exceptions.
                self._error = exc
                return

    def _send(self) -> None:
        self.api.heartbeat_task(
            self.runtime_task_id,
            self.lease_token,
            self.worker_id,
            self.lease_seconds,
            self.request_timeout,
            self.tenant_id,
        )


def _valid_block(text: str, bbox: list[float], confidence: float) -> bool:
    if not text.strip() or len(bbox) != 4 or not math.isfinite(confidence) or confidence < 0 or confidence > 1:
        return False
    if not all(math.isfinite(value) for value in bbox):
        return False
    return bbox[0] >= 0 and bbox[1] >= 0 and bbox[2] > 0 and bbox[3] > 0

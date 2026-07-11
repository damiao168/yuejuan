from __future__ import annotations

import hashlib
import json
import time

from page_processing.api import Client
from page_processing.barcode import detect_barcodes
from page_processing.config import Config
from page_processing.decoder import DecodeError, decode_document
from page_processing.registration import RegistrationError, crop_regions, register_page


class Runner:
    def __init__(self, client: Client, config: Config) -> None:
        self.client = client
        self.config = config

    def run_once(self) -> int:
        tasks = self.client.claim(self.config.worker_id, self.config.batch_size, self.config.lease_seconds)
        for task in tasks:
            try:
                if task.get("task_type") == "capture_file_decode":
                    self._process_capture(task)
                elif task.get("task_type") == "page_registration":
                    self._process_registration(task)
                else:
                    self.client.fail_task(task, "unsupported_page_processing_task", {"task_type": task.get("task_type")})
            except Exception as exc:
                self._fail(task, exc)
        return len(tasks)

    def run_forever(self) -> None:
        while True:
            if self.run_once() == 0:
                time.sleep(self.config.poll_interval)

    def _process_capture(self, task: dict) -> None:
        started = time.perf_counter()
        payload = task.get("payload") or {}
        self.client.heartbeat(task, self.config.worker_id, self.config.lease_seconds)
        source = self.client.download(str(payload["download_url"]))
        pages, detected_type = decode_document(
            source,
            str(payload.get("content_type") or ""),
            render_dpi=int(payload.get("render_dpi") or 300),
            max_pages=min(int(payload.get("max_pages") or self.config.max_pages), self.config.max_pages),
            max_page_pixels=self.config.max_page_pixels,
            max_total_pixels=self.config.max_total_pixels,
        )
        page_results = []
        for page in pages:
            uploaded = self.client.upload_page(task, page.index, page.png)
            page_results.append({"source_index": page.index, "file_asset_id": uploaded["id"], "sha256": uploaded["hash_sha256"], "width": page.width, "height": page.height, "barcodes": detect_barcodes(page.png)})
        result_version = "sha256:" + hashlib.sha256(json.dumps(page_results, sort_keys=True).encode("utf-8")).hexdigest()
        self.client.complete(str(payload["capture_file_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "result_version": result_version, "duration_ms": int((time.perf_counter() - started) * 1000), "decoder_profile": "pdfium-pillow-v1", "pages": page_results, "original_page_count": len(pages), "detected_content_type": detected_type})

    def _process_registration(self, task: dict) -> None:
        started = time.perf_counter()
        payload = task.get("payload") or {}
        self.client.heartbeat(task, self.config.worker_id, self.config.lease_seconds)
        source = self.client.download(str(payload["source_download_url"]))
        template = self.client.download(str(payload["template_download_url"]))
        output = register_page(
            source,
            str(payload.get("source_content_type") or "image/png"),
            template,
            str(payload.get("template_content_type") or ""),
            render_dpi=int(payload.get("render_dpi") or 300),
            template_page_index=int(payload.get("template_page_index") or 1),
        )
        run_id = str(payload["registration_run_id"])
        registered = self.client.upload_asset(task, "page_registration_output", run_id, "registered.png", output.registered_png)
        segments = []
        for index, crop in enumerate(crop_regions(output.registered_png, list(payload.get("question_regions") or [])), start=1):
            asset = self.client.upload_asset(task, "answer_segment_crop", run_id, f"segment-{index:04d}.png", crop.pop("png"))
            segments.append({**crop, "file_asset_id": asset["id"], "sha256": asset["hash_sha256"]})
        evidence = output.evidence
        result_version = "sha256:" + hashlib.sha256(json.dumps({"registered": registered["hash_sha256"], "segments": segments}, sort_keys=True).encode()).hexdigest()
        self.client.complete_registration(run_id, {
            "task_id": task["id"], "lease_token": task["lease_token"], "result_version": result_version,
            "duration_ms": int((time.perf_counter() - started) * 1000), "registered_file_asset_id": registered["id"],
            "registered_sha256": registered["hash_sha256"], "method": evidence.method, "confidence": evidence.confidence,
            "source_to_template_matrix": evidence.source_to_template, "template_to_source_matrix": evidence.template_to_source,
            "feature_count": evidence.feature_count, "match_count": evidence.match_count, "inlier_count": evidence.inlier_count,
            "inlier_ratio": evidence.inlier_ratio, "reprojection_error": evidence.reprojection_error, "coverage": evidence.coverage,
            "segments": segments,
        })

    def _fail(self, task: dict, exc: Exception) -> None:
        payload = task.get("payload") or {}
        code = str(exc) if isinstance(exc, (DecodeError, RegistrationError)) else "page_processing_failed"
        detail = {"error_type": type(exc).__name__, "message": str(exc)[:200]}
        if task.get("task_type") == "capture_file_decode":
            self.client.fail(str(payload["capture_file_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "retryable": not isinstance(exc, DecodeError), "error_code": code, "error_detail": detail, "duration_ms": 0})
        elif task.get("task_type") == "page_registration" and payload.get("registration_run_id"):
            self.client.fail_registration(str(payload["registration_run_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "retryable": not isinstance(exc, RegistrationError), "error_code": code, "error_detail": detail, "duration_ms": 0})
        else:
            self.client.fail_task(task, code, detail)

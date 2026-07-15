from __future__ import annotations

import hashlib
import json
import time

from page_processing.api import Client
from page_processing.barcode import detect_barcodes
from page_processing.config import Config
from page_processing.decoder import DecodeError, decode_document
from page_processing.registration import RegistrationError, crop_regions, register_page, register_page_manual
from page_processing.omr import OMRExtractionError, OMRProfile, extract_marks


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
                elif task.get("task_type") == "page_registration_correction_preview":
                    self._process_correction(task)
                elif task.get("task_type") == "omr_extract":
                    self._process_omr(task)
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

    def _process_correction(self, task: dict) -> None:
        started = time.perf_counter()
        payload = task.get("payload") or {}
        self.client.heartbeat(task, self.config.worker_id, self.config.lease_seconds)
        source = self.client.download(str(payload["source_download_url"]))
        template = self.client.download(str(payload["template_download_url"]))
        output = register_page_manual(
            source, str(payload.get("source_content_type") or "image/png"),
            template, str(payload.get("template_content_type") or ""),
            list(payload.get("source_points") or []), list(payload.get("template_points") or []),
            render_dpi=int(payload.get("render_dpi") or 300),
            template_page_index=int(payload.get("template_page_index") or 1),
        )
        correction_id = str(payload["correction_id"])
        registered = self.client.upload_asset(task, "page_registration_correction_preview", correction_id, "registered-preview.png", output.registered_png)
        segments = []
        for index, crop in enumerate(crop_regions(output.registered_png, list(payload.get("question_regions") or [])), start=1):
            asset = self.client.upload_asset(task, "page_registration_correction_preview", correction_id, f"segment-preview-{index:04d}.png", crop.pop("png"))
            segments.append({**crop, "file_asset_id": asset["id"], "sha256": asset["hash_sha256"]})
        evidence = output.evidence
        result_version = "sha256:" + hashlib.sha256(json.dumps({"registered": registered["hash_sha256"], "segments": segments}, sort_keys=True).encode()).hexdigest()
        self.client.complete_correction(correction_id, {
            "task_id": task["id"], "lease_token": task["lease_token"], "result_version": result_version,
            "duration_ms": int((time.perf_counter() - started) * 1000),
            "preview_registered_file_asset_id": registered["id"],
            "preview_registered_sha256": registered["hash_sha256"],
            "source_to_template_matrix": evidence.source_to_template,
            "template_to_source_matrix": evidence.template_to_source,
            "coverage": evidence.coverage, "reprojection_error": evidence.reprojection_error,
            "validation_report": {"passed": True, "orientation_valid": True, "regions_in_bounds": True, "coverage_valid": True},
            "segments": segments,
        })

    def _process_omr(self, task: dict) -> None:
        started = time.perf_counter()
        payload = task.get("payload") or {}
        self.client.heartbeat(task, self.config.worker_id, self.config.lease_seconds)
        source = self.client.download(str(payload["source_download_url"]))
        profile_data = payload.get("profile") or {}
        profile_hash = str(payload.get("profile_hash") or "").strip()
        if not profile_hash:
            raise OMRExtractionError("omr_profile_hash_missing")
        profile = OMRProfile(
            mode=str(profile_data.get("mode") or "manual_only"),
            marked_threshold=float(profile_data.get("marked_threshold", 0.18)),
            ambiguous_threshold=float(profile_data.get("ambiguous_threshold", 0.10)),
            minimum_margin=float(profile_data.get("minimum_margin", 0.06)),
            border_fraction=float(profile_data.get("border_fraction", 0.12)),
            version=str(profile_data.get("version") or "opencv-fill-v1"),
            reference_mask_dilation_pixels=int(profile_data.get("reference_mask_dilation_pixels", 0)),
        )
        reference_data = payload.get("reference")
        reference = reference_data if isinstance(reference_data, dict) else {}
        reference_bytes: bytes | None = None
        reference_sha256 = ""
        if profile.mode == "template_difference":
            reference_url = str(reference.get("download_url") or "").strip()
            reference_sha256 = str(reference.get("sha256") or "").strip().lower()
            if not str(reference.get("file_asset_id") or "").strip() or not reference_url or not reference_sha256:
                raise OMRExtractionError("omr_reference_missing")
            reference_bytes = self.client.download(reference_url)
            if hashlib.sha256(reference_bytes).hexdigest().lower() != reference_sha256:
                raise OMRExtractionError("omr_reference_hash_mismatch")
        result = extract_marks(
            source,
            list(payload.get("option_regions") or []),
            multiple=bool(payload.get("multiple")),
            profile=profile,
            reference_image_bytes=reference_bytes,
            reference_content_type=str(reference.get("content_type") or ""),
            reference_page_no=int(reference.get("page_no") or 1),
            reference_question_region=reference.get("question_region") if isinstance(reference.get("question_region"), dict) else None,
        )
        result["reference_sha256"] = reference_sha256
        run_id = str(payload["omr_run_id"])
        overlay = self.client.upload_asset(task, "omr_evidence", run_id, "omr-overlay.png", result.pop("overlay_png"))
        result_version = "sha256:" + hashlib.sha256(json.dumps({**result, "profile_hash": profile_hash}, sort_keys=True).encode()).hexdigest()
        self.client.complete_omr(run_id, {
            "task_id": task["id"], "lease_token": task["lease_token"],
            "result_version": result_version,
            "duration_ms": int((time.perf_counter() - started) * 1000),
            "decision": result["decision"], "selected": result["selected"],
            "confidence": result["confidence"], "needs_human_review": result["needs_human_review"],
            "measurements": result["measurements"], "profile_version": result["profile_version"],
            "profile_hash": profile_hash,
            "reference_sha256": result["reference_sha256"],
            "thresholds": result["thresholds"],
            "overlay_file_asset_id": overlay["id"], "overlay_sha256": overlay["hash_sha256"],
        })

    def _fail(self, task: dict, exc: Exception) -> None:
        payload = task.get("payload") or {}
        code = str(exc) if isinstance(exc, (DecodeError, RegistrationError, OMRExtractionError)) else "page_processing_failed"
        detail = {"error_type": type(exc).__name__, "message": str(exc)[:200]}
        if task.get("task_type") == "capture_file_decode":
            self.client.fail(str(payload["capture_file_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "retryable": not isinstance(exc, DecodeError), "error_code": code, "error_detail": detail, "duration_ms": 0})
        elif task.get("task_type") == "page_registration" and payload.get("registration_run_id"):
            self.client.fail_registration(str(payload["registration_run_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "retryable": not isinstance(exc, RegistrationError), "error_code": code, "error_detail": detail, "duration_ms": 0})
        elif task.get("task_type") == "page_registration_correction_preview" and payload.get("correction_id"):
            self.client.fail_correction(str(payload["correction_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "retryable": not isinstance(exc, RegistrationError), "error_code": code, "error_detail": detail, "duration_ms": 0})
        elif task.get("task_type") == "omr_extract" and payload.get("omr_run_id"):
            self.client.fail_omr(str(payload["omr_run_id"]), {"task_id": task["id"], "lease_token": task["lease_token"], "retryable": not isinstance(exc, OMRExtractionError), "error_code": code, "error_detail": detail, "duration_ms": 0})
        else:
            self.client.fail_task(task, code, detail)

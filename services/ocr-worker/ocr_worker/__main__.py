from __future__ import annotations

import logging
import time

from .api import APIError, AuthenticationError, EduGradeClient
from .config import load_settings, resolve_formula_plan
from .engine import PaddleOCREngine
from .formula_validation import FormulaValidator
from .math_layout import PaddleFormulaLayoutDetector
from .math_runner import MathUnderstandingRunner, MathVerificationClient
from .paper_formula import PaperFormulaRunner
from .recognition_router import PaddleFormulaNetEngine, RecognitionRouter
from .runner import OCRRunner, WorkerConfig
from .symbolic_runner import SymbolicVerificationRunner


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    log = logging.getLogger("edugrade.ocr_worker")
    settings = load_settings()
    engine = PaddleOCREngine(
        device=settings.device,
        model_version=settings.model_version,
        cpu_threads=getattr(settings, "cpu_threads", 4),
        enable_mkldnn=getattr(settings, "enable_mkldnn", "auto"),
        enable_hpi=getattr(settings, "enable_hpi", False),
        use_textline_orientation=getattr(settings, "use_textline_orientation", True),
        text_det_limit_type=getattr(settings, "text_det_limit_type", "min"),
        text_det_limit_side_len=getattr(settings, "text_det_limit_side_len", 64),
        text_recognition_batch_size=getattr(settings, "text_recognition_batch_size", 1),
    )
    if getattr(settings, "ocr_runtime_enabled", True) or getattr(settings, "math_runtime_enabled", False):
        log.info("initializing OCR engine before worker registration")
        engine.initialize()
        log.info(
            "OCR engine ready: model=%s device=%s cpu_threads=%s effective_mkldnn=%s hpi=%s orientation=%s",
            engine.model_version, engine.device, engine.cpu_threads, engine.effective_mkldnn,
            engine.enable_hpi, engine.use_textline_orientation,
        )
    client = EduGradeClient(
        base_url=settings.api_base_url,
        tenant_code=settings.tenant_code,
        username=settings.username,
        password=settings.password,
    )
    worker_config = WorkerConfig(
        worker_id=settings.worker_id,
        batch_size=settings.batch_size,
        engine=settings.engine,
        engine_version=settings.engine_version,
        preprocess_profile=settings.preprocess_profile,
        lease_seconds=settings.lease_seconds,
        heartbeat_interval=settings.heartbeat_interval,
        heartbeat_timeout=settings.heartbeat_timeout,
        cpu_threads=getattr(settings, "cpu_threads", 4),
        enable_mkldnn=getattr(settings, "enable_mkldnn", "auto"),
        enable_hpi=getattr(settings, "enable_hpi", False),
        use_textline_orientation=getattr(settings, "use_textline_orientation", True),
        text_det_limit_type=getattr(settings, "text_det_limit_type", "min"),
        text_det_limit_side_len=getattr(settings, "text_det_limit_side_len", 64),
        text_recognition_batch_size=getattr(settings, "text_recognition_batch_size", 1),
    )
    runner = None
    if getattr(settings, "ocr_runtime_enabled", True):
        runner = OCRRunner(api=client, engine=engine, config=worker_config)
    math_runner = None
    symbolic_runner = None
    if getattr(settings, "math_runtime_enabled", False):
        formula_engine = None
        formula_detector = None
        try:
            formula_engine = PaddleFormulaNetEngine(model_version=settings.formula_model_version, device=settings.device)
            formula_engine.initialize()
        except Exception:
            formula_engine = None
            log.exception("formula engine unavailable; mathematical formula regions will require human review")
        try:
            formula_detector = PaddleFormulaLayoutDetector(settings.formula_layout_model_version, settings.device)
            formula_detector.initialize()
        except Exception:
            formula_detector = None
            log.exception("formula layout detector unavailable; mixed mathematical regions will require human review")
        math_runner = MathUnderstandingRunner(
            api=client,
            router=RecognitionRouter(
                text_engine=engine,
                formula_engine=formula_engine,
                formula_detector=formula_detector,
                formula_batch_size=getattr(settings, "formula_recognition_batch_size", None) or 4,
                formula_max_padding=getattr(settings, "formula_max_padding_pixels", 96),
                formula_max_padding_height_ratio=getattr(settings, "formula_max_padding_height_ratio", 0.75),
            ),
            verifier=MathVerificationClient(settings.math_verify_base_url, settings.math_verify_token),
            config=worker_config,
        )
        symbolic_runner = SymbolicVerificationRunner(
            api=client, verifier=MathVerificationClient(settings.math_verify_base_url, settings.math_verify_token),
            config=worker_config,
        )
        log.info(
            "math understanding runtime ready: formula_model=%s formula_available=%s layout_model=%s layout_available=%s",
            settings.formula_model_version,
            formula_engine is not None,
            settings.formula_layout_model_version,
            formula_detector is not None,
        )
    paper_formula_runner = None
    formula_model_lifecycle = "resident"
    formula_processed_since_start = False
    if getattr(settings, "paper_formula_runtime_enabled", False):
        detector = PaddleFormulaLayoutDetector(settings.formula_layout_model_version, settings.device)
        formula_cache_size = getattr(settings, "formula_result_cache_size", 2048)
        primary = PaddleFormulaNetEngine(
            model_version=settings.formula_model_version,
            device=settings.device,
            cache_size=formula_cache_size,
        )
        fallback = PaddleFormulaNetEngine(
            model_version=settings.formula_fallback_model_version,
            device=settings.device,
            cache_size=formula_cache_size,
        )
        formula_plan = resolve_formula_plan(settings)
        formula_model_lifecycle = formula_plan.lifecycle
        log.info(
            "paper formula runtime plan: requested=%s effective=%s batch=%s source=%s reason=%s device=%s profile=%s",
            settings.formula_model_lifecycle,
            formula_model_lifecycle,
            formula_plan.batch_size,
            formula_plan.source,
            formula_plan.reason,
            settings.device,
            formula_plan.profile_sha256 or "none",
        )
        if formula_model_lifecycle == "resident":
            log.info("preloading paper formula detector and plus-M model")
            detector.initialize()
            primary.initialize()
        else:
            log.info("paper formula detector and plus-M will load after a task is claimed")
        if formula_model_lifecycle == "resident" and getattr(settings, "formula_prewarm_fallback", False):
            log.info("preloading governed plus-L fallback model")
            fallback.initialize()
        paper_formula_runner = PaperFormulaRunner(
            api=client,
            config=worker_config,
            detector=detector,
            primary=primary,
            fallback=fallback,
            max_padding=getattr(settings, "formula_max_padding_pixels", 96),
            max_padding_height_ratio=getattr(settings, "formula_max_padding_height_ratio", 0.75),
            batch_size=formula_plan.batch_size,
            runtime_plan={
                "runtime_mode": formula_plan.lifecycle,
                "runtime_plan_source": formula_plan.source,
                "runtime_batch_size": formula_plan.batch_size,
            },
            validator=FormulaValidator(
                render_similarity_threshold=getattr(settings, "formula_render_similarity_threshold", 0.34),
            ),
        )
        log.info("paper formula runtime ready: detector=%s primary=%s fallback=%s", settings.formula_layout_model_version, settings.formula_model_version, settings.formula_fallback_model_version)
    while True:
        try:
            if client.token is None:
                client.login()
                log.info("worker authenticated")
            processed = runner.process_once() if runner is not None else 0
            if math_runner is not None:
                processed += math_runner.process_once()
            if symbolic_runner is not None:
                processed += symbolic_runner.process_once()
            if paper_formula_runner is not None:
                formula_processed = paper_formula_runner.process_once()
                processed += formula_processed
                formula_processed_since_start = formula_processed_since_start or formula_processed > 0
                if formula_processed_since_start and formula_processed == 0 and formula_model_lifecycle == "per_job":
                    # Paddle/oneDNN may retain native allocator arenas even
                    # after Python references are released. A clean process
                    # exit is the only reliable way to return that memory. A
                    # burst is drained first, so queued exams reuse one load
                    # without an arbitrary idle timeout.
                    log.info("paper formula queue drained; per_job lifecycle is releasing model memory")
                    return
            if processed == 0:
                time.sleep(settings.poll_interval)
        except AuthenticationError:
            client.token = None
            log.warning("worker authentication expired; login will be retried")
            time.sleep(settings.poll_interval)
        except APIError as exc:
            log.warning("worker API request failed; polling will continue", extra={"error_type": type(exc).__name__})
            time.sleep(settings.poll_interval)
        except Exception:
            log.exception("unexpected OCR worker error; polling will continue")
            time.sleep(settings.poll_interval)


if __name__ == "__main__":
    main()

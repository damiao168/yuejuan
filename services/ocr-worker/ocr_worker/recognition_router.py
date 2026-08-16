from __future__ import annotations

import tempfile
from collections.abc import Callable
from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
from typing import Any, Protocol

from .engine import OCRBlock, OCREngine


class RegionKind(StrEnum):
    TEXT = "text"
    FORMULA = "formula"
    DIAGRAM = "diagram"
    UNKNOWN = "unknown"


@dataclass(frozen=True)
class FormulaRecognitionResult:
    raw_latex: str
    canonical_latex: str
    confidence: float
    engine: str
    engine_version: str
    status: str
    warnings: tuple[str, ...] = ()


@dataclass(frozen=True)
class RoutedRecognitionResult:
    kind: RegionKind
    text_blocks: tuple[OCRBlock, ...] = ()
    formula: FormulaRecognitionResult | None = None
    preserve_image_evidence: bool = False
    requires_human_review: bool = False
    reason_code: str = ""


class FormulaEngine(Protocol):
    engine_name: str
    model_version: str

    def initialize(self) -> None: ...

    def recognize_formula(self, image_bytes: bytes) -> FormulaRecognitionResult: ...


class RecognitionRouter:
    """Routes already detected regions without creating a second OCR task system."""

    def __init__(
        self,
        *,
        text_engine: OCREngine,
        formula_engine: FormulaEngine | None,
        formula_subjects: frozenset[str] = frozenset({"mathematics", "physics", "chemistry"}),
    ) -> None:
        self.text_engine = text_engine
        self.formula_engine = formula_engine
        self.formula_subjects = formula_subjects

    def recognize(self, subject_code: str, kind: RegionKind, image_bytes: bytes) -> RoutedRecognitionResult:
        if not image_bytes:
            return RoutedRecognitionResult(kind=kind, requires_human_review=True, reason_code="empty_region")
        if kind is RegionKind.TEXT:
            blocks = tuple(self.text_engine.recognize(image_bytes))
            return RoutedRecognitionResult(
                kind=kind,
                text_blocks=blocks,
                requires_human_review=not blocks,
                reason_code="" if blocks else "empty_text_result",
            )
        if kind is RegionKind.FORMULA:
            if subject_code not in self.formula_subjects:
                return RoutedRecognitionResult(
                    kind=kind,
                    preserve_image_evidence=True,
                    requires_human_review=True,
                    reason_code="formula_not_enabled_for_subject",
                )
            if self.formula_engine is None:
                return RoutedRecognitionResult(kind=kind, requires_human_review=True, reason_code="formula_engine_unavailable")
            try:
                result = self.formula_engine.recognize_formula(image_bytes)
            except Exception:  # noqa: BLE001 - optional model runtimes expose heterogeneous errors.
                return RoutedRecognitionResult(kind=kind, requires_human_review=True, reason_code="formula_engine_failed")
            unsafe = result.status != "recognized" or not result.canonical_latex or result.confidence <= 0
            return RoutedRecognitionResult(
                kind=kind,
                formula=result,
                requires_human_review=unsafe,
                reason_code="ambiguous_formula" if unsafe else "",
            )
        if kind is RegionKind.DIAGRAM:
            return RoutedRecognitionResult(kind=kind, preserve_image_evidence=True, requires_human_review=True, reason_code="diagram_requires_review")
        return RoutedRecognitionResult(kind=RegionKind.UNKNOWN, preserve_image_evidence=True, requires_human_review=True, reason_code="unknown_region")


class PaddleFormulaNetEngine:
    engine_name = "paddle-formula"

    def __init__(self, *, model_version: str = "PP-FormulaNet_plus-M", device: str = "cpu") -> None:
        if model_version not in {"PP-FormulaNet_plus-M", "PP-FormulaNet_plus-L"}:
            raise ValueError("unsupported PP-FormulaNet model")
        self.model_version = model_version
        self.device = device
        self._pipeline: Any = None

    def initialize(self) -> None:
        self._load()

    def recognize_formula(self, image_bytes: bytes) -> FormulaRecognitionResult:
        if not image_bytes:
            return FormulaRecognitionResult("", "", 0, self.engine_name, self.model_version, "failed", ("empty_image",))
        pipeline = self._load()
        with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as tmp:
            tmp.write(image_bytes)
            image_path = tmp.name
        try:
            raw = pipeline.predict(input=image_path)
            latex, confidence = _parse_formula_prediction(raw)
        finally:
            Path(image_path).unlink(missing_ok=True)
        canonical = canonicalize_latex(latex)
        return FormulaRecognitionResult(
            raw_latex=latex,
            canonical_latex=canonical,
            confidence=confidence,
            engine=self.engine_name,
            engine_version=self.model_version,
            status="recognized" if canonical else "failed",
        )

    def _load(self) -> Any:
        if self._pipeline is not None:
            return self._pipeline
        try:
            from paddleocr import FormulaRecognition
        except ImportError as exc:
            raise RuntimeError("PaddleOCR formula runtime is not installed") from exc
        self._pipeline = FormulaRecognition(model_name=self.model_version, device=self.device)
        return self._pipeline


class UniMERNetEngine:
    """Challenger adapter; deployment must inject a versioned, governed predictor."""

    engine_name = "unimernet"

    def __init__(self, *, model_version: str, predictor: Callable[[bytes], tuple[str, float]] | None = None) -> None:
        self.model_version = model_version
        self._predictor = predictor

    def initialize(self) -> None:
        if self._predictor is None:
            raise RuntimeError("UniMERNet predictor is not configured")

    def recognize_formula(self, image_bytes: bytes) -> FormulaRecognitionResult:
        self.initialize()
        assert self._predictor is not None
        latex, confidence = self._predictor(image_bytes)
        canonical = canonicalize_latex(latex)
        return FormulaRecognitionResult(latex, canonical, confidence, self.engine_name, self.model_version, "recognized" if canonical else "failed")


def canonicalize_latex(value: str) -> str:
    return "".join(value.replace("\\left", "").replace("\\right", "").split())


def _parse_formula_prediction(raw: object) -> tuple[str, float]:
    pages = raw if isinstance(raw, list) else [raw]
    for page in pages:
        payload = getattr(page, "json", page)
        if callable(payload):
            payload = payload()
        if not isinstance(payload, dict):
            continue
        nested = payload.get("res") if isinstance(payload.get("res"), dict) else payload
        latex = nested.get("rec_formula") or nested.get("formula") or nested.get("latex")
        if isinstance(latex, str) and latex.strip():
            score = nested.get("rec_score", nested.get("score", nested.get("confidence", 0.0)))
            return latex, float(score or 0.0)
    return "", 0.0

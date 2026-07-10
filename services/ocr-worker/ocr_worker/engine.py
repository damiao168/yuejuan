from __future__ import annotations

import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Protocol


@dataclass(frozen=True)
class OCRBlock:
    text: str
    bbox: list[float]
    confidence: float


class OCREngine(Protocol):
    def recognize(self, image_bytes: bytes) -> list[OCRBlock]:
        ...


class PaddleOCREngine:
    def __init__(self, *, device: str = "cpu", language: str = "ch", model_version: str = "ppocr-v5-server") -> None:
        self.device = device
        self.language = language
        self.model_version = model_version
        self._ocr = None

    def recognize(self, image_bytes: bytes) -> list[OCRBlock]:
        if not image_bytes:
            return []
        ocr = self._load()
        with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as tmp:
            tmp.write(image_bytes)
            image_path = tmp.name
        try:
            raw = _run_paddle_ocr(ocr, image_path)
            return _parse_paddle_result(raw)
        finally:
            Path(image_path).unlink(missing_ok=True)

    def _load(self):
        if self._ocr is not None:
            return self._ocr
        try:
            from paddleocr import PaddleOCR
        except ImportError as exc:
            raise RuntimeError("PaddleOCR is not installed. Install edugrade-ocr-worker[paddle].") from exc
        self._ocr = _create_paddle_ocr(PaddleOCR, language=self.language, device=self.device)
        return self._ocr


def _parse_paddle_result(raw: object) -> list[OCRBlock]:
    blocks: list[OCRBlock] = []
    if not isinstance(raw, list):
        return blocks
    for page in raw:
        page_json = getattr(page, "json", None)
        if callable(page_json):
            page_json = page_json()
        if isinstance(page_json, dict):
            blocks.extend(_parse_paddle3_page(page_json))
            continue
        if isinstance(page, dict):
            blocks.extend(_parse_paddle3_page(page))
            continue
        if not isinstance(page, list):
            continue
        for item in page:
            if not isinstance(item, list) or len(item) < 2:
                continue
            bbox = _bbox_from_points(item[0])
            text, confidence = _text_confidence(item[1])
            if text:
                blocks.append(OCRBlock(text=text, bbox=bbox, confidence=confidence))
    return blocks


def _create_paddle_ocr(paddle_ocr_cls: Any, *, language: str, device: str) -> Any:
    try:
        return paddle_ocr_cls(
            lang=language,
            use_doc_orientation_classify=False,
            use_doc_unwarping=False,
            use_textline_orientation=True,
        )
    except ValueError as exc:
        if "Unknown argument" not in str(exc):
            raise
    use_gpu = device.lower().startswith("gpu")
    return paddle_ocr_cls(use_angle_cls=True, lang=language, use_gpu=use_gpu)


def _run_paddle_ocr(ocr: Any, image_path: str) -> object:
    if hasattr(ocr, "predict"):
        return ocr.predict(image_path)
    return ocr.ocr(image_path, cls=True)


def _parse_paddle3_page(page: dict[str, object]) -> list[OCRBlock]:
    texts = page.get("rec_texts")
    scores = page.get("rec_scores")
    polys = page.get("rec_polys") or page.get("dt_polys")
    if not isinstance(texts, list):
        blocks: list[OCRBlock] = []
        for value in page.values():
            if isinstance(value, dict):
                blocks.extend(_parse_paddle3_page(value))
        return blocks
    if not isinstance(scores, list):
        scores = [0.0] * len(texts)
    if not isinstance(polys, list):
        polys = [[] for _ in texts]
    blocks: list[OCRBlock] = []
    for index, text in enumerate(texts):
        if not str(text):
            continue
        score = float(scores[index]) if index < len(scores) else 0.0
        poly = polys[index] if index < len(polys) else []
        blocks.append(OCRBlock(text=str(text), bbox=_bbox_from_points(poly), confidence=score))
    return blocks


def _bbox_from_points(value: object) -> list[float]:
    if not isinstance(value, list) or not value:
        return [0.0, 0.0, 1.0, 1.0]
    xs: list[float] = []
    ys: list[float] = []
    for point in value:
        if isinstance(point, list) and len(point) >= 2:
            xs.append(float(point[0]))
            ys.append(float(point[1]))
    if not xs or not ys:
        return [0.0, 0.0, 1.0, 1.0]
    left = min(xs)
    top = min(ys)
    return [left, top, max(xs) - left, max(ys) - top]


def _text_confidence(value: object) -> tuple[str, float]:
    if isinstance(value, tuple) and len(value) >= 2:
        return str(value[0]), float(value[1])
    if isinstance(value, list) and len(value) >= 2:
        return str(value[0]), float(value[1])
    return "", 0.0

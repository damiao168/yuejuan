from __future__ import annotations

import base64
import math
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
    model_version: str
    device: str

    def initialize(self) -> None:
        ...

    def recognize(self, image_bytes: bytes) -> list[OCRBlock]:
        ...


_PADDLE_MODEL_PROFILES = {
    "ppocr-v5-mobile": {
        "text_detection_model_name": "PP-OCRv5_mobile_det",
        "text_recognition_model_name": "PP-OCRv5_mobile_rec",
    },
    "ppocr-v5-server": {
        "text_detection_model_name": "PP-OCRv5_server_det",
        "text_recognition_model_name": "PP-OCRv5_server_rec",
    }
}

# A valid 320x120 RGB PNG containing large, high-contrast "OCR 123" text.
# Readiness requires a recognized block so both detection and recognition run.
_READINESS_PROBE_IMAGE = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAUAAAAB4CAIAAAAMrLyJAAAD0ElEQVR42u3cO0hcWQCA4assChIQhLHQwkZUIgFxCgkmcYyPJpaCTWBCII1gJWqTMo3YRVIE0YykCaksVHyBsdCAhYgQDIRUBgXBF4iPMXq2GPZu2M2m0WbD91X3nnM9xRz+metlmLwQQgT8P+V7CUDAgIABAYOAAQEDAgYEDAIGBAwIGAQMCBgQMCBgEDDw2wb85s2bZDJ59+7dZDI5Pj6eGxwZGamvr29qanr06NHW1lZusKioKJVKNTU11dfXLy0txSscHh4+efKkuLg4HhkbG7t//35dXd3c3FwURScnJ11dXalUKplMTk1N2TP4W7iGmZmZxsbGg4ODEMLBwUFjY+P8/Pzc3Fxzc/PJyUkIYXp6+uHDh7mLi4uLcwcbGxt37tyJF7l3797Lly/j2d3d3QcPHlxeXm5ubtbU1IQQBgcHh4aGQgjb29sVFRUB+Mu1Am5paVlZWYlPl5eXW1tb29vbP378GA8+e/Ysm83+GPDV1VVJSUl8wc7Ozo+zm5ub79+/DyEcHx8nEokQwv7+/vn5eQhhfn6+srLSnsHNBFxWVnZ6ehqfnp6elpWVlZeXn52d/fviONGZmZnOzs7/mo1lMpmnT5/Gp48fPy4qKlpYWLBnEPvjZu/G8/LyLi8vfzqbzWZTqdTFxcXnz58/ffr066W+fv06NDS0uLgYj7x9+7azs3N8fLylpcU/PnADD7Fu3769trYWn66trdXW1lZVVa2vr8dJp9Pp3HFBQcGHDx+Wl5cHBgYymcwvlj0+Pu7q6hodHU0kElEU9fT0fP/+PYqijo4OD7HgxgLu6+vr7+8/OjrKPUweGBjo7+/v7u5+/vz5+fl5FEXv3r3LHfyora1tdXX1Fx/j6XS6t7e3oaEhN3J0dDQxMRFF0crKSnV1tT2D2LVuodvb2799+9bc3FxYWJjNZnt6enL3t1++fEkmk4lEorS09NWrV//4q+rq6o2Njaurq/z8n7x9ZDKZ2dnZvb29169f37p1a3Jy8sWLF+l0enh4uKCgYGxszJ5BLM8Pu4NvYgECBgQMAgYEDAgYEDAIGBAwIGAQMCBgQMCAgEHAgIABAQMCBgEDAgYEDAIGBAwIGBAwCBgQMCBgQMAgYEDAgIBBwICAAQEDAgYBAwIGBAwCBgQMCBgQMAgYEDAgYEDAIGBAwICAQcCAgAEBAwIGAQMCBgQMCBgEDAgYEDAIGBAwIGBAwCBgQMCAgAEBg4ABAQMCBgEDAgYEDAgYBAwIGBAwCBgQMCBgQMAgYEDAgIABAYOAAQEDAgYBAwIGBAwIGH5nfwJra5FFuOxszQAAAABJRU5ErkJggg=="
)


class PaddleOCREngine:
    def __init__(self, *, device: str = "cpu", model_version: str = "ppocr-v5-mobile") -> None:
        normalized_device = device.strip().lower()
        if normalized_device != "cpu":
            raise ValueError("the packaged paddlepaddle runtime supports only EDUGRADE_OCR_DEVICE=cpu")
        if model_version not in _PADDLE_MODEL_PROFILES:
            supported = ", ".join(sorted(_PADDLE_MODEL_PROFILES))
            raise ValueError(f"unsupported OCR model version {model_version!r}; expected one of: {supported}")
        self.device = normalized_device
        self.model_version = model_version
        self._ocr = None
        self._ready = False

    def initialize(self) -> None:
        if self._ready:
            return
        blocks = self.recognize(_READINESS_PROBE_IMAGE)
        if not any(_valid_readiness_block(block) for block in blocks):
            raise RuntimeError("OCR readiness probe produced no valid recognized text")
        self._ready = True

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
        self._ocr = _create_paddle_ocr(
            PaddleOCR,
            model_version=self.model_version,
            device=self.device,
        )
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


def _create_paddle_ocr(paddle_ocr_cls: Any, *, model_version: str, device: str) -> Any:
    profile = _PADDLE_MODEL_PROFILES.get(model_version)
    if profile is None:
        raise ValueError(f"unsupported OCR model version: {model_version}")
    return paddle_ocr_cls(
        **profile,
        device=device,
        # PaddlePaddle 3.3.1's oneDNN executor cannot run the PP-OCRv5
        # server model's PIR array attributes on CPU. The plain CPU executor
        # is slower but fully supported and preserves model correctness.
        enable_mkldnn=False,
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=True,
    )


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
        return [0.0, 0.0, 0.0, 0.0]
    xs: list[float] = []
    ys: list[float] = []
    for point in value:
        if isinstance(point, list) and len(point) >= 2:
            xs.append(float(point[0]))
            ys.append(float(point[1]))
    if not xs or not ys:
        return [0.0, 0.0, 0.0, 0.0]
    left = min(xs)
    top = min(ys)
    return [left, top, max(xs) - left, max(ys) - top]


def _text_confidence(value: object) -> tuple[str, float]:
    if isinstance(value, tuple) and len(value) >= 2:
        return str(value[0]), float(value[1])
    if isinstance(value, list) and len(value) >= 2:
        return str(value[0]), float(value[1])
    return "", 0.0


def _valid_readiness_block(block: OCRBlock) -> bool:
    return (
        bool(block.text.strip())
        and len(block.bbox) == 4
        and all(math.isfinite(value) for value in block.bbox)
        and block.bbox[2] > 0
        and block.bbox[3] > 0
        and math.isfinite(block.confidence)
        and 0 < block.confidence <= 1
    )

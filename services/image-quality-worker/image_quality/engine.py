from __future__ import annotations

import io
from typing import Any

import cv2
import numpy as np
from PIL import Image, ImageOps, UnidentifiedImageError

from image_quality.schemas import QualityAnalysisResult

METRIC_SCHEMA_VERSION = "image-quality-metrics-v1"
REPORT_SCHEMA_VERSION = "image-quality-report-v1"
MAX_IMAGE_PIXELS = 50_000_000


class ImageQualityError(RuntimeError):
    pass


def analyze_and_normalize(data: bytes) -> QualityAnalysisResult:
    try:
        source = Image.open(io.BytesIO(data))
        if source.width <= 0 or source.height <= 0 or source.width * source.height > MAX_IMAGE_PIXELS:
            raise ImageQualityError("image_dimensions_out_of_range")
        source.load()
    except (UnidentifiedImageError, OSError, Image.DecompressionBombError) as exc:
        raise ImageQualityError("unreadable_image") from exc

    source_width, source_height = source.size
    exif_rotation = _exif_rotation_degrees(source)
    normalized_image = ImageOps.exif_transpose(source).convert("RGB")
    pixel_width, pixel_height = normalized_image.size

    gray = np.asarray(normalized_image.convert("L"))
    metrics = _metrics(gray)
    issues = _issues(metrics)
    quality_status = "review" if issues else "passed"

    output = io.BytesIO()
    normalized_image.save(output, format="PNG")

    report: dict[str, Any] = {
        "metric_schema_version": METRIC_SCHEMA_VERSION,
        "report_schema_version": REPORT_SCHEMA_VERSION,
        "pixel_width": pixel_width,
        "pixel_height": pixel_height,
        "source_dpi": None,
        "source_dpi_method": "missing_metadata",
        "render_dpi": None,
        "source_format": source.format or "unknown",
        "output_format": "image/png",
        "normalized_color_mode": "RGB",
        "metrics": metrics,
        "geometry": {
            "detected_skew_angle": 0.0,
            "skew_confidence": 0.0,
            "skew_correction_applied": False,
            "page_border_status": "unknown",
            "page_border_confidence": 0.0,
            "perspective_status": "unknown",
            "perspective_confidence": 0.0,
        },
    }
    transform = {
        "source_pixel_width": source_width,
        "source_pixel_height": source_height,
        "normalized_pixel_width": pixel_width,
        "normalized_pixel_height": pixel_height,
        "exif_rotation_degrees": exif_rotation,
        "content_rotation_degrees": 0,
        "detected_skew_angle": 0.0,
        "skew_correction_applied": False,
        "crop_box_source_pixels": [0, 0, source_width, source_height],
        "source_to_normalized_matrix": _identity_matrix(),
    }
    return QualityAnalysisResult(
        quality_status=quality_status,
        quality_report=report,
        quality_issues=issues,
        normalization_transform=transform,
        normalized_png=output.getvalue(),
    )


def _metrics(gray: np.ndarray) -> dict[str, float]:
    laplacian_variance = float(cv2.Laplacian(gray, cv2.CV_64F).var())
    sharpness_score = _clamp(laplacian_variance / 1000.0)
    brightness_score = _clamp(float(gray.mean()) / 255.0)
    contrast_score = _clamp(float(gray.std()) / 80.0)
    exposure_quality_score = _clamp(1.0 - abs(brightness_score - 0.72) / 0.72)
    blank_probability = _clamp(1.0 - contrast_score)
    return {
        "sharpness_score": round(sharpness_score, 4),
        "brightness_score": round(brightness_score, 4),
        "exposure_quality_score": round(exposure_quality_score, 4),
        "contrast_score": round(contrast_score, 4),
        "shadow_risk_score": 0.0,
        "blank_probability": round(blank_probability, 4),
        "perspective_risk_score": 0.0,
        "border_completeness_score": 0.0,
    }


def _issues(metrics: dict[str, float]) -> list[dict[str, Any]]:
    issues: list[dict[str, Any]] = []
    if metrics["sharpness_score"] < 0.35:
        issues.append(
            {
                "code": "low_sharpness",
                "severity": "review",
                "metric": "sharpness_score",
                "observed": metrics["sharpness_score"],
                "threshold": 0.35,
                "rule_id": "sharpness-min-v1",
                "action": "manual_review",
                "parameters": {},
            }
        )
    if metrics["blank_probability"] > 0.97:
        issues.append(
            {
                "code": "blank_page",
                "severity": "review",
                "metric": "blank_probability",
                "observed": metrics["blank_probability"],
                "threshold": 0.97,
                "rule_id": "blank-page-v1",
                "action": "manual_review",
                "parameters": {},
            }
        )
    return issues


def _exif_rotation_degrees(image: Image.Image) -> int:
    orientation = image.getexif().get(274)
    return {
        3: 180,
        6: 90,
        8: 270,
    }.get(orientation, 0)


def _identity_matrix() -> list[list[float]]:
    return [[1.0, 0.0, 0.0], [0.0, 1.0, 0.0], [0.0, 0.0, 1.0]]


def _clamp(value: float) -> float:
    return max(0.0, min(1.0, value))

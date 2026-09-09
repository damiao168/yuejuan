from __future__ import annotations

import io
from typing import Any

import cv2
import numpy as np
from PIL import Image, ImageOps, UnidentifiedImageError

from image_quality.schemas import QualityAnalysisResult

METRIC_SCHEMA_VERSION = "image-quality-metrics-v2"
REPORT_SCHEMA_VERSION = "image-quality-report-v2"
MAX_IMAGE_PIXELS = 50_000_000
MAX_ANALYSIS_DIMENSION = 1800
FOCUS_TARGET_SHORT_EDGE = 1600
FOCUS_GRID_ROWS = 12
FOCUS_GRID_COLUMNS = 8
MIN_EFFECTIVE_SHORT_EDGE = 1200
PREFERRED_EFFECTIVE_SHORT_EDGE = 2000

# Initial engineering thresholds. They are deliberately centralized and
# conservative until governed school answer-sheet benchmarks are available.
MAX_SKEW_ANALYSIS_DEGREES = 45.0
MIN_LINE_LENGTH_RATIO = 0.12
MIN_DESKEW_DEGREES = 0.5
MAX_DESKEW_DEGREES = 7.0
MIN_DESKEW_CONFIDENCE = 0.45
LANDSCAPE_ROTATION_RATIO = 1.05
SEVERE_BLUR_SHARPNESS_THRESHOLD = 0.12
PERSPECTIVE_SUSPECTED_SCORE = 0.15


class ImageQualityError(RuntimeError):
    pass


def analyze_and_normalize(data: bytes) -> QualityAnalysisResult:
    try:
        with Image.open(io.BytesIO(data)) as source:
            if (
                source.width <= 0
                or source.height <= 0
                or source.width * source.height > MAX_IMAGE_PIXELS
            ):
                raise ImageQualityError("image_dimensions_out_of_range")
            source.load()
            source_width, source_height = source.size
            source_format = source.format or "unknown"
            source_dpi = _source_dpi(source)
            exif_orientation = _exif_orientation(source)
            exif_rotation = _exif_rotation_degrees(exif_orientation)
            exif_matrix = _exif_transform_matrix(
                exif_orientation, source_width, source_height
            )
            normalized_image = ImageOps.exif_transpose(source).convert("RGB")
    except (UnidentifiedImageError, OSError, Image.DecompressionBombError) as exc:
        raise ImageQualityError("unreadable_image") from exc

    try:
        with normalized_image.convert("L") as grayscale_image:
            gray = np.asarray(grayscale_image, dtype=np.uint8)
            analysis_gray = _analysis_image(gray)
            skew_angle, skew_confidence = _detect_skew(analysis_gray)
            border_status, border_confidence, border_completeness, page_quad = (
                _detect_page_border(analysis_gray)
            )
            perspective_status, perspective_confidence, perspective_risk = (
                _detect_perspective_risk(
                    page_quad, border_confidence, analysis_gray.shape
                )
            )
            effective_short_edge = _effective_page_short_edge(
                page_quad, analysis_gray.shape, gray.shape
            )
            focus = _focus_report(gray)
            illumination = _illumination_report(gray)
            occlusion = _occlusion_report(analysis_gray)
            degradation = _noise_compression_report(gray, source_format)
            metrics = _metrics(
                gray,
                effective_short_edge=effective_short_edge,
                border_completeness=border_completeness,
                perspective_risk=perspective_risk,
                focus=focus,
                illumination=illumination,
                occlusion=occlusion,
                degradation=degradation,
            )

        deskew_applied = _should_apply_deskew(skew_angle, skew_confidence)
        content_rotation = skew_angle if deskew_applied else 0.0
        deskew_matrix = np.eye(3, dtype=np.float64)
        if deskew_applied:
            deskewed_image, deskew_matrix = _apply_deskew(
                normalized_image, content_rotation
            )
            normalized_image.close()
            normalized_image = deskewed_image

        pixel_width, pixel_height = normalized_image.size
        geometry = {
            "detected_skew_angle": round(skew_angle, 4),
            "skew_confidence": round(skew_confidence, 4),
            "skew_correction_applied": deskew_applied,
            "coarse_orientation": "landscape"
            if pixel_width > pixel_height * LANDSCAPE_ROTATION_RATIO
            else "portrait_or_square",
            "page_border_status": border_status,
            "page_border_confidence": round(border_confidence, 4),
            "perspective_status": perspective_status,
            "perspective_confidence": round(perspective_confidence, 4),
            "anchor_detection_status": "not_evaluated",
            "homography_rmse_px": None,
            "curvature_status": "not_evaluated",
        }
        dimensions = _dimensions(metrics, geometry)
        hard_gates = _hard_gates(metrics, geometry)
        issues = _issues(metrics, geometry, hard_gates)
        enhancements = ["limited_deskew"] if deskew_applied else []
        quality_score = _quality_score(metrics)
        decision, quality_status = _decision(
            quality_score, issues, hard_gates, enhancements
        )

        output = io.BytesIO()
        normalized_image.save(output, format="PNG")
    finally:
        normalized_image.close()

    report: dict[str, Any] = {
        "metric_schema_version": METRIC_SCHEMA_VERSION,
        "report_schema_version": REPORT_SCHEMA_VERSION,
        "pixel_width": pixel_width,
        "pixel_height": pixel_height,
        "source_dpi": source_dpi,
        "source_dpi_method": "embedded_metadata"
        if source_dpi is not None
        else "missing_metadata",
        "render_dpi": None,
        "source_format": source_format,
        "output_format": "image/png",
        "normalized_color_mode": "RGB",
        "metrics": metrics,
        "geometry": geometry,
        "focus": focus,
        "illumination": illumination,
        "occlusion_reflection": occlusion,
        "noise_compression": degradation,
        "dimensions": dimensions,
        "hard_gates": hard_gates,
        "quality_score": quality_score,
        "decision": decision,
        "enhancements_applied": enhancements,
        "roi_completeness": {
            "status": "unavailable",
            "required_roi_visible_ratio": None,
            "reason": "template_registration_not_available_at_quality_stage",
        },
        "predictor": {
            "kind": "weighted_rules",
            "version": "weighted-rules-v2",
            "calibration_status": "awaiting_real_answer_sheet_samples",
            "future_model": "xgboost_ocr_failure_probability",
        },
        "stages": {
            "fast_gate": {
                "status": "failed"
                if any(gate["status"] == "failed" for gate in hard_gates)
                else "passed",
                "checks": [
                    "decode",
                    "effective_resolution",
                    "severe_blur",
                    "blank_probability",
                ],
            },
            "page_registration": {"status": "external_stage"},
            "detailed_assessment": {"status": "completed"},
            "post_registration_roi_assessment": {
                "status": "awaiting_template_registration"
            },
        },
    }
    transform = {
        "source_pixel_width": source_width,
        "source_pixel_height": source_height,
        "normalized_pixel_width": pixel_width,
        "normalized_pixel_height": pixel_height,
        "exif_rotation_degrees": exif_rotation,
        "content_rotation_degrees": round(content_rotation, 4),
        "detected_skew_angle": round(skew_angle, 4),
        "skew_correction_applied": deskew_applied,
        "crop_box_source_pixels": [0, 0, source_width, source_height],
        "source_to_normalized_matrix": _matrix_list(deskew_matrix @ exif_matrix),
    }
    return QualityAnalysisResult(
        quality_status=quality_status,
        quality_report=report,
        quality_issues=issues,
        normalization_transform=transform,
        normalized_png=output.getvalue(),
    )


def _metrics(
    gray: np.ndarray,
    *,
    effective_short_edge: int,
    border_completeness: float,
    perspective_risk: float,
    focus: dict[str, Any],
    illumination: dict[str, float],
    occlusion: dict[str, float | str],
    degradation: dict[str, float | str],
) -> dict[str, float | int]:
    effective_resolution_score = _effective_resolution_score(effective_short_edge)
    brightness_score = _clamp(float(gray.mean()) / 255.0)
    ink_coverage = float(focus["ink_coverage"])
    blank_probability = _clamp(1.0 - ink_coverage / 0.012)
    return {
        "sharpness_score": round(float(focus["sharpness_score"]), 4),
        "focus_laplacian_mean": round(float(focus["laplacian_mean"]), 4),
        "focus_laplacian_median": round(float(focus["laplacian_median"]), 4),
        "focus_laplacian_p20": round(float(focus["laplacian_p20"]), 4),
        "focus_tenengrad_median": round(float(focus["tenengrad_median"]), 4),
        "focus_gradient_energy_median": round(
            float(focus["gradient_energy_median"]), 4
        ),
        "blur_pattern": str(focus["blur_pattern"]),
        "bad_focus_patch_ratio": round(float(focus["bad_patch_ratio"]), 4),
        "effective_short_edge_px": effective_short_edge,
        "estimated_a4_dpi": round(effective_short_edge / 8.27, 2),
        "effective_resolution_score": round(effective_resolution_score, 4),
        "brightness_score": round(brightness_score, 4),
        "exposure_quality_score": round(illumination["exposure_quality_score"], 4),
        "contrast_score": round(illumination["contrast_score"], 4),
        "low_contrast_area_ratio": round(illumination["low_contrast_area_ratio"], 4),
        "shadow_risk_score": round(illumination["shadow_risk_score"], 4),
        "glare_risk_score": round(illumination["glare_risk_score"], 4),
        "occlusion_risk_score": round(float(occlusion["risk_score"]), 4),
        "occlusion_confidence": round(float(occlusion["confidence"]), 4),
        "noise_risk_score": round(float(degradation["noise_risk_score"]), 4),
        "compression_risk_score": round(
            float(degradation["compression_risk_score"]), 4
        ),
        "blank_probability": round(blank_probability, 4),
        "perspective_risk_score": round(perspective_risk, 4),
        "border_completeness_score": round(border_completeness, 4),
    }


def _analysis_image(gray: np.ndarray) -> np.ndarray:
    height, width = gray.shape
    longest_side = max(height, width)
    if longest_side <= MAX_ANALYSIS_DIMENSION:
        return gray
    scale = MAX_ANALYSIS_DIMENSION / float(longest_side)
    return cv2.resize(
        gray,
        (max(1, round(width * scale)), max(1, round(height * scale))),
        interpolation=cv2.INTER_AREA,
    )


def _effective_page_short_edge(
    page_quad: np.ndarray | None,
    analysis_shape: tuple[int, int],
    source_shape: tuple[int, int],
) -> int:
    source_height, source_width = source_shape
    if page_quad is None:
        return min(source_width, source_height)
    analysis_height, analysis_width = analysis_shape
    scale_x = source_width / max(float(analysis_width), 1.0)
    scale_y = source_height / max(float(analysis_height), 1.0)
    scaled = page_quad.astype(np.float64) * np.asarray([scale_x, scale_y])
    top_left, top_right, bottom_right, bottom_left = scaled
    page_width = (
        float(np.linalg.norm(top_right - top_left))
        + float(np.linalg.norm(bottom_right - bottom_left))
    ) / 2.0
    page_height = (
        float(np.linalg.norm(bottom_left - top_left))
        + float(np.linalg.norm(bottom_right - top_right))
    ) / 2.0
    return max(1, round(min(page_width, page_height)))


def _issues(
    metrics: dict[str, float | int],
    geometry: dict[str, Any],
    hard_gates: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    issues: list[dict[str, Any]] = []
    failed_gate_codes = {
        str(gate["code"]) for gate in hard_gates if gate["status"] == "failed"
    }
    if "low_effective_resolution" in failed_gate_codes:
        issues.append(
            _issue(
                "low_effective_resolution",
                "failed",
                "effective_short_edge_px",
                metrics["effective_short_edge_px"],
                MIN_EFFECTIVE_SHORT_EDGE,
                "recapture",
            )
        )
    elif metrics["effective_short_edge_px"] < 1600:
        issues.append(
            _issue(
                "low_effective_resolution",
                "review",
                "effective_short_edge_px",
                metrics["effective_short_edge_px"],
                1600,
                "manual_review",
            )
        )
    if "severe_blur" in failed_gate_codes:
        issues.append(
            _issue(
                "low_sharpness",
                "failed",
                "sharpness_score",
                metrics["sharpness_score"],
                SEVERE_BLUR_SHARPNESS_THRESHOLD,
                "recapture",
                {"window": "local_focus_map", "blur_pattern": metrics["blur_pattern"]},
            )
        )
    elif metrics["sharpness_score"] < 0.42 or metrics["bad_focus_patch_ratio"] > 0.15:
        issues.append(
            _issue(
                "low_sharpness",
                "review",
                "bad_focus_patch_ratio",
                metrics["bad_focus_patch_ratio"],
                0.15,
                "manual_review",
                {"window": "local_focus_map", "blur_pattern": metrics["blur_pattern"]},
            )
        )
    if metrics["blank_probability"] > 0.97:
        issues.append(
            _issue(
                "blank_page",
                "review",
                "blank_probability",
                metrics["blank_probability"],
                0.97,
                "manual_review",
            )
        )
    if "incomplete_page" in failed_gate_codes:
        issues.append(
            _issue(
                "incomplete_page_border",
                "failed",
                "border_completeness_score",
                metrics["border_completeness_score"],
                0.75,
                "recapture",
            )
        )
    elif (
        geometry["page_border_confidence"] >= 0.5
        and metrics["border_completeness_score"] < 0.98
    ):
        issues.append(
            _issue(
                "incomplete_page_border",
                "review",
                "border_completeness_score",
                metrics["border_completeness_score"],
                0.98,
                "manual_review",
            )
        )
    if (
        geometry["perspective_confidence"] >= 0.4
        and metrics["perspective_risk_score"] > PERSPECTIVE_SUSPECTED_SCORE
    ):
        issues.append(
            _issue(
                "perspective_risk",
                "review",
                "perspective_risk_score",
                metrics["perspective_risk_score"],
                PERSPECTIVE_SUSPECTED_SCORE,
                "manual_review",
            )
        )
    if (
        abs(float(geometry["detected_skew_angle"])) > MAX_DESKEW_DEGREES
        and geometry["skew_confidence"] >= MIN_DESKEW_CONFIDENCE
    ):
        issues.append(
            _issue(
                "residual_skew",
                "review",
                "detected_skew_angle",
                geometry["detected_skew_angle"],
                MAX_DESKEW_DEGREES,
                "manual_review",
            )
        )
    if geometry["coarse_orientation"] == "landscape":
        issues.append(
            _issue(
                "large_rotation_risk",
                "review",
                "coarse_orientation",
                geometry["coarse_orientation"],
                "portrait_or_square",
                "manual_review",
            )
        )
    if metrics["exposure_quality_score"] < 0.45:
        issues.append(
            _issue(
                "bad_exposure",
                "review",
                "exposure_quality_score",
                metrics["exposure_quality_score"],
                0.45,
                "manual_review",
            )
        )
    if metrics["shadow_risk_score"] > 0.48:
        issues.append(
            _issue(
                "shadow_risk",
                "review",
                "shadow_risk_score",
                metrics["shadow_risk_score"],
                0.48,
                "manual_review",
            )
        )
    if metrics["low_contrast_area_ratio"] > 0.15:
        issues.append(
            _issue(
                "low_local_contrast",
                "review",
                "low_contrast_area_ratio",
                metrics["low_contrast_area_ratio"],
                0.15,
                "manual_review",
            )
        )
    if metrics["glare_risk_score"] > 0.4:
        issues.append(
            _issue(
                "glare_risk",
                "review",
                "glare_risk_score",
                metrics["glare_risk_score"],
                0.4,
                "manual_review",
            )
        )
    if (
        metrics["occlusion_confidence"] >= 0.6
        and metrics["occlusion_risk_score"] > 0.55
    ):
        issues.append(
            _issue(
                "occlusion_risk",
                "review",
                "occlusion_risk_score",
                metrics["occlusion_risk_score"],
                0.55,
                "manual_review",
            )
        )
    if (
        max(
            float(metrics["noise_risk_score"]), float(metrics["compression_risk_score"])
        )
        > 0.72
    ):
        issues.append(
            _issue(
                "noise_or_compression",
                "review",
                "noise_risk_score",
                metrics["noise_risk_score"],
                0.72,
                "manual_review",
            )
        )
    return issues


def _focus_report(gray: np.ndarray) -> dict[str, Any]:
    focus_gray = _focus_image(gray)
    height, width = focus_gray.shape
    rows = FOCUS_GRID_ROWS if height >= width else FOCUS_GRID_COLUMNS
    columns = FOCUS_GRID_COLUMNS if height >= width else FOCUS_GRID_ROWS
    patches: list[dict[str, Any]] = []
    all_ink = 0
    for row in range(rows):
        top, bottom = row * height // rows, (row + 1) * height // rows
        for column in range(columns):
            left, right = column * width // columns, (column + 1) * width // columns
            patch = focus_gray[top:bottom, left:right]
            if patch.size == 0:
                continue
            background = float(np.percentile(patch, 88))
            if background < 160.0:
                continue
            ink_mask = patch < min(215.0, background - 16.0)
            ink_ratio = float(ink_mask.mean())
            all_ink += int(ink_mask.sum())
            if ink_ratio < 0.0025:
                continue
            laplacian = cv2.Laplacian(patch, cv2.CV_64F, ksize=3)
            gx = cv2.Sobel(patch, cv2.CV_64F, 1, 0, ksize=3)
            gy = cv2.Sobel(patch, cv2.CV_64F, 0, 1, ksize=3)
            gradient_squared = gx * gx + gy * gy
            laplacian_variance = float(laplacian.var())
            tenengrad = float(np.sqrt(gradient_squared).mean())
            gradient_energy = float(gradient_squared.mean())
            horizontal_energy = float((gx * gx).mean())
            vertical_energy = float((gy * gy).mean())
            gradient_anisotropy = abs(horizontal_energy - vertical_energy) / max(
                horizontal_energy + vertical_energy, 1.0
            )
            focus_score = _clamp(
                0.5 * laplacian_variance / 260.0
                + 0.3 * tenengrad / 36.0
                + 0.2 * gradient_energy / 3600.0
            )
            patches.append(
                {
                    "row": row,
                    "column": column,
                    "x": round(left / width, 5),
                    "y": round(top / height, 5),
                    "width": round((right - left) / width, 5),
                    "height": round((bottom - top) / height, 5),
                    "ink_ratio": round(ink_ratio, 5),
                    "laplacian_variance": round(laplacian_variance, 4),
                    "tenengrad": round(tenengrad, 4),
                    "gradient_energy": round(gradient_energy, 4),
                    "gradient_anisotropy": round(gradient_anisotropy, 4),
                    "focus_score": round(focus_score, 4),
                    "status": "bad"
                    if focus_score < 0.35
                    else "warning"
                    if focus_score < 0.55
                    else "good",
                }
            )
    scores = np.asarray(
        [float(item["focus_score"]) for item in patches], dtype=np.float64
    )
    laplacians = np.asarray(
        [float(item["laplacian_variance"]) for item in patches], dtype=np.float64
    )
    tenengrads = np.asarray(
        [float(item["tenengrad"]) for item in patches], dtype=np.float64
    )
    energies = np.asarray(
        [float(item["gradient_energy"]) for item in patches], dtype=np.float64
    )
    anisotropies = np.asarray(
        [float(item["gradient_anisotropy"]) for item in patches], dtype=np.float64
    )
    if scores.size == 0:
        scores = laplacians = tenengrads = energies = anisotropies = np.asarray(
            [0.0], dtype=np.float64
        )
    bad_ratio = float(np.mean(scores < 0.35))
    sharpness_score = _clamp(
        0.55 * float(np.percentile(scores, 20)) + 0.45 * float(np.median(scores))
    )
    median_anisotropy = float(np.median(anisotropies))
    blur_pattern = (
        "directional_suspected"
        if sharpness_score < 0.45 and median_anisotropy > 0.55
        else "defocus_or_mixed"
        if sharpness_score < 0.45
        else "not_detected"
    )
    return {
        "grid": {"rows": rows, "columns": columns},
        "ink_patch_count": len(patches),
        "ink_coverage": round(all_ink / max(height * width, 1), 6),
        "focus_mean": round(float(scores.mean()), 4),
        "focus_median": round(float(np.median(scores)), 4),
        "focus_p20": round(float(np.percentile(scores, 20)), 4),
        "focus_min": round(float(scores.min()), 4),
        "laplacian_mean": round(float(laplacians.mean()), 4),
        "laplacian_median": round(float(np.median(laplacians)), 4),
        "laplacian_p20": round(float(np.percentile(laplacians, 20)), 4),
        "tenengrad_median": round(float(np.median(tenengrads)), 4),
        "gradient_energy_median": round(float(np.median(energies)), 4),
        "gradient_anisotropy_median": round(median_anisotropy, 4),
        "blur_pattern": blur_pattern,
        "bad_patch_ratio": round(bad_ratio, 4),
        "sharpness_score": round(sharpness_score, 4),
        "map": patches,
    }


def _focus_image(gray: np.ndarray) -> np.ndarray:
    height, width = gray.shape
    short_edge = min(height, width)
    if short_edge <= FOCUS_TARGET_SHORT_EDGE:
        return gray
    scale = FOCUS_TARGET_SHORT_EDGE / float(short_edge)
    return cv2.resize(
        gray,
        (max(1, round(width * scale)), max(1, round(height * scale))),
        interpolation=cv2.INTER_AREA,
    )


def _illumination_report(gray: np.ndarray) -> dict[str, float]:
    height, width = gray.shape
    rows, columns = 12, 8
    backgrounds: list[float] = []
    local_contrasts: list[float] = []
    for row in range(rows):
        top, bottom = row * height // rows, (row + 1) * height // rows
        for column in range(columns):
            left, right = column * width // columns, (column + 1) * width // columns
            patch = gray[top:bottom, left:right]
            if not patch.size:
                continue
            background = float(np.percentile(patch, 88))
            if background < 160.0:
                continue
            backgrounds.append(background)
            ink_values = patch[patch < min(215.0, background - 20.0)]
            if ink_values.size / patch.size >= 0.0025:
                foreground = float(np.percentile(ink_values, 50))
                local_contrasts.append(_clamp((background - foreground) / 140.0))
    background_values = np.asarray(
        backgrounds or [float(gray.mean())], dtype=np.float64
    )
    contrast_values = np.asarray(local_contrasts or [0.0], dtype=np.float64)
    shadow_risk = _detect_shadow_risk(gray)
    bright_blocks = float(np.mean(background_values >= 250.0))
    illumination_spread = float(
        np.percentile(background_values, 90) - np.percentile(background_values, 10)
    )
    glare_risk = (
        0.0
        if illumination_spread > 120.0
        else _clamp(bright_blocks * max(0.0, illumination_spread - 24.0) / 70.0)
    )
    likely_page_pixels = gray[gray > 90]
    exposure_pixels = (
        likely_page_pixels
        if likely_page_pixels.size >= gray.size * 0.3
        else gray.ravel()
    )
    dark_ratio = float(np.mean(exposure_pixels < 115))
    clipped_ratio = float(np.mean(exposure_pixels > 253))
    exposure_quality = _clamp(
        1.0
        - 2.5 * dark_ratio
        - 0.35 * max(0.0, clipped_ratio - 0.88)
        - 0.6 * shadow_risk
    )
    return {
        "background_p10": round(float(np.percentile(background_values, 10)), 4),
        "background_p90": round(float(np.percentile(background_values, 90)), 4),
        "illumination_spread": round(illumination_spread, 4),
        "shadow_risk_score": round(shadow_risk, 4),
        "glare_risk_score": round(glare_risk, 4),
        "contrast_score": round(float(np.median(contrast_values)), 4),
        "low_contrast_area_ratio": round(float(np.mean(contrast_values < 0.32)), 4),
        "exposure_quality_score": round(exposure_quality, 4),
    }


def _occlusion_report(gray: np.ndarray) -> dict[str, float | str]:
    height, width = gray.shape
    mask = (gray < 55).astype(np.uint8)
    mask[: max(2, height // 80), :] = 0
    mask[-max(2, height // 80) :, :] = 0
    mask[:, : max(2, width // 80)] = 0
    mask[:, -max(2, width // 80) :] = 0
    count, _, stats, _ = cv2.connectedComponentsWithStats(mask, 8)
    candidates: list[float] = []
    image_area = float(height * width)
    for index in range(1, count):
        component_width = float(stats[index, cv2.CC_STAT_WIDTH])
        component_height = float(stats[index, cv2.CC_STAT_HEIGHT])
        area_ratio = float(stats[index, cv2.CC_STAT_AREA]) / max(image_area, 1.0)
        aspect = max(component_width, component_height) / max(
            min(component_width, component_height), 1.0
        )
        if 0.003 <= area_ratio <= 0.25 and aspect <= 6.0:
            candidates.append(area_ratio)
    largest = max(candidates, default=0.0)
    risk = _clamp(largest / 0.06)
    confidence = _clamp(len(candidates) / 3.0 + largest / 0.12)
    return {
        "status": "suspected" if risk > 0.35 else "normal",
        "risk_score": round(risk, 4),
        "confidence": round(confidence, 4),
        "candidate_count": len(candidates),
        "largest_candidate_area_ratio": round(largest, 5),
    }


def _noise_compression_report(
    gray: np.ndarray, source_format: str
) -> dict[str, float | str]:
    analysis = _analysis_image(gray)
    smooth = cv2.GaussianBlur(analysis, (3, 3), 0)
    residual = cv2.absdiff(analysis, smooth).astype(np.float32)
    gx = cv2.Sobel(analysis, cv2.CV_32F, 1, 0, ksize=3)
    gy = cv2.Sobel(analysis, cv2.CV_32F, 0, 1, ksize=3)
    flat = np.sqrt(gx * gx + gy * gy) < 18.0
    noise_sigma = (
        float(np.median(residual[flat])) if np.any(flat) else float(np.median(residual))
    )
    noise_risk = _clamp((noise_sigma - 1.5) / 10.0)
    vertical_boundary = (
        float(np.mean(np.abs(np.diff(analysis.astype(np.float32), axis=1))[:, 7::8]))
        if analysis.shape[1] > 16
        else 0.0
    )
    horizontal_boundary = (
        float(np.mean(np.abs(np.diff(analysis.astype(np.float32), axis=0))[7::8, :]))
        if analysis.shape[0] > 16
        else 0.0
    )
    ordinary_vertical = (
        float(np.mean(np.abs(np.diff(analysis.astype(np.float32), axis=1))[:, 3::8]))
        if analysis.shape[1] > 16
        else 0.0
    )
    ordinary_horizontal = (
        float(np.mean(np.abs(np.diff(analysis.astype(np.float32), axis=0))[3::8, :]))
        if analysis.shape[0] > 16
        else 0.0
    )
    boundary = (vertical_boundary + horizontal_boundary) / 2.0
    ordinary = (ordinary_vertical + ordinary_horizontal) / 2.0
    blockiness = _clamp((boundary - ordinary - 0.5) / 8.0)
    return {
        "status": "suspected" if max(noise_risk, blockiness) > 0.55 else "normal",
        "source_format": source_format.lower(),
        "estimated_noise_sigma": round(noise_sigma, 4),
        "noise_risk_score": round(noise_risk, 4),
        "jpeg_blockiness": round(blockiness, 4),
        "compression_risk_score": round(blockiness, 4),
        "moire_status": "not_evaluated",
    }


def _dimensions(
    metrics: dict[str, float | int], geometry: dict[str, Any]
) -> dict[str, dict[str, Any]]:
    illumination_score = _clamp(
        0.55 * float(metrics["exposure_quality_score"])
        + 0.45 * (1.0 - float(metrics["shadow_risk_score"]))
    )
    contrast_score = float(metrics["contrast_score"])
    scores = {
        "completeness": float(metrics["border_completeness_score"]),
        "effective_resolution": float(metrics["effective_resolution_score"]),
        "sharpness": float(metrics["sharpness_score"]),
        "geometry": _clamp(
            1.0
            - 0.65 * float(metrics["perspective_risk_score"])
            - 0.35 * min(abs(float(geometry["detected_skew_angle"])) / 15.0, 1.0)
        ),
        "illumination_contrast": _clamp(
            0.5 * illumination_score + 0.5 * contrast_score
        ),
        "occlusion_reflection": _clamp(
            1.0
            - max(
                float(metrics["occlusion_risk_score"])
                * float(metrics["occlusion_confidence"]),
                float(metrics["glare_risk_score"]),
            )
        ),
        "noise_compression": _clamp(
            1.0
            - max(
                float(metrics["noise_risk_score"]),
                float(metrics["compression_risk_score"]),
            )
        ),
    }
    return {
        key: {"score": round(value * 100.0, 1), "status": _score_status(value)}
        for key, value in scores.items()
    }


def _hard_gates(
    metrics: dict[str, float | int], geometry: dict[str, Any]
) -> list[dict[str, Any]]:
    severe_blur = (
        float(metrics["sharpness_score"]) < SEVERE_BLUR_SHARPNESS_THRESHOLD
        and float(metrics["focus_laplacian_median"]) < 30.0
    ) or (
        float(metrics["bad_focus_patch_ratio"]) > 0.5
        and float(metrics["focus_tenengrad_median"]) < 8.0
    )
    incomplete = (
        geometry["page_border_confidence"] >= 0.7
        and float(metrics["border_completeness_score"]) < 0.75
    )
    return [
        _gate("page_decode", "passed", True, True),
        _gate(
            "low_effective_resolution",
            "failed"
            if int(metrics["effective_short_edge_px"]) < MIN_EFFECTIVE_SHORT_EDGE
            else "passed",
            metrics["effective_short_edge_px"],
            MIN_EFFECTIVE_SHORT_EDGE,
        ),
        _gate(
            "severe_blur",
            "failed" if severe_blur else "passed",
            metrics["sharpness_score"],
            SEVERE_BLUR_SHARPNESS_THRESHOLD,
        ),
        _gate(
            "incomplete_page",
            "failed" if incomplete else "passed",
            metrics["border_completeness_score"],
            0.75,
        ),
        _gate(
            "required_roi_completeness",
            "not_evaluated",
            None,
            0.98,
            {"reason": "template_registration_not_available"},
        ),
        _gate(
            "critical_roi_occlusion",
            "not_evaluated",
            None,
            0.0,
            {"reason": "template_registration_not_available"},
        ),
    ]


def _quality_score(metrics: dict[str, float | int]) -> float:
    illumination = _clamp(
        0.55 * float(metrics["exposure_quality_score"])
        + 0.45 * (1.0 - float(metrics["shadow_risk_score"]))
    )
    geometry = _clamp(1.0 - float(metrics["perspective_risk_score"]))
    occlusion = _clamp(
        1.0
        - max(
            float(metrics["occlusion_risk_score"])
            * float(metrics["occlusion_confidence"]),
            float(metrics["glare_risk_score"]),
        )
    )
    noise = _clamp(
        1.0
        - max(
            float(metrics["noise_risk_score"]), float(metrics["compression_risk_score"])
        )
    )
    score = 100.0 * (
        0.25 * float(metrics["sharpness_score"])
        + 0.15 * float(metrics["effective_resolution_score"])
        + 0.15 * geometry
        + 0.15 * illumination
        + 0.15 * float(metrics["contrast_score"])
        + 0.10 * occlusion
        + 0.05 * noise
    )
    return round(score, 1)


def _decision(
    score: float,
    issues: list[dict[str, Any]],
    hard_gates: list[dict[str, Any]],
    enhancements: list[str],
) -> tuple[str, str]:
    if any(gate["status"] == "failed" for gate in hard_gates):
        return "REJECT", "failed"
    if issues:
        return ("REJECT", "failed") if score < 50.0 else ("LOW_QUALITY", "review")
    if enhancements:
        return "PASS_WITH_ENHANCEMENT", "passed"
    return ("PASS", "passed") if score >= 78.0 else ("LOW_QUALITY", "review")


def _issue(
    code: str,
    severity: str,
    metric: str,
    observed: Any,
    threshold: Any,
    action: str,
    parameters: dict[str, Any] | None = None,
) -> dict[str, Any]:
    return {
        "code": code,
        "severity": severity,
        "metric": metric,
        "observed": observed,
        "threshold": threshold,
        "rule_id": f"{code}-v2",
        "action": action,
        "parameters": parameters or {},
    }


def _gate(
    code: str,
    status: str,
    observed: Any,
    threshold: Any,
    parameters: dict[str, Any] | None = None,
) -> dict[str, Any]:
    return {
        "code": code,
        "status": status,
        "observed": observed,
        "threshold": threshold,
        "parameters": parameters or {},
    }


def _score_status(value: float) -> str:
    if value >= 0.78:
        return "passed"
    if value >= 0.5:
        return "review"
    return "failed"


def _effective_resolution_score(short_edge: int) -> float:
    if short_edge < MIN_EFFECTIVE_SHORT_EDGE:
        return _clamp(0.35 * short_edge / MIN_EFFECTIVE_SHORT_EDGE)
    if short_edge < 1600:
        return 0.45 + 0.25 * (short_edge - MIN_EFFECTIVE_SHORT_EDGE) / 400.0
    if short_edge < PREFERRED_EFFECTIVE_SHORT_EDGE:
        return 0.75 + 0.25 * (short_edge - 1600) / 400.0
    return 1.0


def _detect_skew(gray: np.ndarray) -> tuple[float, float]:
    """Return line tilt in raster coordinates and heuristic evidence strength.

    Positive raster angles slope downward to the right. OpenCV's affine
    rotation with the same signed angle counter-rotates that tilt.
    """
    _, width = gray.shape
    blurred = cv2.GaussianBlur(gray, (5, 5), 0)
    edges = cv2.Canny(blurred, 50, 150)
    minimum_length = max(40, int(width * MIN_LINE_LENGTH_RATIO))
    lines = cv2.HoughLinesP(
        edges,
        1,
        np.pi / 1800.0,
        threshold=max(30, int(width * 0.05)),
        minLineLength=minimum_length,
        maxLineGap=max(8, int(width * 0.02)),
    )
    if lines is None:
        return 0.0, 0.0

    angles: list[float] = []
    weights: list[float] = []
    for x1, y1, x2, y2 in lines.reshape(-1, 4):
        dx, dy = float(x2 - x1), float(y2 - y1)
        length = float(np.hypot(dx, dy))
        if length < minimum_length:
            continue
        angle = float(np.degrees(np.arctan2(dy, dx)))
        # Treat horizontal and vertical document structure as evidence for the
        # same page-axis rotation. This keeps large rotations up to 45 degrees visible
        # instead of discarding them before the quality decision.
        angle = ((angle + 45.0) % 90.0) - 45.0
        if abs(angle) > MAX_SKEW_ANALYSIS_DEGREES:
            continue
        angles.append(angle)
        # Capping prevents one long page border from deciding the result.
        weights.append(min(length, width * 0.45))

    if len(angles) < 2:
        return 0.0, min(0.12, len(angles) * 0.06)
    values = np.asarray(angles, dtype=np.float64)
    line_weights = np.asarray(weights, dtype=np.float64)
    angle = _weighted_median(values, line_weights)
    deviations = np.abs(values - angle)
    mad = _weighted_median(deviations, line_weights)
    consistency = _clamp(1.0 - mad / 3.0)
    count_support = min(1.0, len(values) / 8.0)
    length_support = min(1.0, float(line_weights.sum()) / max(width * 2.0, 1.0))
    # This is heuristic evidence strength, not a calibrated probability.
    confidence = _clamp(consistency * (0.55 * count_support + 0.45 * length_support))
    if confidence < 0.15:
        return 0.0, confidence
    return float(angle), confidence


def _should_apply_deskew(angle: float, confidence: float) -> bool:
    return (
        np.isfinite(angle)
        and np.isfinite(confidence)
        and MIN_DESKEW_DEGREES <= abs(angle) <= MAX_DESKEW_DEGREES
        and confidence >= MIN_DESKEW_CONFIDENCE
    )


def _apply_deskew(
    image: Image.Image, rotation_degrees: float
) -> tuple[Image.Image, np.ndarray]:
    width, height = image.size
    center = ((width - 1) / 2.0, (height - 1) / 2.0)
    affine = cv2.getRotationMatrix2D(center, rotation_degrees, 1.0).astype(np.float64)
    corners = np.array(
        [
            [[0.0, 0.0]],
            [[width - 1.0, 0.0]],
            [[width - 1.0, height - 1.0]],
            [[0.0, height - 1.0]],
        ],
        dtype=np.float64,
    )
    transformed = cv2.transform(corners, affine).reshape(-1, 2)
    minimum = transformed.min(axis=0)
    maximum = transformed.max(axis=0)
    affine[:, 2] -= minimum
    output_width = int(np.ceil(maximum[0] - minimum[0])) + 1
    output_height = int(np.ceil(maximum[1] - minimum[1])) + 1

    rgb = np.asarray(image, dtype=np.uint8)
    rotated = cv2.warpAffine(
        rgb,
        affine,
        (output_width, output_height),
        flags=cv2.INTER_LINEAR,
        borderMode=cv2.BORDER_CONSTANT,
        borderValue=(255, 255, 255),
    )
    matrix = np.eye(3, dtype=np.float64)
    matrix[:2, :] = affine
    return Image.fromarray(rotated), matrix


def _detect_page_border(
    gray: np.ndarray,
) -> tuple[str, float, float, np.ndarray | None]:
    height, width = gray.shape
    image_area = float(height * width)
    blurred = cv2.GaussianBlur(gray, (5, 5), 0)
    edges = cv2.Canny(blurred, 45, 140)
    closed = cv2.morphologyEx(
        edges, cv2.MORPH_CLOSE, np.ones((5, 5), np.uint8), iterations=2
    )
    contours, _ = cv2.findContours(closed, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

    best_quad: np.ndarray | None = None
    best_score = 0.0
    for contour in contours:
        perimeter = float(cv2.arcLength(contour, True))
        if perimeter < 0.8 * (width + height):
            continue
        approximation = cv2.approxPolyDP(contour, 0.02 * perimeter, True)
        if len(approximation) != 4 or not cv2.isContourConvex(approximation):
            continue
        quad = approximation.reshape(4, 2).astype(np.float32)
        area_ratio = abs(float(cv2.contourArea(quad))) / max(image_area, 1.0)
        if area_ratio < 0.45 or area_ratio > 1.01:
            continue
        center = quad.mean(axis=0)
        center_offset = float(
            np.linalg.norm(center - np.array([width / 2.0, height / 2.0]))
        )
        center_score = _clamp(
            1.0 - center_offset / max(np.hypot(width, height) * 0.3, 1.0)
        )
        candidate_score = 0.85 * area_ratio + 0.15 * center_score
        if candidate_score > best_score:
            best_quad, best_score = quad, candidate_score

    if best_quad is not None:
        area_ratio = abs(float(cv2.contourArea(best_quad))) / max(image_area, 1.0)
        completeness = _clamp(area_ratio / 0.78)
        confidence = _clamp(0.55 + 0.45 * min(area_ratio / 0.8, 1.0))
        status = "complete" if completeness >= 0.8 else "partial"
        return status, confidence, completeness, _order_quad(best_quad)

    side_support = _border_side_support(edges)
    completeness = float(np.mean(side_support))
    detected_sides = sum(value >= 0.35 for value in side_support)
    if detected_sides:
        confidence = _clamp(0.2 + 0.15 * detected_sides + 0.2 * completeness)
        status = "complete" if completeness >= 0.8 else "partial"
        return status, confidence, completeness, None

    # A flatbed scan may contain paper to every canvas edge and therefore no
    # visible outer contour. Report moderate completeness with low confidence
    # instead of inventing a reliable border observation.
    edge_band = max(2, int(min(width, height) * 0.015))
    edge_pixels = np.concatenate(
        (
            gray[:edge_band, :].ravel(),
            gray[-edge_band:, :].ravel(),
            gray[:, :edge_band].ravel(),
            gray[:, -edge_band:].ravel(),
        )
    )
    if float(np.percentile(edge_pixels, 25)) >= 210.0:
        return "unknown", 0.25, 0.75, None
    return "unknown", 0.0, 0.0, None


def _border_side_support(edges: np.ndarray) -> list[float]:
    height, width = edges.shape
    lines = cv2.HoughLinesP(
        edges,
        1,
        np.pi / 720.0,
        threshold=max(25, int(min(width, height) * 0.04)),
        minLineLength=max(30, int(min(width, height) * 0.12)),
        maxLineGap=max(8, int(min(width, height) * 0.025)),
    )
    support = [0.0, 0.0, 0.0, 0.0]  # top, right, bottom, left
    if lines is None:
        return support
    for x1, y1, x2, y2 in lines.reshape(-1, 4):
        dx, dy = abs(float(x2 - x1)), abs(float(y2 - y1))
        if dx >= 2.5 * max(dy, 1.0):
            y = (y1 + y2) / 2.0
            coverage = _clamp(dx / max(width, 1))
            if y <= height * 0.2:
                support[0] = max(support[0], coverage)
            if y >= height * 0.8:
                support[2] = max(support[2], coverage)
        elif dy >= 2.5 * max(dx, 1.0):
            x = (x1 + x2) / 2.0
            coverage = _clamp(dy / max(height, 1))
            if x >= width * 0.8:
                support[1] = max(support[1], coverage)
            if x <= width * 0.2:
                support[3] = max(support[3], coverage)
    return support


def _detect_perspective_risk(
    quad: np.ndarray | None, border_confidence: float, shape: tuple[int, int]
) -> tuple[str, float, float]:
    if quad is None:
        return "unknown", 0.0, 0.0
    top_left, top_right, bottom_right, bottom_left = quad
    top = float(np.linalg.norm(top_right - top_left))
    bottom = float(np.linalg.norm(bottom_right - bottom_left))
    left = float(np.linalg.norm(bottom_left - top_left))
    right = float(np.linalg.norm(bottom_right - top_right))
    if min(top, bottom, left, right) <= 1.0:
        return "unknown", 0.0, 0.0

    horizontal_difference = abs(top - bottom) / max(top, bottom)
    vertical_difference = abs(left - right) / max(left, right)
    angle_deviations = []
    for previous, current, following in (
        (bottom_left, top_left, top_right),
        (top_left, top_right, bottom_right),
        (top_right, bottom_right, bottom_left),
        (bottom_right, bottom_left, top_left),
    ):
        first, second = previous - current, following - current
        cosine = abs(float(np.dot(first, second))) / max(
            float(np.linalg.norm(first) * np.linalg.norm(second)), 1.0
        )
        angle_deviations.append(_clamp(cosine))
    angle_risk = float(np.mean(angle_deviations))
    risk = _clamp(
        0.35 * horizontal_difference + 0.35 * vertical_difference + 0.30 * angle_risk
    )
    height, width = shape
    area_ratio = abs(float(cv2.contourArea(quad))) / max(float(height * width), 1.0)
    confidence = _clamp(border_confidence * min(area_ratio / 0.6, 1.0))
    status = "suspected" if risk > PERSPECTIVE_SUSPECTED_SCORE else "normal"
    return status, confidence, risk


def _detect_shadow_risk(gray: np.ndarray) -> float:
    height, width = gray.shape
    rows, columns = 8, 8
    illumination = np.empty((rows, columns), dtype=np.float32)
    for row in range(rows):
        top, bottom = row * height // rows, (row + 1) * height // rows
        for column in range(columns):
            left, right = column * width // columns, (column + 1) * width // columns
            block = gray[top:bottom, left:right]
            illumination[row, column] = (
                float(np.percentile(block, 80)) if block.size else 255.0
            )
    illumination = cv2.GaussianBlur(illumination, (3, 3), 0)
    high = float(np.percentile(illumination, 90))
    likely_page = illumination[illumination >= max(100.0, high * 0.45)]
    if likely_page.size < 4:
        likely_page = illumination.ravel()
    low = float(np.percentile(likely_page, 10))
    high = float(np.percentile(likely_page, 90))
    spread = float(high - low)
    variation = float(likely_page.std())
    return _clamp(0.65 * spread / 90.0 + 0.35 * variation / 45.0)


def _order_quad(quad: np.ndarray) -> np.ndarray:
    points = quad.astype(np.float32)
    sums = points.sum(axis=1)
    differences = np.diff(points, axis=1).ravel()
    return np.array(
        [
            points[np.argmin(sums)],
            points[np.argmin(differences)],
            points[np.argmax(sums)],
            points[np.argmax(differences)],
        ],
        dtype=np.float32,
    )


def _weighted_median(values: np.ndarray, weights: np.ndarray) -> float:
    order = np.argsort(values)
    sorted_values = values[order]
    sorted_weights = weights[order]
    midpoint = float(sorted_weights.sum()) / 2.0
    index = int(np.searchsorted(np.cumsum(sorted_weights), midpoint, side="left"))
    return float(sorted_values[min(index, len(sorted_values) - 1)])


def _exif_orientation(image: Image.Image) -> int:
    orientation = int(image.getexif().get(274, 1))
    return orientation if 1 <= orientation <= 8 else 1


def _source_dpi(image: Image.Image) -> float | None:
    raw = image.info.get("dpi")
    if not isinstance(raw, (tuple, list)) or len(raw) < 2:
        return None
    try:
        horizontal, vertical = float(raw[0]), float(raw[1])
    except (TypeError, ValueError):
        return None
    if (
        not np.isfinite(horizontal)
        or not np.isfinite(vertical)
        or min(horizontal, vertical) <= 0
    ):
        return None
    return round((horizontal + vertical) / 2.0, 2)


def _exif_rotation_degrees(orientation: int) -> int:
    return {
        3: 180,
        4: 180,
        5: 270,
        6: 90,
        7: 90,
        8: 270,
    }.get(orientation, 0)


def _exif_transform_matrix(orientation: int, width: int, height: int) -> np.ndarray:
    matrices = {
        1: [[1.0, 0.0, 0.0], [0.0, 1.0, 0.0], [0.0, 0.0, 1.0]],
        2: [[-1.0, 0.0, width - 1.0], [0.0, 1.0, 0.0], [0.0, 0.0, 1.0]],
        3: [[-1.0, 0.0, width - 1.0], [0.0, -1.0, height - 1.0], [0.0, 0.0, 1.0]],
        4: [[1.0, 0.0, 0.0], [0.0, -1.0, height - 1.0], [0.0, 0.0, 1.0]],
        5: [[0.0, 1.0, 0.0], [1.0, 0.0, 0.0], [0.0, 0.0, 1.0]],
        6: [[0.0, -1.0, height - 1.0], [1.0, 0.0, 0.0], [0.0, 0.0, 1.0]],
        7: [[0.0, -1.0, height - 1.0], [-1.0, 0.0, width - 1.0], [0.0, 0.0, 1.0]],
        8: [[0.0, 1.0, 0.0], [-1.0, 0.0, width - 1.0], [0.0, 0.0, 1.0]],
    }
    return np.asarray(matrices[orientation], dtype=np.float64)


def _matrix_list(matrix: np.ndarray) -> list[list[float]]:
    if matrix.shape != (3, 3) or not np.isfinite(matrix).all():
        raise ImageQualityError("invalid_normalization_transform")
    return [[round(float(value), 8) for value in row] for row in matrix]


def _clamp(value: float) -> float:
    return max(0.0, min(1.0, value))

from __future__ import annotations

from dataclasses import dataclass

import cv2
import numpy as np

from page_processing.decoder import DecodeError, decode_document


class RegistrationError(RuntimeError):
    pass


@dataclass(frozen=True)
class RegistrationEvidence:
    method: str
    confidence: float
    source_to_template: list[list[float]]
    template_to_source: list[list[float]]
    feature_count: int
    match_count: int
    inlier_count: int
    inlier_ratio: float
    reprojection_error: float
    coverage: float


@dataclass(frozen=True)
class RegistrationOutput:
    registered_png: bytes
    width: int
    height: int
    evidence: RegistrationEvidence


def register_page(
    source: bytes,
    source_content_type: str,
    template: bytes,
    template_content_type: str,
    *,
    render_dpi: int = 300,
    minimum_matches: int = 12,
    template_page_index: int = 1,
) -> RegistrationOutput:
    source_image = _decode_first(source, source_content_type, render_dpi)
    template_image = _decode_page(template, template_content_type, render_dpi, template_page_index)
    height, width = template_image.shape[:2]

    if source_image.shape == template_image.shape:
        difference = float(np.mean(cv2.absdiff(source_image, template_image)))
        if difference <= 1.0:
            matrix = np.eye(3, dtype=np.float64)
            return RegistrationOutput(
                registered_png=_encode_png(source_image),
                width=width,
                height=height,
                evidence=RegistrationEvidence(
                    method="pixel_identity",
                    confidence=1.0,
                    source_to_template=matrix.tolist(),
                    template_to_source=matrix.tolist(),
                    feature_count=0,
                    match_count=0,
                    inlier_count=0,
                    inlier_ratio=1.0,
                    reprojection_error=0.0,
                    coverage=1.0,
                ),
            )

    source_gray = cv2.cvtColor(source_image, cv2.COLOR_RGB2GRAY)
    template_gray = cv2.cvtColor(template_image, cv2.COLOR_RGB2GRAY)
    result = _feature_homography(source_gray, template_gray, "orb", minimum_matches)
    if result is None:
        result = _feature_homography(source_gray, template_gray, "akaze", minimum_matches)
    if result is None:
        raise RegistrationError("registration_insufficient_matches")

    matrix, method, feature_count, match_count, mask, reprojection_error = result
    inlier_count = int(mask.sum())
    inlier_ratio = inlier_count / max(match_count, 1)
    coverage = _coverage(matrix, source_image.shape[1], source_image.shape[0], width, height)
    confidence = float(np.clip(0.55 * inlier_ratio + 0.30 * min(coverage, 1.0) + 0.15 * max(0.0, 1.0 - reprojection_error / 8.0), 0.0, 1.0))
    if inlier_count < 8 or inlier_ratio < 0.35 or reprojection_error > 8.0 or coverage < 0.55:
        raise RegistrationError("registration_low_confidence")

    registered = cv2.warpPerspective(source_image, matrix, (width, height), flags=cv2.INTER_CUBIC, borderMode=cv2.BORDER_CONSTANT, borderValue=(255, 255, 255))
    inverse = np.linalg.inv(matrix)
    return RegistrationOutput(
        registered_png=_encode_png(registered),
        width=width,
        height=height,
        evidence=RegistrationEvidence(
            method=method,
            confidence=confidence,
            source_to_template=matrix.tolist(),
            template_to_source=inverse.tolist(),
            feature_count=feature_count,
            match_count=match_count,
            inlier_count=inlier_count,
            inlier_ratio=inlier_ratio,
            reprojection_error=reprojection_error,
            coverage=coverage,
        ),
    )


def register_page_manual(
    source: bytes,
    source_content_type: str,
    template: bytes,
    template_content_type: str,
    source_points: list[dict],
    template_points: list[dict],
    *,
    render_dpi: int = 300,
    template_page_index: int = 1,
) -> RegistrationOutput:
    source_image = _decode_first(source, source_content_type, render_dpi)
    template_image = _decode_page(template, template_content_type, render_dpi, template_page_index)
    source_normalized = _validated_quad(source_points)
    template_normalized = _validated_quad(template_points)
    if np.sign(cv2.contourArea(source_normalized, oriented=True)) != np.sign(cv2.contourArea(template_normalized, oriented=True)):
        raise RegistrationError("manual_registration_mirrored")
    source_height, source_width = source_image.shape[:2]
    target_height, target_width = template_image.shape[:2]
    source_pixels = source_normalized * np.float32([source_width - 1, source_height - 1])
    template_pixels = template_normalized * np.float32([target_width - 1, target_height - 1])
    matrix = cv2.getPerspectiveTransform(source_pixels.astype(np.float32), template_pixels.astype(np.float32))
    if not np.isfinite(matrix).all() or abs(float(np.linalg.det(matrix))) < 1e-10:
        raise RegistrationError("manual_registration_matrix_invalid")
    condition = float(np.linalg.cond(matrix))
    if not np.isfinite(condition) or condition > 1e8:
        raise RegistrationError("manual_registration_matrix_unstable")
    coverage = _coverage(matrix, source_width, source_height, target_width, target_height)
    if coverage < 0.70 or coverage > 1.15:
        raise RegistrationError("manual_registration_coverage_invalid")
    registered = cv2.warpPerspective(source_image, matrix, (target_width, target_height), flags=cv2.INTER_CUBIC, borderMode=cv2.BORDER_CONSTANT, borderValue=(255, 255, 255))
    inverse = np.linalg.inv(matrix)
    return RegistrationOutput(
        registered_png=_encode_png(registered), width=target_width, height=target_height,
        evidence=RegistrationEvidence(
            method="manual_four_point", confidence=1.0,
            source_to_template=matrix.tolist(), template_to_source=inverse.tolist(),
            feature_count=0, match_count=4, inlier_count=4, inlier_ratio=1.0,
            reprojection_error=0.0, coverage=coverage,
        ),
    )


def _validated_quad(points: list[dict]) -> np.ndarray:
    if len(points) != 4:
        raise RegistrationError("manual_registration_points_invalid")
    try:
        quad = np.float32([[float(point["x"]), float(point["y"])] for point in points])
    except (KeyError, TypeError, ValueError) as exc:
        raise RegistrationError("manual_registration_points_invalid") from exc
    if not np.isfinite(quad).all() or np.any(quad < 0) or np.any(quad > 1):
        raise RegistrationError("manual_registration_points_out_of_range")
    if len({(float(point[0]), float(point[1])) for point in quad}) != 4:
        raise RegistrationError("manual_registration_points_duplicate")
    if not cv2.isContourConvex(quad.reshape((-1, 1, 2))):
        raise RegistrationError("manual_registration_points_crossed")
    if abs(float(cv2.contourArea(quad))) < 0.05:
        raise RegistrationError("manual_registration_area_too_small")
    return quad


def crop_regions(registered_png: bytes, regions: list[dict]) -> list[dict]:
    image = _decode_png(registered_png)
    height, width = image.shape[:2]
    outputs: list[dict] = []
    for region in regions:
        try:
            x = float(region["x"])
            y = float(region["y"])
            w = float(region["width"])
            h = float(region["height"])
            question_id = str(region["question_id"])
        except (KeyError, TypeError, ValueError) as exc:
            raise RegistrationError("invalid_template_region") from exc
        if not question_id or x < 0 or y < 0 or w <= 0 or h <= 0 or x + w > 1 or y + h > 1:
            raise RegistrationError("invalid_template_region")
        left, top = int(round(x * width)), int(round(y * height))
        right, bottom = int(round((x + w) * width)), int(round((y + h) * height))
        left, top = max(0, left), max(0, top)
        right, bottom = min(width, right), min(height, bottom)
        if right <= left or bottom <= top:
            raise RegistrationError("empty_template_region")
        outputs.append(
            {
                "question_id": question_id,
                "label": str(region.get("label") or ""),
                "normalized_bbox": {"x": x, "y": y, "width": w, "height": h},
                "pixel_bbox": {"x": left, "y": top, "width": right - left, "height": bottom - top},
                "png": _encode_png(image[top:bottom, left:right]),
            }
        )
    return outputs


def _feature_homography(source: np.ndarray, template: np.ndarray, method: str, minimum_matches: int):
    detector = cv2.ORB_create(nfeatures=5000, fastThreshold=7) if method == "orb" else cv2.AKAZE_create()
    source_points, source_descriptors = detector.detectAndCompute(source, None)
    template_points, template_descriptors = detector.detectAndCompute(template, None)
    if source_descriptors is None or template_descriptors is None:
        return None
    matcher = cv2.BFMatcher(cv2.NORM_HAMMING)
    good = []
    for pair in matcher.knnMatch(source_descriptors, template_descriptors, k=2):
        if len(pair) == 2 and pair[0].distance < 0.75 * pair[1].distance:
            good.append(pair[0])
    if len(good) < minimum_matches:
        return None
    src = np.float32([source_points[item.queryIdx].pt for item in good]).reshape(-1, 1, 2)
    dst = np.float32([template_points[item.trainIdx].pt for item in good]).reshape(-1, 1, 2)
    matrix, mask = cv2.findHomography(src, dst, cv2.RANSAC, 4.0)
    if matrix is None or mask is None or not np.isfinite(matrix).all():
        return None
    mask = mask.ravel().astype(bool)
    projected = cv2.perspectiveTransform(src, matrix)
    errors = np.linalg.norm(projected.reshape(-1, 2) - dst.reshape(-1, 2), axis=1)
    reprojection = float(errors[mask].mean()) if mask.any() else float("inf")
    return matrix, method, len(source_points), len(good), mask, reprojection


def _coverage(matrix: np.ndarray, source_width: int, source_height: int, target_width: int, target_height: int) -> float:
    corners = np.float32([[[0, 0], [source_width, 0], [source_width, source_height], [0, source_height]]])
    transformed = cv2.perspectiveTransform(corners, matrix)[0]
    area = abs(float(cv2.contourArea(transformed)))
    return float(np.clip(area / max(target_width * target_height, 1), 0.0, 1.25))


def _decode_first(data: bytes, content_type: str, render_dpi: int) -> np.ndarray:
    return _decode_page(data, content_type, render_dpi, 1)


def _decode_page(data: bytes, content_type: str, render_dpi: int, page_index: int) -> np.ndarray:
    if page_index < 1 or page_index > 100:
        raise RegistrationError("invalid_template_page")
    try:
        pages, _ = decode_document(data, content_type, render_dpi=render_dpi, max_pages=100, max_page_pixels=60_000_000, max_total_pixels=600_000_000)
    except DecodeError as exc:
        raise RegistrationError(str(exc)) from exc
    if page_index > len(pages):
        raise RegistrationError("template_page_missing")
    return _decode_png(pages[page_index - 1].png)


def _decode_png(data: bytes) -> np.ndarray:
    encoded = np.frombuffer(data, np.uint8)
    image = cv2.imdecode(encoded, cv2.IMREAD_COLOR)
    if image is None:
        raise RegistrationError("image_decode_failed")
    return cv2.cvtColor(image, cv2.COLOR_BGR2RGB)


def _encode_png(image: np.ndarray) -> bytes:
    bgr = cv2.cvtColor(image, cv2.COLOR_RGB2BGR)
    ok, encoded = cv2.imencode(".png", bgr, [cv2.IMWRITE_PNG_COMPRESSION, 6])
    if not ok:
        raise RegistrationError("image_encode_failed")
    return encoded.tobytes()

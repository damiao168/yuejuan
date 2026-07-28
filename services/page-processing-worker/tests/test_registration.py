import cv2
import numpy as np
import pytest
from page_processing.registration import (
    RegistrationError,
    crop_regions,
    register_page,
    register_page_manual,
)


def _png(image: np.ndarray) -> bytes:
    ok, encoded = cv2.imencode(".png", image)
    assert ok
    return encoded.tobytes()


def test_registers_perspective_page_and_crops_real_region() -> None:
    template = np.full((900, 700, 3), 255, np.uint8)
    for y in range(80, 850, 80):
        cv2.line(template, (40, y), (660, y), (0, 0, 0), 2)
    cv2.putText(template, "EDUGRADE Q1 2026", (70, 145), cv2.FONT_HERSHEY_SIMPLEX, 1.2, (0, 0, 0), 3)
    cv2.rectangle(template, (100, 250), (600, 700), (0, 0, 0), 4)
    source_corners = np.float32([[0, 0], [700, 0], [700, 900], [0, 900]])
    skewed_corners = np.float32([[35, 20], [670, 45], [690, 860], [20, 885]])
    skew = cv2.getPerspectiveTransform(source_corners, skewed_corners)
    source = cv2.warpPerspective(template, skew, (700, 900), borderValue=(255, 255, 255))

    result = register_page(_png(source), "image/png", _png(template), "image/png")
    crops = crop_regions(result.registered_png, [{"question_id": "q1", "label": "Q1", "x": 0.14, "y": 0.27, "width": 0.72, "height": 0.52}])

    assert result.evidence.method in {"orb", "akaze"}
    assert result.evidence.inlier_count >= 8
    assert result.evidence.confidence >= 0.6
    assert len(result.evidence.source_to_template) == 3
    assert crops[0]["question_id"] == "q1"
    assert len(crops[0]["png"]) > 1000


def test_identical_page_uses_identity_registration() -> None:
    page = np.full((300, 200, 3), 255, np.uint8)
    cv2.putText(page, "Q1", (30, 120), cv2.FONT_HERSHEY_SIMPLEX, 2, (0, 0, 0), 3)
    result = register_page(_png(page), "image/png", _png(page), "image/png")
    assert result.evidence.method == "pixel_identity"
    assert result.evidence.confidence == 1.0


def test_manual_four_point_registration_produces_stable_output() -> None:
    page = np.full((400, 300, 3), 255, np.uint8)
    cv2.rectangle(page, (20, 20), (280, 380), (0, 0, 0), 3)
    points = [{"x": 0.0, "y": 0.0}, {"x": 1.0, "y": 0.0}, {"x": 1.0, "y": 1.0}, {"x": 0.0, "y": 1.0}]

    result = register_page_manual(_png(page), "image/png", _png(page), "image/png", points, points)

    assert result.evidence.method == "manual_four_point"
    assert result.evidence.coverage == pytest.approx(1.0, abs=0.02)
    assert result.width == 300
    assert result.height == 400


@pytest.mark.parametrize("points,error", [
    ([{"x": 0.0, "y": 0.0}] * 4, "manual_registration_points_duplicate"),
    ([{"x": 0.0, "y": 0.0}, {"x": 1.0, "y": 1.0}, {"x": 1.0, "y": 0.0}, {"x": 0.0, "y": 1.0}], "manual_registration_points_crossed"),
    ([{"x": -0.1, "y": 0.0}, {"x": 1.0, "y": 0.0}, {"x": 1.0, "y": 1.0}, {"x": 0.0, "y": 1.0}], "manual_registration_points_out_of_range"),
])
def test_manual_registration_rejects_invalid_geometry(points: list[dict], error: str) -> None:
    page = np.full((100, 100, 3), 255, np.uint8)
    target = [{"x": 0.0, "y": 0.0}, {"x": 1.0, "y": 0.0}, {"x": 1.0, "y": 1.0}, {"x": 0.0, "y": 1.0}]
    with pytest.raises(RegistrationError, match=error):
        register_page_manual(_png(page), "image/png", _png(page), "image/png", points, target)

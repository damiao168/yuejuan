from __future__ import annotations

import io

import numpy as np
import pytest
from image_quality.engine import ImageQualityError, analyze_and_normalize
from PIL import Image, ImageDraw, ImageFilter


def test_clear_image_passes_and_outputs_rgb_png() -> None:
    image_bytes = make_clear_answer_like_image()

    result = analyze_and_normalize(image_bytes)

    assert result.quality_status == "passed"
    assert result.quality_report["normalized_color_mode"] == "RGB"
    assert result.quality_report["metrics"]["sharpness_score"] > 0.5
    assert result.normalized_png.startswith(b"\x89PNG")
    with Image.open(io.BytesIO(result.normalized_png)) as normalized:
        assert normalized.mode == "RGB"


def test_blurry_image_triggers_low_sharpness_review() -> None:
    clear = Image.open(io.BytesIO(make_clear_answer_like_image()))
    blurry = clear.filter(ImageFilter.GaussianBlur(radius=7))
    output = io.BytesIO()
    blurry.save(output, format="JPEG", quality=90)

    result = analyze_and_normalize(output.getvalue())

    assert result.quality_status == "review"
    assert result.quality_report["metrics"]["sharpness_score"] < 0.5
    assert any(issue["code"] == "low_sharpness" for issue in result.quality_issues)


def test_exif_rotation_records_transform() -> None:
    image = Image.new("RGB", (320, 520), "white")
    draw = ImageDraw.Draw(image)
    draw.rectangle((20, 20, 300, 500), outline="black", width=5)
    draw.line((40, 120, 280, 120), fill="black", width=3)
    exif = Image.Exif()
    exif[274] = 6
    output = io.BytesIO()
    image.save(output, format="JPEG", exif=exif)

    result = analyze_and_normalize(output.getvalue())

    assert result.normalization_transform["exif_rotation_degrees"] == 90
    assert result.normalization_transform["normalized_pixel_width"] == 520
    assert result.normalization_transform["normalized_pixel_height"] == 320


def test_unreadable_image_is_rejected() -> None:
    with pytest.raises(ImageQualityError, match="unreadable_image"):
        analyze_and_normalize(b"not-an-image")


def test_oversized_dimensions_are_rejected_before_decode(monkeypatch) -> None:
    class OversizedImage:
        width = 100_000
        height = 100_000

        def __enter__(self):
            return self

        def __exit__(self, _exc_type, _exc, _traceback):
            return None

    monkeypatch.setattr("image_quality.engine.Image.open", lambda _: OversizedImage())
    with pytest.raises(ImageQualityError, match="image_dimensions_out_of_range"):
        analyze_and_normalize(b"header")


def test_detects_skew_without_claiming_a_correction() -> None:
    with Image.open(io.BytesIO(make_clear_answer_like_image())) as source:
        rotated = source.rotate(4.0, resample=Image.Resampling.BICUBIC, fillcolor="white")
        image_bytes = encode_image(rotated)

    result = analyze_and_normalize(image_bytes)

    geometry = result.quality_report["geometry"]
    assert abs(abs(geometry["detected_skew_angle"]) - 4.0) < 1.5
    assert geometry["skew_confidence"] > 0.2
    assert geometry["skew_correction_applied"] is False
    assert result.normalization_transform["skew_correction_applied"] is False
    assert result.normalization_transform["content_rotation_degrees"] == 0


def test_complete_page_border_scores_higher_than_cropped_border() -> None:
    complete = analyze_and_normalize(make_clear_answer_like_image())
    with Image.open(io.BytesIO(make_clear_answer_like_image())) as source:
        cropped = source.copy()
    ImageDraw.Draw(cropped).rectangle((730, 0, cropped.width, cropped.height), fill="white")
    cropped_result = analyze_and_normalize(encode_image(cropped))

    complete_score = complete.quality_report["metrics"]["border_completeness_score"]
    cropped_score = cropped_result.quality_report["metrics"]["border_completeness_score"]
    assert complete_score > cropped_score
    assert complete.quality_report["geometry"]["page_border_status"] == "complete"


def test_trapezoid_has_more_perspective_risk_than_rectangle() -> None:
    rectangle = analyze_and_normalize(make_page_on_background([(80, 60), (720, 60), (720, 1040), (80, 1040)]))
    trapezoid = analyze_and_normalize(make_page_on_background([(190, 60), (610, 60), (740, 1040), (60, 1040)]))

    rectangle_risk = rectangle.quality_report["metrics"]["perspective_risk_score"]
    trapezoid_risk = trapezoid.quality_report["metrics"]["perspective_risk_score"]
    assert trapezoid_risk > rectangle_risk
    assert trapezoid.quality_report["geometry"]["perspective_confidence"] > 0.4


def test_gradient_shadow_has_more_risk_than_uniform_page() -> None:
    clean_bytes = make_clear_answer_like_image()
    with Image.open(io.BytesIO(clean_bytes)) as source:
        pixels = np.asarray(source.convert("RGB"), dtype=np.float32)
    gradient = np.linspace(1.0, 0.42, pixels.shape[1], dtype=np.float32)[None, :, None]
    shadowed = Image.fromarray(np.clip(pixels * gradient, 0, 255).astype(np.uint8), mode="RGB")

    clean = analyze_and_normalize(clean_bytes)
    shadow = analyze_and_normalize(encode_image(shadowed))

    assert shadow.quality_report["metrics"]["shadow_risk_score"] > clean.quality_report["metrics"]["shadow_risk_score"]


def make_clear_answer_like_image() -> bytes:
    image = Image.new("RGB", (800, 1100), "white")
    draw = ImageDraw.Draw(image)
    draw.rectangle((35, 35, 765, 1065), outline="black", width=6)
    for y in range(120, 980, 80):
        draw.line((90, y, 710, y), fill="black", width=4)
        draw.line((90, y + 28, 430, y + 28), fill="black", width=3)
    for x in range(140, 680, 120):
        draw.rectangle((x, 60, x + 24, 84), outline="black", width=3)
    output = io.BytesIO()
    image.save(output, format="JPEG", quality=95)
    return output.getvalue()


def make_page_on_background(points: list[tuple[int, int]]) -> bytes:
    image = Image.new("RGB", (800, 1100), (45, 45, 45))
    draw = ImageDraw.Draw(image)
    draw.polygon(points, fill="white", outline="black", width=5)
    return encode_image(image)


def encode_image(image: Image.Image) -> bytes:
    output = io.BytesIO()
    image.save(output, format="PNG")
    return output.getvalue()

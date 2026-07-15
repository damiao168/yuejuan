from __future__ import annotations

import io

from PIL import Image, ImageDraw

from page_processing.omr import OMRExtractionError, extract_marks


REGIONS = [
    {"label": "A", "x": 20, "y": 20, "width": 30, "height": 30},
    {"label": "B", "x": 70, "y": 20, "width": 30, "height": 30},
    {"label": "C", "x": 120, "y": 20, "width": 30, "height": 30},
]


def sheet(*marked: str) -> bytes:
    image = Image.new("RGB", (180, 70), "white")
    draw = ImageDraw.Draw(image)
    for region in REGIONS:
        box = (region["x"], region["y"], region["x"] + region["width"], region["y"] + region["height"])
        draw.rectangle(box, outline="black", width=2)
        if region["label"] in marked:
            draw.ellipse((box[0] + 6, box[1] + 6, box[2] - 6, box[3] - 6), fill="black")
    output = io.BytesIO()
    image.save(output, format="PNG")
    return output.getvalue()


def test_extracts_one_clear_mark() -> None:
    result = extract_marks(sheet("B"), REGIONS)
    assert result["decision"] == "selected"
    assert result["selected"] == ["B"]
    assert result["needs_human_review"] is False
    assert result["confidence"] >= 0.8
    assert result["overlay_png"].startswith(b"\x89PNG")


def test_blank_and_multiple_route_to_review() -> None:
    blank = extract_marks(sheet(), REGIONS)
    assert blank["decision"] == "blank"
    assert blank["needs_human_review"] is True

    multiple = extract_marks(sheet("A", "C"), REGIONS)
    assert multiple["decision"] == "multiple"
    assert multiple["needs_human_review"] is True


def test_multiple_choice_preserves_clear_set() -> None:
    result = extract_marks(sheet("A", "C"), REGIONS, multiple=True)
    assert result["decision"] == "selected"
    assert result["selected"] == ["A", "C"]


def test_rejects_missing_or_unsafe_regions() -> None:
    try:
        extract_marks(sheet("A"), [])
        raise AssertionError("missing regions must fail")
    except OMRExtractionError as exc:
        assert str(exc) == "omr_option_regions_missing"

    try:
        extract_marks(sheet("A"), [{"label": "A", "x": -1, "y": 0, "width": 5, "height": 5}])
        raise AssertionError("unsafe region must fail")
    except OMRExtractionError as exc:
        assert str(exc) == "omr_option_region_out_of_bounds"

from __future__ import annotations

import io

import pypdfium2 as pdfium
import pytest
from PIL import Image

from page_processing.decoder import DecodeError, decode_document


def test_decodes_single_image_to_rgb_png() -> None:
    source = io.BytesIO()
    Image.new("L", (320, 480), color=240).save(source, format="JPEG")
    pages, detected = decode_document(source.getvalue(), "image/jpeg")
    assert detected == "image/jpeg"
    assert len(pages) == 1
    assert (pages[0].width, pages[0].height) == (320, 480)
    assert pages[0].png.startswith(b"\x89PNG")


def test_decodes_multiframe_tiff() -> None:
    source = io.BytesIO()
    first = Image.new("RGB", (100, 200), "white")
    second = Image.new("RGB", (100, 200), "gray")
    first.save(source, format="TIFF", save_all=True, append_images=[second])
    pages, detected = decode_document(source.getvalue(), "image/tiff")
    assert detected == "image/tiff"
    assert len(pages) == 2


@pytest.mark.parametrize(
    ("mode", "compression"),
    [("1", "group4"), ("L", "tiff_lzw"), ("RGB", "tiff_adobe_deflate")],
)
def test_decodes_production_tiff_modes(mode: str, compression: str) -> None:
    source = io.BytesIO()
    first = Image.new(mode, (96, 128), 1 if mode == "1" else 240)
    second = Image.new(mode, (96, 128), 0 if mode == "1" else 180)
    first.save(source, format="TIFF", compression=compression, save_all=True, append_images=[second])
    pages, detected = decode_document(source.getvalue(), "application/octet-stream")
    assert detected == "image/tiff"
    assert [(page.width, page.height) for page in pages] == [(96, 128), (96, 128)]
    assert all(page.png.startswith(b"\x89PNG") for page in pages)


def test_sniffs_real_format_before_declared_mime() -> None:
    source = io.BytesIO()
    Image.new("RGB", (40, 50), "white").save(source, format="TIFF")
    pages, detected = decode_document(source.getvalue(), "application/pdf")
    assert len(pages) == 1
    assert detected == "image/tiff"


def test_rejects_tiff_frame_and_total_pixel_limits_before_materializing_all_pages() -> None:
    source = io.BytesIO()
    frames = [Image.new("L", (100, 100), color=index * 20) for index in range(3)]
    frames[0].save(source, format="TIFF", save_all=True, append_images=frames[1:])
    with pytest.raises(DecodeError, match="image_frame_limit_exceeded"):
        decode_document(source.getvalue(), "image/tiff", max_pages=2)
    with pytest.raises(DecodeError, match="document_pixel_limit_exceeded"):
        decode_document(source.getvalue(), "image/tiff", max_total_pixels=20_000)


def test_reports_stable_error_for_truncated_tiff() -> None:
    source = io.BytesIO()
    Image.new("RGB", (100, 100), "white").save(source, format="TIFF")
    with pytest.raises(DecodeError, match="invalid_tiff"):
        decode_document(source.getvalue()[:64], "image/tiff")


def test_decodes_pdf_pages() -> None:
    document = pdfium.PdfDocument.new()
    document.new_page(144, 216)
    document.new_page(144, 216)
    source = io.BytesIO()
    document.save(source)
    document.close()
    pages, detected = decode_document(source.getvalue(), "application/pdf", render_dpi=72)
    assert detected == "application/pdf"
    assert len(pages) == 2
    assert (pages[0].width, pages[0].height) == (144, 216)


def test_rejects_limits_and_invalid_input() -> None:
    with pytest.raises(DecodeError):
        decode_document(b"not-an-image", "image/png")
    source = io.BytesIO()
    Image.new("RGB", (200, 200), "white").save(source, format="PNG")
    with pytest.raises(DecodeError, match="page_pixel_limit_exceeded"):
        decode_document(source.getvalue(), "image/png", max_page_pixels=100)

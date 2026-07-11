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

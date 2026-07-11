from __future__ import annotations

import io
from dataclasses import dataclass

import pypdfium2 as pdfium
from PIL import Image, ImageOps, ImageSequence, UnidentifiedImageError


class DecodeError(RuntimeError):
    pass


@dataclass(frozen=True)
class DecodedPage:
    index: int
    png: bytes
    width: int
    height: int


def decode_document(
    data: bytes,
    content_type: str,
    *,
    render_dpi: int = 300,
    max_pages: int = 500,
    max_page_pixels: int = 50_000_000,
    max_total_pixels: int = 1_000_000_000,
) -> tuple[list[DecodedPage], str]:
    if not data:
        raise DecodeError("empty_document")
    if data.startswith(b"%PDF-") or content_type == "application/pdf":
        images = _decode_pdf(data, render_dpi, max_pages)
        detected = "application/pdf"
    else:
        images, detected = _decode_image(data, max_pages)
    pages: list[DecodedPage] = []
    total_pixels = 0
    for index, image in enumerate(images, start=1):
        image = ImageOps.exif_transpose(image).convert("RGB")
        pixels = image.width * image.height
        total_pixels += pixels
        if pixels <= 0 or pixels > max_page_pixels:
            raise DecodeError("page_pixel_limit_exceeded")
        if total_pixels > max_total_pixels:
            raise DecodeError("document_pixel_limit_exceeded")
        output = io.BytesIO()
        image.save(output, format="PNG", optimize=False)
        pages.append(DecodedPage(index=index, png=output.getvalue(), width=image.width, height=image.height))
        image.close()
    if not pages:
        raise DecodeError("document_has_no_pages")
    return pages, detected


def _decode_pdf(data: bytes, render_dpi: int, max_pages: int) -> list[Image.Image]:
    try:
        document = pdfium.PdfDocument(data)
    except Exception as exc:
        raise DecodeError("invalid_pdf") from exc
    try:
        if len(document) == 0 or len(document) > max_pages:
            raise DecodeError("pdf_page_limit_exceeded")
        scale = render_dpi / 72.0
        images: list[Image.Image] = []
        for page_index in range(len(document)):
            page = document[page_index]
            try:
                bitmap = page.render(scale=scale, rotation=0, rev_byteorder=True)
                images.append(bitmap.to_pil().copy())
                bitmap.close()
            finally:
                page.close()
        return images
    finally:
        document.close()


def _decode_image(data: bytes, max_pages: int) -> tuple[list[Image.Image], str]:
    try:
        source = Image.open(io.BytesIO(data))
    except (UnidentifiedImageError, OSError) as exc:
        raise DecodeError("unsupported_or_invalid_image") from exc
    try:
        frame_count = getattr(source, "n_frames", 1)
        if frame_count <= 0 or frame_count > max_pages:
            raise DecodeError("image_frame_limit_exceeded")
        images = [frame.copy() for frame in ImageSequence.Iterator(source)]
        detected = "image/tiff" if (source.format or "").upper() == "TIFF" else Image.MIME.get(source.format or "", "image/unknown")
        return images, detected
    finally:
        source.close()

from __future__ import annotations

from io import BytesIO

import numpy as np
from PIL import Image
import zxingcpp

MAX_BARCODES_PER_PAGE = 8
MAX_BARCODE_TEXT_LENGTH = 2048


def detect_barcodes(png: bytes) -> list[dict]:
    image = np.asarray(Image.open(BytesIO(png)).convert("RGB"))
    observations: list[dict] = []
    for result in zxingcpp.read_barcodes(image)[:MAX_BARCODES_PER_PAGE]:
        text = str(result.text or "")
        if not text or len(text) > MAX_BARCODE_TEXT_LENGTH:
            continue
        position = result.position
        polygon = [
            {"x": int(point.x), "y": int(point.y)}
            for point in (position.top_left, position.top_right, position.bottom_right, position.bottom_left)
        ]
        observations.append({
            "format": str(result.format),
            "text": text,
            "polygon": polygon,
            "orientation": int(getattr(result, "orientation", 0) or 0),
        })
    return observations

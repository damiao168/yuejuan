from io import BytesIO

from page_processing import barcode
from PIL import Image


class Point:
    def __init__(self, x: int, y: int) -> None:
        self.x = x
        self.y = y


class Position:
    top_left = Point(1, 2)
    top_right = Point(10, 2)
    bottom_right = Point(10, 12)
    bottom_left = Point(1, 12)


class Result:
    text = "EG1.payload.signature"
    format = "QRCode"
    orientation = 90
    position = Position()


def test_detect_barcodes_returns_bounded_observation(monkeypatch):
    monkeypatch.setattr(barcode.zxingcpp, "read_barcodes", lambda _image: [Result()])
    image = Image.new("RGB", (20, 20), "white")
    output = BytesIO()
    image.save(output, format="PNG")

    observations = barcode.detect_barcodes(output.getvalue())

    assert observations == [{
        "format": "QRCode",
        "text": "EG1.payload.signature",
        "polygon": [{"x": 1, "y": 2}, {"x": 10, "y": 2}, {"x": 10, "y": 12}, {"x": 1, "y": 12}],
        "orientation": 90,
    }]

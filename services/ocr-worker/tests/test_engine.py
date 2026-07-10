import unittest

from ocr_worker.engine import _create_paddle_ocr, _parse_paddle_result


class EngineTests(unittest.TestCase):
    def test_parse_paddle3_predict_result(self):
        raw = [
            {
                "rec_texts": ["智能阅卷 OCR 测试页", "得分"],
                "rec_scores": [0.95, 0.88],
                "rec_polys": [
                    [[10, 20], [210, 20], [210, 60], [10, 60]],
                    [[30, 90], [130, 90], [130, 120], [30, 120]],
                ],
            }
        ]

        blocks = _parse_paddle_result(raw)

        self.assertEqual([block.text for block in blocks], ["智能阅卷 OCR 测试页", "得分"])
        self.assertEqual(blocks[0].bbox, [10.0, 20.0, 200.0, 40.0])
        self.assertEqual(blocks[0].confidence, 0.95)

    def test_parse_paddle3_nested_json_result(self):
        raw = [
            {
                "res": {
                    "rec_texts": ["智能阅卷 OCR 测试页"],
                    "rec_scores": [0.95],
                    "rec_polys": [[[79, 77], [527, 77], [527, 125], [79, 125]]],
                }
            }
        ]

        blocks = _parse_paddle_result(raw)

        self.assertEqual(blocks[0].text, "智能阅卷 OCR 测试页")
        self.assertEqual(blocks[0].bbox, [79.0, 77.0, 448.0, 48.0])

    def test_parse_paddle3_ocr_result_prefers_json_property(self):
        class FakeOCRResult(dict):
            @property
            def json(self):
                return {
                    "res": {
                        "rec_texts": ["智能阅卷 OCR 测试页"],
                        "rec_scores": [0.95],
                        "rec_polys": [[[79, 77], [527, 77], [527, 125], [79, 125]]],
                    }
                }

        raw = [FakeOCRResult(rec_texts=["智能阅卷 OCR 测试页"], rec_scores=[0.95])]

        blocks = _parse_paddle_result(raw)

        self.assertEqual(blocks[0].bbox, [79.0, 77.0, 448.0, 48.0])

    def test_create_paddle_ocr_uses_v3_arguments_first(self):
        calls = []

        class FakePaddleOCR:
            def __init__(self, **kwargs):
                calls.append(kwargs)

        _create_paddle_ocr(FakePaddleOCR, language="ch", device="cpu")

        self.assertEqual(calls[0]["lang"], "ch")
        self.assertFalse(calls[0]["use_doc_orientation_classify"])
        self.assertFalse(calls[0]["use_doc_unwarping"])
        self.assertTrue(calls[0]["use_textline_orientation"])
        self.assertNotIn("use_gpu", calls[0])

    def test_create_paddle_ocr_falls_back_to_v2_arguments(self):
        calls = []

        class FakePaddleOCR:
            def __init__(self, **kwargs):
                calls.append(kwargs)
                if "use_gpu" not in kwargs:
                    raise ValueError("Unknown argument: use_doc_unwarping")

        _create_paddle_ocr(FakePaddleOCR, language="ch", device="gpu")

        self.assertTrue(calls[1]["use_angle_cls"])
        self.assertTrue(calls[1]["use_gpu"])


if __name__ == "__main__":
    unittest.main()

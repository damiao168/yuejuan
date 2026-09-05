import unittest
from pathlib import Path

from ocr_worker.engine import PaddleOCREngine, _create_paddle_ocr, _parse_paddle_result


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

    def test_missing_polygon_does_not_fabricate_a_valid_bounding_box(self):
        blocks = _parse_paddle_result([{"rec_texts": ["text"], "rec_scores": [0.9]}])

        self.assertEqual(blocks[0].bbox, [0.0, 0.0, 0.0, 0.0])

    def test_create_paddle_ocr_uses_exact_model_profile_and_device(self):
        calls = []

        class FakePaddleOCR:
            def __init__(self, **kwargs):
                calls.append(kwargs)

        _create_paddle_ocr(FakePaddleOCR, model_version="ppocr-v5-server", device="cpu")

        self.assertEqual(calls[0]["text_detection_model_name"], "PP-OCRv5_server_det")
        self.assertEqual(calls[0]["text_recognition_model_name"], "PP-OCRv5_server_rec")
        self.assertEqual(calls[0]["device"], "cpu")
        self.assertFalse(calls[0]["enable_mkldnn"])
        self.assertFalse(calls[0]["use_doc_orientation_classify"])
        self.assertFalse(calls[0]["use_doc_unwarping"])
        self.assertTrue(calls[0]["use_textline_orientation"])

    def test_mobile_profile_uses_lightweight_v5_models(self):
        calls = []

        class FakePaddleOCR:
            def __init__(self, **kwargs):
                calls.append(kwargs)

        _create_paddle_ocr(FakePaddleOCR, model_version="ppocr-v5-mobile", device="cpu")

        self.assertEqual(calls[0]["text_detection_model_name"], "PP-OCRv5_mobile_det")
        self.assertEqual(calls[0]["text_recognition_model_name"], "PP-OCRv5_mobile_rec")

    def test_mkldnn_auto_is_mobile_only_and_false_is_never_enabled(self):
        self.assertTrue(PaddleOCREngine(model_version="ppocr-v5-mobile", enable_mkldnn="auto").effective_mkldnn)
        self.assertFalse(PaddleOCREngine(model_version="ppocr-v5-server", enable_mkldnn="auto").effective_mkldnn)
        self.assertFalse(PaddleOCREngine(model_version="ppocr-v5-mobile", enable_mkldnn="false").effective_mkldnn)

    def test_unknown_model_is_rejected_instead_of_misreported(self):
        with self.assertRaisesRegex(ValueError, "unsupported OCR model version"):
            PaddleOCREngine(model_version="not-a-real-model")

    def test_unsupported_device_is_rejected_instead_of_misreported(self):
        with self.assertRaisesRegex(ValueError, "supports only"):
            PaddleOCREngine(device="gpu:0")

    def test_initialize_runs_full_inference_probe_once(self):
        calls = []
        test_case = self

        class FakeOCR:
            def predict(self, image_path):
                self_path = Path(image_path)
                test_case.assertTrue(self_path.exists())
                test_case.assertGreater(self_path.stat().st_size, 0)
                calls.append(image_path)
                return [{
                    "rec_texts": ["OCR 123"],
                    "rec_scores": [0.99],
                    "rec_polys": [[[10, 10], [200, 10], [200, 60], [10, 60]]],
                }]

        engine = PaddleOCREngine()
        engine._ocr = FakeOCR()

        engine.initialize()
        engine.initialize()

        self.assertEqual(len(calls), 1)

    def test_initialize_rejects_detection_only_probe(self):
        class FakeOCR:
            def predict(self, _image_path):
                return []

        engine = PaddleOCREngine()
        engine._ocr = FakeOCR()

        with self.assertRaisesRegex(RuntimeError, "no valid recognized text"):
            engine.initialize()

    def test_region_inference_preserves_small_crop_upscaling(self):
        calls = []

        class FakeOCR:
            def predict(self, _image_path, **kwargs):
                calls.append(kwargs)
                return [{"rec_texts": ["answer"], "rec_scores": [0.9], "rec_polys": [[[1, 1], [10, 1], [10, 5], [1, 5]]]}]

        engine = PaddleOCREngine(text_det_limit_type="max", text_det_limit_side_len=960)
        engine._ocr = FakeOCR()
        engine.recognize_region(b"png")
        self.assertEqual(calls, [{"text_det_limit_type": "min", "text_det_limit_side_len": 64}])

    def test_mobile_auto_falls_back_for_known_onednn_error(self):
        modes = []

        class FakeOCR:
            def __init__(self, mkldnn):
                self.mkldnn = mkldnn

            def predict(self, _image_path):
                if self.mkldnn:
                    raise RuntimeError("oneDNN PIR attribute conversion failed")
                return [{"rec_texts": ["OCR 123"], "rec_scores": [0.99], "rec_polys": [[[10, 10], [200, 10], [200, 60], [10, 60]]]}]

        class TestEngine(PaddleOCREngine):
            def _load(self):
                if self._ocr is None:
                    modes.append(self.effective_mkldnn)
                    self._ocr = FakeOCR(self.effective_mkldnn)
                return self._ocr

        engine = TestEngine(enable_mkldnn="auto")
        engine.initialize()
        self.assertEqual(modes, [True, False])
        self.assertFalse(engine.effective_mkldnn)

    def test_forced_true_and_unknown_errors_do_not_fallback(self):
        class FailingOCR:
            def __init__(self, message):
                self.message = message

            def predict(self, _image_path):
                raise RuntimeError(self.message)

        for mode, message in (("true", "oneDNN PIR attribute conversion failed"), ("auto", "unrelated model corruption"), ("auto", "model download token expired"), ("auto", "PIR model file missing")):
            engine = PaddleOCREngine(enable_mkldnn=mode)
            engine._ocr = FailingOCR(message)
            with self.assertRaisesRegex(RuntimeError, message):
                engine.initialize()


if __name__ == "__main__":
    unittest.main()

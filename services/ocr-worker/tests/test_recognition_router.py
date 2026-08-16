from ocr_worker.engine import OCRBlock
from ocr_worker.recognition_router import (
    FormulaRecognitionResult,
    RecognitionRouter,
    RegionKind,
    UniMERNetEngine,
    _parse_formula_prediction,
)


class TextEngine:
    model_version = "text-v1"
    device = "cpu"

    def initialize(self):
        return None

    def recognize(self, _image):
        return [OCRBlock("所以", [0, 0, 10, 10], 0.9)]


class FormulaEngine:
    engine_name = "fixture"
    model_version = "formula-v1"

    def initialize(self):
        return None

    def recognize_formula(self, _image):
        return FormulaRecognitionResult(" x = 2 ", "x=2", 0.9, self.engine_name, self.model_version, "recognized")


def test_routes_regions_and_fails_closed():
    router = RecognitionRouter(text_engine=TextEngine(), formula_engine=FormulaEngine())
    assert router.recognize("mathematics", RegionKind.TEXT, b"image").text_blocks[0].text == "所以"
    assert router.recognize("mathematics", RegionKind.FORMULA, b"image").formula.canonical_latex == "x=2"
    diagram = router.recognize("mathematics", RegionKind.DIAGRAM, b"image")
    assert diagram.preserve_image_evidence and diagram.requires_human_review
    assert router.recognize("mathematics", RegionKind.UNKNOWN, b"image").requires_human_review


def test_missing_formula_engine_never_falls_back_to_text():
    result = RecognitionRouter(text_engine=TextEngine(), formula_engine=None).recognize("physics", RegionKind.FORMULA, b"image")
    assert result.requires_human_review
    assert result.reason_code == "formula_engine_unavailable"
    assert not result.text_blocks


def test_humanities_never_invoke_formula_model():
    class ExplodingFormulaEngine(FormulaEngine):
        def recognize_formula(self, _image):
            raise AssertionError("formula engine must not run for humanities")

    router = RecognitionRouter(text_engine=TextEngine(), formula_engine=ExplodingFormulaEngine())
    for subject in ("chinese", "english", "history", "ethics_politics", "geography", "biology"):
        result = router.recognize(subject, RegionKind.FORMULA, b"image")
        assert result.reason_code == "formula_not_enabled_for_subject"
        assert result.preserve_image_evidence and result.requires_human_review


def test_paddle_formula_result_and_unimernet_adapter():
    latex, confidence = _parse_formula_prediction([{"res": {"rec_formula": "x^2", "rec_score": 0.8}}])
    assert (latex, confidence) == ("x^2", 0.8)
    challenger = UniMERNetEngine(model_version="checkpoint-1", predictor=lambda _: (" y = 3 ", 0.7))
    assert challenger.recognize_formula(b"image").canonical_latex == "y=3"

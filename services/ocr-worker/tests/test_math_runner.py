from types import SimpleNamespace

from ocr_worker.api import APIError
from ocr_worker.engine import OCRBlock
from ocr_worker.math_runner import _build_artifact
from ocr_worker.recognition_router import (
    FormulaRecognitionResult,
    FormulaRegionRecognition,
    RegionKind,
    RoutedRecognitionResult,
)


class Verifier:
    def normalize(self, latex: str):
        assert latex == "x=2"
        return {"ast": {"kind": "equation", "children": [{"kind": "symbol", "value": "x"}, {"kind": "number", "value": "2"}]}}


def test_build_artifact_binds_formula_to_immutable_task_input():
    task = {
        "answer_segment_id": "segment-1",
        "exam_question_snapshot_id": "snapshot-1",
        "subject_code": "mathematics",
        "input_hash": "crop-sha256",
    }
    routed = RoutedRecognitionResult(
        kind=RegionKind.FORMULA,
        formula=FormulaRecognitionResult("x = 2", "x=2", 0.91, "paddle-formula", "PP-FormulaNet_plus-M", "recognized"),
    )
    artifact = _build_artifact(task, routed, Verifier())
    assert artifact["answer_segment_id"] == "segment-1"
    assert artifact["exam_question_snapshot_id"] == "snapshot-1"
    assert artifact["formulas"][0]["ast"]["kind"] == "equation"
    assert artifact["verifications"][0]["status"] == "verified"
    assert artifact["verifications"][0]["formula_id"] == "formula-1"
    assert "step_id" not in artifact["verifications"][0]


def test_failed_formula_recognition_still_produces_reviewable_evidence():
    task = {
        "answer_segment_id": "segment-2",
        "exam_question_snapshot_id": "snapshot-2",
        "subject_code": "physics",
        "input_hash": "crop-sha256-2",
    }
    routed = RoutedRecognitionResult(
        kind=RegionKind.FORMULA,
        preserve_image_evidence=True,
        requires_human_review=True,
        reason_code="formula_engine_failed",
    )
    artifact = _build_artifact(task, routed, SimpleNamespace())
    assert artifact["formulas"] == []
    assert artifact["blocks"][0]["kind"] == "unknown"
    assert artifact["solution_graph"]["requires_human_review"] is True


def test_symbolic_service_outage_does_not_discard_recognition_evidence():
    task = {"answer_segment_id": "s", "exam_question_snapshot_id": "q", "subject_code": "mathematics", "input_hash": "hash"}
    routed = RoutedRecognitionResult(kind=RegionKind.FORMULA, formula=FormulaRecognitionResult(
        "x=2", "x=2", .9, "fixture", "v1", "recognized",
    ))

    class Unavailable:
        def normalize(self, latex):
            raise APIError("unavailable")

    artifact = _build_artifact(task, routed, Unavailable())
    assert artifact["formulas"][0]["canonical_latex"] == "x=2"
    assert artifact["formulas"][0]["ast"] is None
    assert artifact["verifications"][0]["status"] == "uncertain"
    assert artifact["verifications"][0]["reason_code"] == "verification_unavailable"


def test_build_mixed_artifact_normalizes_boxes_and_preserves_formula_candidates():
    task = {
        "answer_segment_id": "segment-3",
        "exam_question_snapshot_id": "snapshot-3",
        "subject_code": "mathematics",
        "input_hash": "crop-sha256-3",
    }
    formula = FormulaRecognitionResult(
        "x = 2", "x=2", 0.91, "paddle-formula", "PP-FormulaNet_plus-M", "recognized",
    )
    routed = RoutedRecognitionResult(
        kind=RegionKind.MIXED,
        text_blocks=(OCRBlock("解：由题意得", [0, 10, 60, 20], 0.85),),
        formula_regions=(
            FormulaRegionRecognition(
                bbox=(60, 10, 40, 20),
                detector_confidence=0.92,
                crop_complete=True,
                candidates=(formula,),
                selected_candidate=0,
            ),
        ),
        image_size=(100, 40),
        layout_version="layout-v1",
    )

    artifact = _build_artifact(task, routed, Verifier())

    assert artifact["engine_version"] == "math-runtime-v2"
    assert [block["kind"] for block in artifact["blocks"]] == ["text", "formula"]
    assert artifact["blocks"][1]["bbox"] == {"x": 0.6, "y": 0.25, "width": 0.4, "height": 0.5}
    assert artifact["formulas"][0]["candidates"] == [
        {
            "latex": "x=2",
            "engine": "paddle-formula",
            "version": "PP-FormulaNet_plus-M",
            "confidence": 0.91,
            "syntax_valid": True,
        }
    ]
    assert artifact["formulas"][0]["selected_candidate"] == 0
    assert artifact["solution_graph"]["steps"][0]["formula_ids"] == ["formula-1"]

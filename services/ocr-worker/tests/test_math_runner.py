from types import SimpleNamespace

from ocr_worker.math_runner import _build_artifact
from ocr_worker.recognition_router import (
    FormulaRecognitionResult,
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

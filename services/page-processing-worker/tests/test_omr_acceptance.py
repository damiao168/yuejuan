from page_processing.omr_acceptance import build_synthetic_acceptance_cases, evaluate_synthetic_acceptance


def test_story056_synthetic_omr_acceptance_has_100_mixed_submissions() -> None:
    cases = build_synthetic_acceptance_cases()
    assert len(cases) == 100
    assert {case.question_type for case in cases} == {"single_choice", "true_false", "multiple_choice"}


def test_story056_synthetic_omr_acceptance_routes_risk_without_silent_zero() -> None:
    report = evaluate_synthetic_acceptance()
    assert report["total_submissions"] == 100
    assert report["ambiguous_recall"] == 1.0
    assert report["silent_zero_count"] == 0
    assert report["false_zero_routed_count"] == 0
    assert report["wrong_auto_selection_count"] == 0
    assert report["omr_confusion_matrix"]["expected_auto"]["review"] == 0
    assert report["omr_confusion_matrix"]["expected_review"]["auto"] == 0
    assert report["p95_duration_ms"] >= 0

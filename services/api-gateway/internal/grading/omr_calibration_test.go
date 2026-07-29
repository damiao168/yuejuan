package grading

import "testing"

func TestSummarizeOMRCalibrationRequiresCompleteBalancedZeroErrorEvidence(t *testing.T) {
	session := OMRCalibrationSession{
		OptionLabels:            []string{"A", "B"},
		MinimumSamples:          100,
		MinimumSamplesPerOption: 10,
		Status:                  "draft",
	}
	cases := make([]OMRCalibrationCase, 100)
	for i := range cases {
		matches := true
		label := "A"
		if i >= 50 {
			label = "B"
		}
		cases[i] = OMRCalibrationCase{
			ID:              string(rune(i + 1)),
			ExpectedOptions: []string{label},
			Matches:         &matches,
		}
	}

	summary := summarizeOMRCalibration(session, cases)
	if !summary.ReadyToApprove || summary.MatchCount != 100 || summary.OptionCoverage["A"] != 50 || summary.OptionCoverage["B"] != 50 {
		t.Fatalf("complete balanced exact evidence must be approvable: %#v", summary)
	}

	mismatch := false
	cases[0].Matches = &mismatch
	summary = summarizeOMRCalibration(session, cases)
	if summary.ReadyToApprove || summary.MismatchCount != 1 || !containsString(summary.Blockers, "calibration_mismatch_detected") {
		t.Fatalf("one disagreement must block approval: %#v", summary)
	}

	cases[0].Matches = nil
	cases[0].ExpectedOptions = nil
	summary = summarizeOMRCalibration(session, cases)
	if summary.ReadyToApprove || summary.PendingCount != 1 || !containsString(summary.Blockers, "calibration_labels_pending") {
		t.Fatalf("an unlabeled sample must block approval: %#v", summary)
	}

	matched := true
	cases[0].Matches = &matched
	cases[0].ExpectedOptions = []string{"A"}
	for i := range cases {
		cases[i].ExpectedOptions = []string{"A"}
	}
	summary = summarizeOMRCalibration(session, cases)
	if summary.ReadyToApprove || summary.OptionCoverage["B"] != 0 || !containsString(summary.Blockers, "option_coverage_incomplete") {
		t.Fatalf("unrepresented option must block approval: %#v", summary)
	}
}

func TestSummarizeTemplateOMRCalibrationRequiresStrataAndOnlyBlocksEligibleErrors(t *testing.T) {
	session := OMRCalibrationSession{
		ScopeType:                "template",
		QuestionIDs:              []string{"q1", "q2"},
		OptionLabels:             []string{"A", "B"},
		MinimumSamples:           100,
		MinimumSamplesPerOption:  10,
		MinimumSamplesPerStratum: 10,
		Status:                   "draft",
	}
	strata := []string{"selected_high", "selected_low", "blank", "ambiguous"}
	cases := make([]OMRCalibrationCase, 100)
	for index := range cases {
		matches := index%4 == 0
		label := "A"
		if index%2 == 1 {
			label = "B"
		}
		expected := []string{label}
		if strata[index%4] == "blank" {
			expected = []string{}
			matches = true
		}
		cases[index] = OMRCalibrationCase{
			ID:              string(rune(index + 1)),
			QuestionID:      session.QuestionIDs[index%2],
			SampleStratum:   strata[index%4],
			ExpectedOptions: expected,
			Matches:         &matches,
		}
	}
	summary := summarizeOMRCalibration(session, cases)
	if !summary.ReadyToApprove || summary.MismatchCount == 0 || summary.EligibleMismatchCount != 0 {
		t.Fatalf("errors outside the auto-confirm branch must remain review-only without blocking approval: %#v", summary)
	}

	mismatch := false
	cases[0].Matches = &mismatch
	summary = summarizeOMRCalibration(session, cases)
	if summary.ReadyToApprove || summary.EligibleMismatchCount != 1 || !containsString(summary.Blockers, "eligible_calibration_mismatch_detected") {
		t.Fatalf("a high-confidence candidate error must block template approval: %#v", summary)
	}

	cases[0].Matches = nil
	cases[0].ExpectedOptions = nil
	for index := range cases {
		if cases[index].SampleStratum == "ambiguous" {
			cases[index].SampleStratum = "selected_low"
		}
	}
	summary = summarizeOMRCalibration(session, cases)
	if summary.ReadyToApprove || !containsString(summary.Blockers, "stratum_coverage_incomplete") {
		t.Fatalf("missing a result stratum must block template approval: %#v", summary)
	}
}

func TestDecorateOMRCalibrationDetailKeepsWorkerResultBlindUntilLabeled(t *testing.T) {
	detail := OMRCalibrationDetail{Cases: []OMRCalibrationCase{{
		AnswerSegmentID:    "segment-1",
		SampleStratum:      "selected_high",
		ObservedDecision:   "selected",
		ObservedOptions:    []string{"A"},
		ObservedConfidence: 0.99,
		Measurements:       []map[string]any{{"option": "A"}},
	}}}
	blind := decorateOMRCalibrationDetail(detail)
	if blind.Cases[0].ObservedDecision != "" || blind.Cases[0].ObservedOptions != nil || blind.Cases[0].Measurements != nil || blind.Cases[0].SampleStratum != "" {
		t.Fatalf("unlabeled calibration response leaked the worker result: %#v", blind.Cases[0])
	}
	if blind.Cases[0].SegmentImageURL == "" {
		t.Fatal("blind calibration response must retain the protected image route")
	}

	matched := true
	revealed := decorateOMRCalibrationDetail(OMRCalibrationDetail{Cases: []OMRCalibrationCase{{
		AnswerSegmentID:    "segment-1",
		SampleStratum:      "selected_high",
		ObservedDecision:   "selected",
		ObservedOptions:    []string{"A"},
		ObservedConfidence: 0.99,
		Measurements:       []map[string]any{{"option": "A"}},
		Matches:            &matched,
	}}})
	if revealed.Cases[0].ObservedDecision != "selected" || len(revealed.Cases[0].ObservedOptions) != 1 || revealed.Cases[0].SampleStratum != "selected_high" {
		t.Fatalf("labeled calibration response must reveal the frozen comparison: %#v", revealed.Cases[0])
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

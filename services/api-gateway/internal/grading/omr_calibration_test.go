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

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

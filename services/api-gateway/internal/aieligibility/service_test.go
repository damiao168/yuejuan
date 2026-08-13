package aieligibility

import (
	"context"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/assessment"
)

func TestEligibilityBlocksR3ExtendedResponseBeforeExternalAI(t *testing.T) {
	service := NewService(NewMemoryStore())
	if _, err := service.PutPolicy(context.Background(), "tenant-a", policyFor(assessment.SubjectChinese, "extended_response", assessment.RiskR3, []assessment.ScoringMode{assessment.ScoringHumanPrimary})); err != nil {
		t.Fatalf("put policy: %v", err)
	}
	result, err := service.Decide(context.Background(), "tenant-a", inputFor("run-r3", snapshotFor(assessment.SubjectChinese, "extended_response", assessment.RiskR3), assessment.ScoringAIFastConfirm))
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if result.Decision != assessment.ScoringHumanPrimary || result.ExternalAIAllowed || result.Reasons[0].Code != "hard_risk_archetype_prohibition" {
		t.Fatalf("unsafe R3 decision: %#v", result)
	}
	if result.OutputConstraint.AllowModelFinalScore || result.OutputConstraint.FinalScoreAuthority != "human_review" {
		t.Fatalf("unsafe output constraint: %#v", result.OutputConstraint)
	}
}

func TestEligibilityUsesQualityEvidenceAndPersistsIdempotently(t *testing.T) {
	service := NewService(NewMemoryStore())
	input := policyFor(assessment.SubjectEnglish, "short_constructed", assessment.RiskR2, []assessment.ScoringMode{assessment.ScoringAIAssist})
	if _, err := service.PutPolicy(context.Background(), "tenant-a", input); err != nil {
		t.Fatal(err)
	}
	request := inputFor("run-quality", snapshotFor(assessment.SubjectEnglish, "short_constructed", assessment.RiskR2), assessment.ScoringAIAssist)
	request.OCRQuality, request.ParserQuality = ptr(0.8), ptr(0.99)
	denied, err := service.Decide(context.Background(), "tenant-a", request)
	if err != nil {
		t.Fatal(err)
	}
	if denied.ExternalAIAllowed || denied.Reasons[0].Code != "ocr_quality_below_policy" {
		t.Fatalf("want quality block, got %#v", denied)
	}
	request.OCRQuality = ptr(0.99)
	replayed, err := service.Decide(context.Background(), "tenant-a", request)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != denied.ID || replayed.Reasons[0].Code != "ocr_quality_below_policy" {
		t.Fatalf("decision audit must be immutable/idempotent: %#v", replayed)
	}
}

func TestEligibilityAdmitsOnlyCriteriaEvidenceAIAndPrefersMathRule(t *testing.T) {
	service := NewService(NewMemoryStore())
	policy := policyFor(assessment.SubjectMathematics, "numeric_expression", assessment.RiskR1, []assessment.ScoringMode{assessment.ScoringRuleAuto, assessment.ScoringAIAssist})
	if _, err := service.PutPolicy(context.Background(), "tenant-a", policy); err != nil {
		t.Fatal(err)
	}
	result, err := service.Decide(context.Background(), "tenant-a", inputFor("run-math", snapshotFor(assessment.SubjectMathematics, "numeric_expression", assessment.RiskR1), assessment.ScoringAIAssist))
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != assessment.ScoringRuleAuto || result.ExternalAIAllowed || result.OutputConstraint.FinalScoreAuthority != "server_rubric_and_deterministic_rule" {
		t.Fatalf("math should use rule-auto: %#v", result)
	}
	if _, err := service.PutPolicy(context.Background(), "tenant-a", policyForVersion(assessment.SubjectEnglish, "short_constructed", assessment.RiskR2, []assessment.ScoringMode{assessment.ScoringAIAssist}, 0)); err != nil {
		t.Fatal(err)
	}
	ai, err := service.Decide(context.Background(), "tenant-a", inputFor("run-ai", snapshotFor(assessment.SubjectEnglish, "short_constructed", assessment.RiskR2), assessment.ScoringAIAssist))
	if err != nil {
		t.Fatal(err)
	}
	if !ai.ExternalAIAllowed || !ai.OutputConstraint.CriteriaEvidenceOnly || ai.OutputConstraint.AllowModelFinalScore || ai.OutputConstraint.FinalScoreAuthority != "server_rubric_or_human_confirmation" {
		t.Fatalf("AI boundary was not preserved: %#v", ai)
	}
}

func TestPolicyRejectsUnsafeR3AIModes(t *testing.T) {
	service := NewService(NewMemoryStore())
	_, err := service.PutPolicy(context.Background(), "tenant-a", policyFor(assessment.SubjectChinese, "extended_response", assessment.RiskR3, []assessment.ScoringMode{assessment.ScoringAIFastConfirm}))
	if err != ErrInvalidInput {
		t.Fatalf("unsafe policy error=%v", err)
	}
}

func TestR3ExtendedMayUseAssistButNeverFastConfirmOrModelFinalScore(t *testing.T) {
	service := NewService(NewMemoryStore())
	policy := policyFor(assessment.SubjectChinese, "extended_response", assessment.RiskR3, []assessment.ScoringMode{assessment.ScoringAIAssist, assessment.ScoringHumanPrimary})
	if _, err := service.PutPolicy(context.Background(), "tenant-a", policy); err != nil {
		t.Fatal(err)
	}
	result, err := service.Decide(context.Background(), "tenant-a", inputFor("run-r3-assist", snapshotFor(assessment.SubjectChinese, "extended_response", assessment.RiskR3), assessment.ScoringAIAssist))
	if err != nil || !result.ExternalAIAllowed || result.OutputConstraint.AllowModelFinalScore || result.OutputConstraint.FinalScoreAuthority != "server_rubric_or_human_confirmation" {
		t.Fatalf("R3 assist must remain non-final: %#v err=%v", result, err)
	}
}

func TestEligibilityMatrixCoversNineSubjects(t *testing.T) {
	service := NewService(NewMemoryStore())
	for _, subject := range []assessment.SubjectCode{
		assessment.SubjectChinese, assessment.SubjectMathematics, assessment.SubjectEnglish, assessment.SubjectPhysics,
		assessment.SubjectChemistry, assessment.SubjectBiology, assessment.SubjectHistory, assessment.SubjectGeography,
		assessment.SubjectEthicsPolitics,
	} {
		if _, err := service.PutPolicy(context.Background(), "tenant-a", policyFor(subject, "short_constructed", assessment.RiskR2, []assessment.ScoringMode{assessment.ScoringAIAssist})); err != nil {
			t.Fatalf("put %s policy: %v", subject, err)
		}
		request := inputFor("run-"+string(subject), snapshotFor(subject, "short_constructed", assessment.RiskR2), assessment.ScoringAIAssist)
		allowed, err := service.Decide(context.Background(), "tenant-a", request)
		if err != nil || !allowed.ExternalAIAllowed || allowed.OutputConstraint.AllowModelFinalScore {
			t.Fatalf("subject %s decision=%#v err=%v", subject, allowed, err)
		}
	}
	blocked := inputFor("run-history-evidence", snapshotFor(assessment.SubjectHistory, "short_constructed", assessment.RiskR2), assessment.ScoringAIAssist)
	blocked.EvidenceAvailable = false
	result, err := service.Decide(context.Background(), "tenant-a", blocked)
	if err != nil || result.ExternalAIAllowed || result.Reasons[0].Code != "material_evidence_missing" {
		t.Fatalf("history without material evidence must abstain: %#v err=%v", result, err)
	}
}

func policyFor(subject assessment.SubjectCode, archetype string, risk assessment.RiskTier, modes []assessment.ScoringMode) PutPolicyInput {
	return policyForVersion(subject, archetype, risk, modes, 0)
}
func policyForVersion(subject assessment.SubjectCode, archetype string, risk assessment.RiskTier, modes []assessment.ScoringMode, expected int) PutPolicyInput {
	return PutPolicyInput{SubjectCode: subject, EducationStage: assessment.StageJunior, ArchetypeCode: archetype, RiskTier: risk,
		MinOCRQuality: 0.9, MinParserQuality: 0.9, MinEvalN: 20, MaxSevereErrorRate: 0.05, AllowedModes: modes, Status: PolicyActive, ExpectedVersion: expected}
}
func snapshotFor(subject assessment.SubjectCode, archetype string, risk assessment.RiskTier) assessment.ExamQuestionSnapshot {
	return assessment.ExamQuestionSnapshot{ID: "snapshot-1", ContentHash: "frozen", SubjectCode: subject, EducationStage: assessment.StageJunior, ArchetypeCode: archetype, RiskTier: risk, ScoringPolicySnapshot: assessment.ScoringPolicy{Mode: assessment.ScoringAIAssist}}
}
func inputFor(runID string, snapshot assessment.ExamQuestionSnapshot, mode assessment.ScoringMode) DecisionInput {
	return DecisionInput{RunItemID: runID, AssessmentSnapshot: snapshot, RequestedMode: mode, OCRQuality: ptr(0.99), ParserQuality: ptr(0.99), RubricComplete: true, EvidenceAvailable: true, Evaluation: EvaluationEvidence{Approved: true, SampleCount: 24, SevereErrorRate: 0.01, EvaluationRef: "approved-eval"}, Calibration: CalibrationEvidence{Available: true, CalibrationRef: "calibration"}}
}
func ptr(value float64) *float64 { return &value }

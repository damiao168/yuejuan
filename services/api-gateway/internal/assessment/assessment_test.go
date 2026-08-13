package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDefaultProfilesCoverNineSubjectsAcrossBothStages(t *testing.T) {
	profiles := DefaultSubjectProfiles("tenant-a")
	if len(profiles) != 18 {
		t.Fatalf("profile count=%d, want 18", len(profiles))
	}
	seen := map[string]bool{}
	for _, profile := range profiles {
		key := string(profile.EducationStage) + "/" + string(profile.SubjectCode)
		if seen[key] {
			t.Fatalf("duplicate profile %s", key)
		}
		seen[key] = true
		if !profile.EducationStage.Valid() || !profile.SubjectCode.Valid() || profile.Version != 1 {
			t.Fatalf("invalid default profile: %#v", profile)
		}
	}
	for _, subject := range []SubjectCode{
		SubjectChinese, SubjectMathematics, SubjectEnglish, SubjectPhysics,
		SubjectChemistry, SubjectBiology, SubjectHistory, SubjectGeography,
		SubjectEthicsPolitics,
	} {
		for _, stage := range []EducationStage{StageJunior, StageSenior} {
			if !seen[string(stage)+"/"+string(subject)] {
				t.Fatalf("missing %s/%s profile", stage, subject)
			}
		}
	}
}

func TestNormalizeSubjectCodeUsesCanonicalContract(t *testing.T) {
	tests := map[string]SubjectCode{
		"math":       SubjectMathematics,
		"数学":         SubjectMathematics,
		"politics":   SubjectEthicsPolitics,
		"道德与法治":      SubjectEthicsPolitics,
		"geography":  SubjectGeography,
		"  English ": SubjectEnglish,
	}
	for input, expected := range tests {
		actual, ok := NormalizeSubjectCode(input)
		if !ok || actual != expected {
			t.Fatalf("NormalizeSubjectCode(%q)=(%q,%v), want %q", input, actual, ok, expected)
		}
	}
	if _, ok := NormalizeSubjectCode("general_llm_subject"); ok {
		t.Fatal("unknown UI string must not become a subject code")
	}
}

func TestCanonicalArchetypesCarryEvidenceAndScoringDefaults(t *testing.T) {
	archetypes := DefaultQuestionArchetypes()
	if len(archetypes) != 8 {
		t.Fatalf("archetype count=%d, want 8", len(archetypes))
	}
	seen := map[string]bool{}
	for _, archetype := range archetypes {
		if !IsQuestionArchetype(archetype.Code) || seen[archetype.Code] || len(archetype.EvidenceTypes) == 0 || !archetype.DefaultScoringMode.Valid() {
			t.Fatalf("invalid archetype: %#v", archetype)
		}
		seen[archetype.Code] = true
	}
	structured, ok := memoryArchetypeByCode("structured_steps")
	if !ok || !containsEvidence(structured.EvidenceTypes, EvidenceMathStep) || !containsEvidence(structured.EvidenceTypes, EvidenceChemicalEquation) {
		t.Fatal("structured steps must support math and chemistry evidence")
	}
}

func TestR3ExtendedResponseRejectsFastConfirm(t *testing.T) {
	policy := ScoringPolicy{Mode: ScoringAIFastConfirm, RequireEvidence: true}
	if err := ValidateScoringPolicy(RiskR3, "extended_response", policy); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("error=%v, want ErrPolicyViolation", err)
	}
	if err := ValidateScoringPolicy(RiskR2, "extended_response", policy); err != nil {
		t.Fatalf("R2 policy unexpectedly rejected: %v", err)
	}
}

func TestMemoryStoreRevisionSnapshotAndEvidenceLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	profiles, err := store.ListSubjectProfiles(ctx, "tenant-a", StageJunior, SubjectChinese)
	if err != nil || len(profiles) != 1 {
		t.Fatalf("profiles=(%d,%v)", len(profiles), err)
	}
	input := ConfigureQuestionInput{
		SubjectProfileID:     profiles[0].ID,
		ArchetypeCode:        "extended_response",
		AllowedEvidenceTypes: []EvidenceType{EvidenceTextSpan, EvidenceConcept},
		RiskTier:             RiskR3,
		ScoringPolicy:        ScoringPolicy{Mode: ScoringHumanPrimary, RequireEvidence: true, HumanReviewBelowConfidence: true},
	}
	config, err := store.ConfigureQuestion(ctx, "tenant-a", "exam-a", "question-a", input)
	if err != nil || config.Revision != 1 {
		t.Fatalf("configure=(%#v,%v)", config, err)
	}
	input.ExpectedRevision = 99
	if _, err := store.ConfigureQuestion(ctx, "tenant-a", "exam-a", "question-a", input); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale update error=%v", err)
	}
	input.ExpectedRevision = 1
	config, err = store.ConfigureQuestion(ctx, "tenant-a", "exam-a", "question-a", input)
	if err != nil || config.Revision != 2 {
		t.Fatalf("update=(%#v,%v)", config, err)
	}
	snapshot, err := store.FreezeQuestionSnapshot(ctx, "tenant-a", "exam-a", "question-a")
	if err != nil || len(snapshot.ContentHash) != 64 || snapshot.SubjectCode != SubjectChinese {
		t.Fatalf("snapshot=(%#v,%v)", snapshot, err)
	}
	input.ExpectedRevision = 2
	if _, err := store.ConfigureQuestion(ctx, "tenant-a", "exam-a", "question-a", input); !errors.Is(err, ErrExamFrozen) {
		t.Fatalf("frozen update error=%v", err)
	}

	quality := 0.93
	evidence, err := store.CreateScoringEvidence(ctx, "tenant-a", CreateScoringEvidenceInput{
		SubmissionID: "submission-a", QuestionID: "question-a",
		ExamQuestionSnapshotID: snapshot.ID, EvidenceType: EvidenceTextSpan,
		SourceArtifactID: "asset-a", Payload: map[string]any{"text": "evidence"},
		BoundingBox: &BoundingBox{X: 1, Y: 2, Width: 3, Height: 4}, Quality: &quality,
	})
	if err != nil || evidence.ID == "" {
		t.Fatalf("evidence=(%#v,%v)", evidence, err)
	}
	if _, err := store.CreateScoringEvidence(ctx, "tenant-a", CreateScoringEvidenceInput{
		SubmissionID: "submission-a", QuestionID: "question-a",
		ExamQuestionSnapshotID: snapshot.ID, EvidenceType: EvidenceMathStep,
		SourceArtifactID: "asset-a",
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("disallowed evidence error=%v", err)
	}
	items, err := store.ListScoringEvidence(ctx, "tenant-a", "submission-a", "question-a")
	if err != nil || len(items) != 1 {
		t.Fatalf("evidence list=(%d,%v)", len(items), err)
	}
}

func TestMemoryStoreTenantIsolationAndSnapshotCopies(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	profiles, _ := store.ListSubjectProfiles(ctx, "tenant-a", StageSenior, SubjectMathematics)
	input := ConfigureQuestionInput{
		SubjectProfileID: profiles[0].ID, ArchetypeCode: "structured_steps",
		AllowedEvidenceTypes: []EvidenceType{EvidenceMathStep}, RiskTier: RiskR2,
		ScoringPolicy: ScoringPolicy{Mode: ScoringAIAssist, RequireEvidence: true},
	}
	if _, err := store.ConfigureQuestion(ctx, "tenant-a", "exam", "question", input); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.FreezeQuestionSnapshot(ctx, "tenant-a", "exam", "question")
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ProfileSnapshot["subject_code"] = "tampered"
	reloaded, err := store.GetQuestionSnapshot(ctx, "tenant-a", "exam", "question")
	if err != nil || reloaded.ProfileSnapshot["subject_code"] == "tampered" {
		t.Fatal("snapshot must be returned as an immutable copy")
	}
	if _, err := store.GetQuestionSnapshot(ctx, "tenant-b", "exam", "question"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant lookup error=%v", err)
	}
}

func TestAssessmentMigrationHasTenantConstraintsAndAtomicFreeze(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/000076_storyA01_assessment_domain.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := strings.Join(strings.Fields(string(raw)), " ")
	for _, expected := range []string{
		"FOREIGN KEY (tenant_id, exam_id) REFERENCES exam(tenant_id, id)",
		"FOREIGN KEY (tenant_id, question_id) REFERENCES question(tenant_id, id)",
		"FOREIGN KEY (tenant_id, submission_id) REFERENCES submission(tenant_id, id)",
		"FOREIGN KEY (tenant_id, source_artifact_id) REFERENCES file_asset(tenant_id, id)",
		"CREATE TRIGGER trg_assessment_freeze_exam_on_ready BEFORE UPDATE OF status ON exam",
		"CREATE TRIGGER trg_assessment_snapshot_immutable BEFORE UPDATE OR DELETE ON exam_question_snapshot",
		"FOREIGN KEY (tenant_id, exam_question_snapshot_id) REFERENCES exam_question_snapshot(tenant_id, id)",
		"R3 extended response cannot use AI_FAST_CONFIRM",
	} {
		if !strings.Contains(sqlText, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}
}

func TestSnapshotUnmarshalAcceptsDatabaseRowShape(t *testing.T) {
	raw := []byte(`{
      "id":"snapshot-1",
      "tenant_id":"tenant-1",
      "exam_id":"exam-1",
      "question_id":"question-1",
      "snapshot_version":1,
      "subject_profile_id":"profile-1",
      "profile_snapshot_json":{"code":"senior.chemistry.standard","education_stage":"senior","subject_code":"chemistry","version":3},
      "archetype_snapshot_json":{"code":"structured_steps"},
      "rubric_snapshot_json":{"version":"v2"},
      "scoring_policy_snapshot_json":{"mode":"AI_ASSIST","require_evidence":true,"human_review_below_confidence":true},
      "allowed_evidence_types":["chemical_equation"],
      "risk_tier":"R2",
      "content_hash":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "created_at":"2026-08-09T00:00:00Z"
    }`)
	var snapshot ExamQuestionSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.SubjectProfileCode != "senior.chemistry.standard" ||
		snapshot.SubjectProfileVersion != 3 || snapshot.EducationStage != StageSenior ||
		snapshot.SubjectCode != SubjectChemistry || snapshot.ArchetypeCode != "structured_steps" ||
		snapshot.ScoringPolicySnapshot.Mode != ScoringAIAssist {
		t.Fatalf("database row was not normalized: %#v", snapshot)
	}
}

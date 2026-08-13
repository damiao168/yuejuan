package assessment

import (
	"encoding/json"
	"time"
)

type ConfigureQuestionInput struct {
	SubjectProfileID     string         `json:"subject_profile_id"`
	ArchetypeCode        string         `json:"archetype_code"`
	AllowedEvidenceTypes []EvidenceType `json:"allowed_evidence_types"`
	RiskTier             RiskTier       `json:"risk_tier"`
	ScoringPolicy        ScoringPolicy  `json:"scoring_policy"`
	ExpectedRevision     int64          `json:"expected_revision"`
}

type QuestionAssessmentConfig struct {
	ID                    string         `json:"id"`
	TenantID              string         `json:"tenant_id"`
	ExamID                string         `json:"exam_id"`
	QuestionID            string         `json:"question_id"`
	SubjectProfileID      string         `json:"subject_profile_id"`
	SubjectProfileCode    string         `json:"subject_profile_code"`
	SubjectProfileVersion int            `json:"subject_profile_version"`
	EducationStage        EducationStage `json:"education_stage"`
	SubjectCode           SubjectCode    `json:"subject_code"`
	ArchetypeCode         string         `json:"archetype_code"`
	AllowedEvidenceTypes  []EvidenceType `json:"allowed_evidence_types"`
	RiskTier              RiskTier       `json:"risk_tier"`
	ScoringPolicy         ScoringPolicy  `json:"scoring_policy"`
	Revision              int64          `json:"revision"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type ExamQuestionSnapshot struct {
	ID                    string         `json:"id"`
	TenantID              string         `json:"tenant_id"`
	ExamID                string         `json:"exam_id"`
	QuestionID            string         `json:"question_id"`
	SnapshotVersion       int            `json:"snapshot_version"`
	SubjectProfileID      string         `json:"subject_profile_id"`
	SubjectProfileCode    string         `json:"subject_profile_code"`
	SubjectProfileVersion int            `json:"subject_profile_version"`
	EducationStage        EducationStage `json:"education_stage"`
	SubjectCode           SubjectCode    `json:"subject_code"`
	ArchetypeCode         string         `json:"archetype_code"`
	AllowedEvidenceTypes  []EvidenceType `json:"allowed_evidence_types"`
	RiskTier              RiskTier       `json:"risk_tier"`
	ProfileSnapshot       map[string]any `json:"profile_snapshot"`
	ArchetypeSnapshot     map[string]any `json:"archetype_snapshot"`
	RubricSnapshot        map[string]any `json:"rubric_snapshot"`
	ScoringPolicySnapshot ScoringPolicy  `json:"scoring_policy_snapshot"`
	ContentHash           string         `json:"content_hash"`
	CreatedAt             time.Time      `json:"created_at"`
}

// UnmarshalJSON accepts both the public API shape and PostgreSQL to_jsonb(row)
// output. Persisted snapshot columns use a _json suffix and keep profile
// identity inside profile_snapshot_json; consumers still receive one stable
// domain shape without consulting mutable profile tables.
func (s *ExamQuestionSnapshot) UnmarshalJSON(data []byte) error {
	type snapshotAlias ExamQuestionSnapshot
	aux := struct {
		*snapshotAlias
		ProfileSnapshotJSON       json.RawMessage `json:"profile_snapshot_json"`
		ArchetypeSnapshotJSON     json.RawMessage `json:"archetype_snapshot_json"`
		RubricSnapshotJSON        json.RawMessage `json:"rubric_snapshot_json"`
		ScoringPolicySnapshotJSON json.RawMessage `json:"scoring_policy_snapshot_json"`
	}{snapshotAlias: (*snapshotAlias)(s)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if err := unmarshalOptionalObject(aux.ProfileSnapshotJSON, &s.ProfileSnapshot); err != nil {
		return err
	}
	if err := unmarshalOptionalObject(aux.ArchetypeSnapshotJSON, &s.ArchetypeSnapshot); err != nil {
		return err
	}
	if err := unmarshalOptionalObject(aux.RubricSnapshotJSON, &s.RubricSnapshot); err != nil {
		return err
	}
	if len(aux.ScoringPolicySnapshotJSON) > 0 && string(aux.ScoringPolicySnapshotJSON) != "null" {
		if err := json.Unmarshal(aux.ScoringPolicySnapshotJSON, &s.ScoringPolicySnapshot); err != nil {
			return err
		}
	}
	s.deriveProfileIdentity()
	return nil
}

func unmarshalOptionalObject(raw json.RawMessage, destination *map[string]any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, destination)
}

func (s *ExamQuestionSnapshot) deriveProfileIdentity() {
	if s.SubjectProfileCode == "" {
		s.SubjectProfileCode, _ = s.ProfileSnapshot["code"].(string)
	}
	if s.EducationStage == "" {
		if value, ok := s.ProfileSnapshot["education_stage"].(string); ok {
			s.EducationStage = EducationStage(value)
		}
	}
	if s.SubjectCode == "" {
		if value, ok := s.ProfileSnapshot["subject_code"].(string); ok {
			s.SubjectCode = SubjectCode(value)
		}
	}
	if s.SubjectProfileVersion == 0 {
		if value, ok := s.ProfileSnapshot["version"].(float64); ok {
			s.SubjectProfileVersion = int(value)
		}
	}
	if s.ArchetypeCode == "" {
		s.ArchetypeCode, _ = s.ArchetypeSnapshot["code"].(string)
	}
}

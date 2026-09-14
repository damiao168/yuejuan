package questionbank

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"edugrade-enterprise/services/api-gateway/internal/assessment"
	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/paper"
	"github.com/google/uuid"
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

func validScope(s auth.AccessScope) bool { return s.TenantID != "" && s.ActorID != "" && !s.IsPlatform }
func validID(id string) bool             { _, err := uuid.Parse(id); return err == nil }
func cleanFilter(f Filter) Filter {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return f
}
func validText(value string, max int) bool {
	return utf8.RuneCountInString(value) <= max && !strings.ContainsRune(value, 0)
}
func normalizeBank(in CreateBankInput) (CreateBankInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !validID(in.SchoolID) || in.Name == "" || !validText(in.Name, 160) || !validText(in.Description, 2000) {
		return in, ErrInvalidInput
	}
	return in, nil
}
func normalizeContent(in Content) (Content, error) {
	in.Stem = strings.TrimSpace(in.Stem)
	subject, ok := assessment.NormalizeSubjectCode(in.Metadata.SubjectCode)
	if !ok || !assessment.EducationStage(in.Metadata.EducationStage).Valid() || !paper.IsValidQuestionType(in.QuestionType) || !assessment.IsQuestionArchetype(in.AssessmentArchetype) {
		return in, ErrInvalidInput
	}
	if in.Stem == "" || !validText(in.Stem, 50000) || math.IsNaN(in.DefaultScore) || math.IsInf(in.DefaultScore, 0) || in.DefaultScore <= 0 || in.DefaultScore > 100000 || math.Abs(in.DefaultScore*100-math.Round(in.DefaultScore*100)) > 1e-8 {
		return in, ErrInvalidInput
	}
	in.DefaultScore = math.Round(in.DefaultScore*100) / 100
	in.Metadata.SubjectCode = string(subject)
	in.Metadata.GradeScope = strings.TrimSpace(in.Metadata.GradeScope)
	if in.Metadata.GradeScope == "" || !validText(in.Metadata.GradeScope, 80) {
		return in, ErrInvalidInput
	}
	if in.Metadata.DifficultyBand == "" {
		in.Metadata.DifficultyBand = "unclassified"
	}
	if in.Metadata.CognitiveLevel == "" {
		in.Metadata.CognitiveLevel = "unclassified"
	}
	if in.Metadata.Copyright == "" {
		in.Metadata.Copyright = "unknown"
	}
	if in.Metadata.Language == "" {
		in.Metadata.Language = "zh-CN"
	}
	if in.Metadata.IntendedUse == "" {
		in.Metadata.IntendedUse = "practice"
	}
	if !oneOf(in.Metadata.DifficultyBand, "unclassified", "easy", "medium", "hard") || !oneOf(in.Metadata.CognitiveLevel, "unclassified", "remember", "understand", "apply", "analyze", "evaluate", "create") || !oneOf(in.Metadata.Copyright, "unknown", "owned", "licensed") || !oneOf(in.Metadata.Language, "zh-CN", "en") || !oneOf(in.Metadata.IntendedUse, "practice", "homework", "quiz", "exam", "mock_exam") {
		return in, ErrInvalidInput
	}
	if in.Metadata.SuggestedTimeMinutes != nil && (*in.Metadata.SuggestedTimeMinutes < 1 || *in.Metadata.SuggestedTimeMinutes > 1440) || in.Metadata.SourceYear != nil && (*in.Metadata.SourceYear < 1900 || *in.Metadata.SourceYear > 2200) {
		return in, ErrInvalidInput
	}
	if len(in.Options) > 32 || len(in.KnowledgePoints) > 64 {
		return in, ErrInvalidInput
	}
	// Copy arrays so neither callers nor Memory fixtures can change stored facts.
	in.Options = append([]string{}, in.Options...)
	in.KnowledgePoints = append([]string{}, in.KnowledgePoints...)
	for _, option := range in.Options {
		if strings.TrimSpace(option) == "" || !validText(option, 10000) {
			return in, ErrInvalidInput
		}
	}
	seen := map[string]bool{}
	for i, point := range in.KnowledgePoints {
		point = strings.TrimSpace(point)
		if point == "" || !validText(point, 120) || seen[point] {
			return in, ErrInvalidInput
		}
		in.KnowledgePoints[i] = point
		seen[point] = true
	}
	in.CustomMetadata = copyMetadataValues(in.CustomMetadata)
	return in, nil
}
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
func contentHash(in Content) string {
	// This is only a draft content hash, never the future published scoring bundle.
	raw, _ := json.Marshal(struct {
		SchemaVersion int     `json:"schema_version"`
		ScoreUnits    int64   `json:"score_units"`
		Content       Content `json:"content"`
	}{1, int64(math.Round(in.DefaultScore * 100)), in})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

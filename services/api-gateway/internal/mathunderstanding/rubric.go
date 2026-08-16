package mathunderstanding

import (
	"fmt"

	"edugrade-enterprise/services/api-gateway/internal/paper"
)

type EvidenceRequirement struct {
	Type      string                `json:"type"`
	Criterion string                `json:"criterion,omitempty"`
	Target    string                `json:"target,omitempty"`
	Minimum   int                   `json:"minimum,omitempty"`
	Children  []EvidenceRequirement `json:"children,omitempty"`
}

type EvidenceMatchInput struct {
	Requirements  []EvidenceRequirement
	Graph         SolutionGraph
	Verifications []MathVerification
	Concepts      []string
	Units         []string
	Domains       []string
}

func RequirementsFromRubric(points []paper.RubricPoint) []EvidenceRequirement {
	out := []EvidenceRequirement{}
	for _, point := range points {
		for _, requirement := range point.EvidenceRequirements {
			out = append(out, evidenceRequirementFromPaper(point.ID, requirement))
		}
	}
	return out
}

func evidenceRequirementFromPaper(criterion string, requirement paper.EvidenceRequirement) EvidenceRequirement {
	out := EvidenceRequirement{Type: requirement.Type, Criterion: criterion, Target: requirement.Target, Minimum: requirement.Minimum}
	for _, child := range requirement.Children {
		out.Children = append(out.Children, evidenceRequirementFromPaper(criterion, child))
	}
	return out
}

// MatchRubricEvidence creates evidence assertions only. It does not calculate
// or persist a score and therefore continues to use the existing grading flow.
func MatchRubricEvidence(input EvidenceMatchInput) ([]RubricEvidence, error) {
	out := make([]RubricEvidence, 0, len(input.Requirements))
	for index, requirement := range input.Requirements {
		if requirement.Criterion == "" {
			return nil, ErrInvalidInput
		}
		status, sources := matchRequirement(requirement, input)
		confidence := input.Graph.OverallConfidence
		if status == "uncertain" {
			confidence = min(confidence, .49)
		}
		out = append(out, RubricEvidence{ID: fmt.Sprintf("rubric-evidence-%d", index+1), RubricCriterionKey: requirement.Criterion, EvidenceType: requirement.Type, SourceArtifactIDs: sources, Status: status, Confidence: confidence})
	}
	return out, nil
}

func matchRequirement(requirement EvidenceRequirement, input EvidenceMatchInput) (string, []string) {
	switch requirement.Type {
	case "all_of", "any_of", "at_least":
		supported, uncertain, sources := 0, 0, []string{}
		for _, child := range requirement.Children {
			child.Criterion = requirement.Criterion
			status, refs := matchRequirement(child, input)
			sources = append(sources, refs...)
			if status == "supported" {
				supported++
			} else if status == "uncertain" {
				uncertain++
			}
		}
		needed := len(requirement.Children)
		if requirement.Type == "any_of" {
			needed = 1
		}
		if requirement.Type == "at_least" {
			needed = requirement.Minimum
		}
		if needed <= 0 {
			return "uncertain", sources
		}
		if supported >= needed {
			return "supported", sources
		}
		if supported+uncertain >= needed {
			return "uncertain", sources
		}
		return "unsupported", sources
	case "valid_transformation":
		for _, check := range input.Verifications {
			if check.Kind == "equivalence" && check.Status == "verified" {
				return "supported", []string{check.ID}
			}
		}
		return "uncertain", nil
	case "final_result":
		for _, check := range input.Verifications {
			if check.Status == "verified" && (check.Kind == "equivalence" || check.Kind == "substitution") {
				return "supported", []string{check.ID}
			}
		}
		return "uncertain", nil
	case "concept":
		return membershipStatus(requirement.Target, input.Concepts)
	case "unit":
		return membershipStatus(requirement.Target, input.Units)
	case "domain":
		return membershipStatus(requirement.Target, input.Domains)
	default:
		return "uncertain", nil
	}
}

func membershipStatus(target string, values []string) (string, []string) {
	for _, value := range values {
		if value == target {
			return "supported", []string{"evidence:" + value}
		}
	}
	return "uncertain", nil
}
func min(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

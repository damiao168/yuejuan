package releasegate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"edugrade-enterprise/services/api-gateway/internal/scorerelease"
)

type Service struct {
	store    Store
	base     BaseGateReader
	regrades RegradeBlockerReader
	now      func() time.Time
}

func NewService(store Store, base BaseGateReader) *Service {
	return &Service{store: store, base: base, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) WithRegradeBlockerReader(reader RegradeBlockerReader) *Service {
	s.regrades = reader
	return s
}

func (s *Service) CreatePolicy(ctx context.Context, tenantID, examID, actorID string, input CreatePolicyInput) (Policy, error) {
	if !validIDs(tenantID, examID, actorID) || !normalizePolicy(&input) {
		return Policy{}, ErrInvalidInput
	}
	return s.store.CreatePolicy(ctx, tenantID, examID, actorID, input)
}

// Preview recalculates the A18 base gate and captures immutable evidence. It
// is intentionally useful even when the gate is blocked: it gives operations
// a precise, non-mutable reason for remediation or a permitted warning waiver.
func (s *Service) Preview(ctx context.Context, tenantID, examID, releaseID, actorID string) (Evidence, error) {
	if !validIDs(tenantID, examID, actorID) {
		return Evidence{}, ErrInvalidInput
	}
	return s.evaluateAndAppend(ctx, tenantID, examID, strings.TrimSpace(releaseID), actorID, EvidencePreview)
}

// RecheckForPublish is the only gate API a publication coordinator should
// call. It never reuses Preview results and always appends fresh evidence.
func (s *Service) RecheckForPublish(ctx context.Context, tenantID, examID, releaseID, actorID string) (Evidence, error) {
	if !validIDs(tenantID, examID, releaseID, actorID) {
		return Evidence{}, ErrInvalidInput
	}
	evidence, err := s.evaluateAndAppend(ctx, tenantID, examID, releaseID, actorID, EvidencePublish)
	if err != nil {
		return Evidence{}, err
	}
	if !evidence.Evaluation.Passed {
		return evidence, ErrGateBlocked
	}
	return evidence, nil
}

func (s *Service) RequestWaiver(ctx context.Context, tenantID, examID, actorID string, input RequestWaiverInput) (Waiver, error) {
	input.EvidenceID, input.IssueCode, input.Reason = strings.TrimSpace(input.EvidenceID), strings.TrimSpace(input.IssueCode), strings.TrimSpace(input.Reason)
	if !validIDs(tenantID, examID, actorID, input.EvidenceID, input.IssueCode) || utf8.RuneCountInString(input.Reason) == 0 || utf8.RuneCountInString(input.Reason) > 1000 {
		return Waiver{}, ErrInvalidInput
	}
	evidence, err := s.store.GetEvidence(ctx, tenantID, input.EvidenceID)
	if err != nil {
		return Waiver{}, err
	}
	if evidence.ExamID != examID || evidence.Phase != EvidencePreview || !warningCanBeWaived(evidence.Evaluation, input.IssueCode) {
		return Waiver{}, ErrForbiddenWaiver
	}
	return s.store.RequestWaiver(ctx, Waiver{TenantID: tenantID, ExamID: examID, PolicyID: evidence.Evaluation.Policy.ID,
		EvidenceID: evidence.ID, IssueCode: input.IssueCode, Reason: input.Reason, Status: WaiverRequested,
		RequestedBy: actorID, RequestedAt: s.now().UTC()})
}

func (s *Service) DecideWaiver(ctx context.Context, tenantID, waiverID, actorID string, input DecideWaiverInput) (Waiver, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if !validIDs(tenantID, waiverID, actorID) || utf8.RuneCountInString(input.Reason) == 0 || utf8.RuneCountInString(input.Reason) > 1000 {
		return Waiver{}, ErrInvalidInput
	}
	waiver, err := s.store.GetWaiver(ctx, tenantID, waiverID)
	if err != nil {
		return Waiver{}, err
	}
	if waiver.Status != WaiverRequested {
		return Waiver{}, ErrForbiddenWaiver
	}
	// Revalidate against immutable evidence immediately before accepting an
	// approval, so neither a malicious request nor a later policy edit can
	// turn a base blocker into a waiver.
	evidence, err := s.store.GetEvidence(ctx, tenantID, waiver.EvidenceID)
	if err != nil {
		return Waiver{}, err
	}
	if waiver.ExamID != evidence.ExamID || !warningCanBeWaived(evidence.Evaluation, waiver.IssueCode) {
		return Waiver{}, ErrForbiddenWaiver
	}
	return s.store.DecideWaiver(ctx, tenantID, waiverID, actorID, input)
}

func (s *Service) GetEvidence(ctx context.Context, tenantID, evidenceID string) (Evidence, error) {
	if !validIDs(tenantID, evidenceID) {
		return Evidence{}, ErrInvalidInput
	}
	return s.store.GetEvidence(ctx, tenantID, evidenceID)
}

func (s *Service) evaluateAndAppend(ctx context.Context, tenantID, examID, releaseID, actorID, phase string) (Evidence, error) {
	base, err := s.base.Gate(ctx, tenantID, examID)
	if err != nil {
		return Evidence{}, err
	}
	if s.regrades != nil {
		count, err := s.regrades.BlockingRegradeCount(ctx, tenantID, examID)
		if err != nil {
			return Evidence{}, err
		}
		if count > 0 {
			base.Blocking = append(base.Blocking, scorerelease.GateIssue{Code: "regrade_release_pending", Message: "a regrade job has not been materialised into a successor release", Blocking: true, Count: count, ActionRoute: "regrade"})
			if base.Counts == nil {
				base.Counts = map[string]int{}
			}
			base.Counts["regrade_release_pending"] += count
			base.Passed = false
		}
	}
	policy, err := s.store.ActivePolicy(ctx, tenantID, examID)
	if err != nil && err != ErrNotFound {
		return Evidence{}, err
	}
	if err == ErrNotFound {
		policy = defaultPolicy(tenantID, examID)
	}
	waivers, err := s.store.ApprovedWaivers(ctx, tenantID, examID, policy.ID)
	if err != nil {
		return Evidence{}, err
	}
	evaluation := evaluate(policy, base, waivers, s.now().UTC())
	evidence := Evidence{TenantID: tenantID, ExamID: examID, ReleaseID: releaseID, Phase: phase, Evaluation: evaluation, CreatedBy: actorID, CreatedAt: s.now().UTC()}
	evidence.Hash = evidenceHash(evidence)
	return s.store.AppendEvidence(ctx, evidence)
}

func evaluate(policy Policy, baseGate scorerelease.Gate, waivers []Waiver, now time.Time) Evaluation {
	result := Evaluation{Policy: clonePolicy(policy), BaseGate: cloneGate(baseGate), Passed: len(baseGate.Blocking) == 0,
		PendingWarningCodes: []string{}, AppliedWaiverIDs: []string{}, GeneratedAt: now.UTC()}
	approved := map[string]string{}
	for _, waiver := range waivers {
		if waiver.Status == WaiverApproved {
			approved[waiver.IssueCode] = waiver.ID
		}
	}
	if !policy.RequireWarningAcknowledgement {
		return result
	}
	seen := map[string]bool{}
	for _, issue := range baseGate.Warnings {
		if seen[issue.Code] || !policyAllowsWarning(policy, issue.Code) {
			continue
		}
		seen[issue.Code] = true
		if waiverID := approved[issue.Code]; waiverID != "" {
			result.AppliedWaiverIDs = append(result.AppliedWaiverIDs, waiverID)
			continue
		}
		result.PendingWarningCodes = append(result.PendingWarningCodes, issue.Code)
	}
	sort.Strings(result.PendingWarningCodes)
	sort.Strings(result.AppliedWaiverIDs)
	result.Passed = result.Passed && len(result.PendingWarningCodes) == 0
	return result
}

func warningCanBeWaived(evaluation Evaluation, code string) bool {
	if !policyAllowsWarning(evaluation.Policy, code) {
		return false
	}
	for _, issue := range evaluation.BaseGate.Blocking {
		if issue.Code == code {
			return false
		}
	}
	for _, issue := range evaluation.BaseGate.Warnings {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func policyAllowsWarning(policy Policy, code string) bool {
	for _, candidate := range policy.WaivableWarningCodes {
		if candidate == code {
			return true
		}
	}
	return false
}

func defaultPolicy(tenantID, examID string) Policy {
	return Policy{TenantID: tenantID, ExamID: examID, Version: DefaultPolicyVersion, Status: PolicyActive,
		WaivableWarningCodes: []string{}}
}

func normalizePolicy(input *CreatePolicyInput) bool {
	input.Version = strings.TrimSpace(input.Version)
	if utf8.RuneCountInString(input.Version) < 3 || utf8.RuneCountInString(input.Version) > 120 {
		return false
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(input.WaivableWarningCodes))
	for _, code := range input.WaivableWarningCodes {
		code = strings.TrimSpace(code)
		if code == "" || utf8.RuneCountInString(code) > 120 || seen[code] {
			return false
		}
		seen[code] = true
		clean = append(clean, code)
	}
	sort.Strings(clean)
	input.WaivableWarningCodes = clean
	// Without acknowledgement a waivable list has no operational meaning and
	// would make a policy review misleading.
	return input.RequireWarningAcknowledgement || len(clean) == 0
}

func evidenceHash(evidence Evidence) string {
	// Identity/timestamps are excluded so the hash proves the immutable gate
	// payload, not a particular storage implementation's identifier format.
	payload := struct {
		ExamID     string     `json:"exam_id"`
		ReleaseID  string     `json:"release_id"`
		Phase      string     `json:"phase"`
		Evaluation Evaluation `json:"evaluation"`
	}{evidence.ExamID, evidence.ReleaseID, evidence.Phase, evidence.Evaluation}
	bytes, _ := json.Marshal(payload)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func clonePolicy(value Policy) Policy {
	value.WaivableWarningCodes = append([]string(nil), value.WaivableWarningCodes...)
	return value
}

func cloneGate(value scorerelease.Gate) scorerelease.Gate {
	value.Blocking = append([]scorerelease.GateIssue(nil), value.Blocking...)
	value.Warnings = append([]scorerelease.GateIssue(nil), value.Warnings...)
	value.Counts = map[string]int{}
	for key, count := range value.Counts {
		value.Counts[key] = count
	}
	return value
}

func validIDs(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

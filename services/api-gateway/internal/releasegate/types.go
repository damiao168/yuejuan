// Package releasegate owns the policy and evidence layer around A18 score
// releases.  It deliberately does not create or mutate a score release: A18
// remains the single public-score boundary.  This package records which
// versioned quality policy was applied, any allowed warning acknowledgement,
// and the immutable result of each pre-publish recomputation.
package releasegate

import (
	"context"
	"errors"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/scorerelease"
)

var (
	ErrNotFound          = errors.New("release gate resource not found")
	ErrInvalidInput      = errors.New("invalid release gate input")
	ErrForbiddenWaiver   = errors.New("release gate waiver is not allowed")
	ErrGateBlocked       = errors.New("release gate is blocked")
	ErrEvidenceImmutable = errors.New("release gate evidence is immutable")
)

const (
	PolicyActive    = "active"
	PolicyRetired   = "retired"
	EvidencePreview = "preview"
	EvidencePublish = "publish"
	WaiverRequested = "requested"
	WaiverApproved  = "approved"
	WaiverRejected  = "rejected"

	DefaultPolicyVersion = "A20.core.v1"
)

// Policy is deliberately narrow.  The base blockers are calculated by A18;
// this versioned policy may require acknowledgement for explicitly named
// warnings, but can never downgrade a blocker.
type Policy struct {
	ID                            string    `json:"id"`
	TenantID                      string    `json:"tenant_id"`
	ExamID                        string    `json:"exam_id"`
	Version                       string    `json:"version"`
	Status                        string    `json:"status"`
	RequireWarningAcknowledgement bool      `json:"require_warning_acknowledgement"`
	WaivableWarningCodes          []string  `json:"waivable_warning_codes"`
	CreatedBy                     string    `json:"created_by"`
	CreatedAt                     time.Time `json:"created_at"`
}

// Evaluation is the effective gate after policy. BaseGate is retained in
// full, including every blocker. PendingWarningCodes are a policy decision
// layer and never cause a source blocker to become waivable.
type Evaluation struct {
	Policy              Policy            `json:"policy"`
	BaseGate            scorerelease.Gate `json:"base_gate"`
	Passed              bool              `json:"passed"`
	PendingWarningCodes []string          `json:"pending_warning_codes"`
	AppliedWaiverIDs    []string          `json:"applied_waiver_ids"`
	GeneratedAt         time.Time         `json:"generated_at"`
}

// Evidence is append-only forensic evidence. A preview gives an
// administrator a stable object against which to request a warning waiver;
// a publish evidence row is always recomputed immediately before publication.
type Evidence struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	ExamID     string     `json:"exam_id"`
	ReleaseID  string     `json:"release_id,omitempty"`
	Phase      string     `json:"phase"`
	Evaluation Evaluation `json:"evaluation"`
	Hash       string     `json:"hash"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Waiver is only a request against a warning in a specific immutable preview
// evidence row. The approval is a separate append-only decision in storage;
// callers only see the current decision projection.
type Waiver struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	ExamID         string     `json:"exam_id"`
	PolicyID       string     `json:"policy_id,omitempty"`
	EvidenceID     string     `json:"evidence_id"`
	IssueCode      string     `json:"issue_code"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	RequestedBy    string     `json:"requested_by"`
	RequestedAt    time.Time  `json:"requested_at"`
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecisionReason string     `json:"decision_reason,omitempty"`
}

type CreatePolicyInput struct {
	Version                       string   `json:"version"`
	RequireWarningAcknowledgement bool     `json:"require_warning_acknowledgement"`
	WaivableWarningCodes          []string `json:"waivable_warning_codes"`
}

type RequestWaiverInput struct {
	EvidenceID string `json:"evidence_id"`
	IssueCode  string `json:"issue_code"`
	Reason     string `json:"reason"`
}

type DecideWaiverInput struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

// BaseGateReader is A18's canonical safety checklist. Implementations must
// calculate from source facts, not accept a browser-supplied gate.
type BaseGateReader interface {
	Gate(context.Context, string, string) (scorerelease.Gate, error)
}

// RegradeBlockerReader exposes only the release-relevant state of A19.
// Pending correction plans must not be bypassed by publishing another version.
type RegradeBlockerReader interface {
	BlockingRegradeCount(context.Context, string, string) (int, error)
}

type Store interface {
	CreatePolicy(context.Context, string, string, string, CreatePolicyInput) (Policy, error)
	ActivePolicy(context.Context, string, string) (Policy, error)
	GetEvidence(context.Context, string, string) (Evidence, error)
	AppendEvidence(context.Context, Evidence) (Evidence, error)
	RequestWaiver(context.Context, Waiver) (Waiver, error)
	GetWaiver(context.Context, string, string) (Waiver, error)
	DecideWaiver(context.Context, string, string, string, DecideWaiverInput) (Waiver, error)
	ApprovedWaivers(context.Context, string, string, string) ([]Waiver, error)
}

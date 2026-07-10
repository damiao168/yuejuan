package orchestrator

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("orchestrator resource not found")
	ErrInvalidInput      = errors.New("invalid orchestrator input")
	ErrInvalidTransition = errors.New("invalid orchestrator status transition")
	ErrRetryExhausted    = errors.New("agent task retry exhausted")
)

type Run struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	WorkflowType string    `json:"workflow_type"`
	TargetType   string    `json:"target_type"`
	TargetID     string    `json:"target_id"`
	Status       string    `json:"status"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	Tasks        []Task    `json:"tasks,omitempty"`
}

type Task struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	OrchestrationRunID string         `json:"orchestration_run_id"`
	AgentType          string         `json:"agent_type"`
	Status             string         `json:"status"`
	InputRef           map[string]any `json:"input_ref"`
	OutputRef          map[string]any `json:"output_ref"`
	EvidenceRef        map[string]any `json:"evidence_ref"`
	Confidence         *float64       `json:"confidence,omitempty"`
	AttemptNo          int            `json:"attempt_no"`
	MaxAttempts        int            `json:"max_attempts"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	CreatedBy          string         `json:"created_by"`
	StartedAt          *time.Time     `json:"started_at,omitempty"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

type CreateRunInput struct {
	WorkflowType string `json:"workflow_type"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
}

type CreateTaskInput struct {
	AgentType   string         `json:"agent_type"`
	InputRef    map[string]any `json:"input_ref"`
	MaxAttempts int            `json:"max_attempts"`
}

type CompleteTaskInput struct {
	OutputRef           map[string]any `json:"output_ref"`
	EvidenceRef         map[string]any `json:"evidence_ref"`
	Confidence          *float64       `json:"confidence"`
	RequiresHumanReview bool           `json:"requires_human_review"`
}

type FailTaskInput struct {
	ErrorMessage string `json:"error_message"`
}

type Store interface {
	CreateRun(ctx context.Context, tenantID string, actorID string, input CreateRunInput) (Run, error)
	GetRun(ctx context.Context, tenantID string, id string) (Run, error)
	ListTasks(ctx context.Context, tenantID string, runID string) ([]Task, error)
	CreateTask(ctx context.Context, tenantID string, runID string, actorID string, input CreateTaskInput) (Task, error)
	StartTask(ctx context.Context, tenantID string, id string) (Task, error)
	CompleteTask(ctx context.Context, tenantID string, id string, input CompleteTaskInput) (Task, error)
	FailTask(ctx context.Context, tenantID string, id string, errorMessage string) (Task, error)
	RetryTask(ctx context.Context, tenantID string, id string) (Task, error)
}

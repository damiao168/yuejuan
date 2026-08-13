package review

import (
	"context"
	"time"
)

var _ TaskContextStore = (*MemoryStore)(nil)

func (s *MemoryStore) GetTaskContext(ctx context.Context, tenantID, taskID string) (TaskContext, error) {
	workspace, err := s.GetWorkspace(ctx, tenantID, taskID)
	if err != nil {
		return TaskContext{}, err
	}
	out := taskContextFromWorkspace(workspace)
	out.Claim.CanRenew = workspace.Task.AssignedTo != "" && isClaimableAssignedStatus(workspace.Task.Status)

	s.mu.RLock()
	claim, claimed := s.claims[key(tenantID, taskID)]
	s.mu.RUnlock()
	if claimed && claim.OwnerID == workspace.Task.AssignedTo {
		claimedAt := claim.ClaimedAt
		expiresAt := claim.ExpiresAt
		out.Claim.ClaimedAt = &claimedAt
		out.Claim.ExpiresAt = &expiresAt
		if expiresAt.After(time.Now().UTC()) {
			out.Claim.State = "claimed"
		} else {
			out.Claim.State = "expired"
		}
	}
	return out, nil
}

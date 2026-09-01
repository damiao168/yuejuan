package mathunderstanding

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"edugrade-enterprise/services/api-gateway/internal/auth"
)

type assignmentLookupStub struct {
	allowed bool
	err     error
	calls   int
}

func (s *assignmentLookupStub) HasActiveAssignment(_ context.Context, _, _, _ string) (bool, error) {
	s.calls++
	return s.allowed, s.err
}

func TestMathEvidenceAccessUsesExactAssignmentLookup(t *testing.T) {
	lookup := &assignmentLookupStub{allowed: true}
	handler := NewHandler(nil, nil, nil, lookup, nil)
	request := httptest.NewRequest("GET", "/", nil)
	if !handler.canAccess(request, auth.User{ID: "reviewer-a", TenantID: "tenant-a"}, "segment-501") {
		t.Fatal("exact active assignment was rejected")
	}
	if lookup.calls != 1 {
		t.Fatalf("expected one exact lookup, got %d", lookup.calls)
	}
}

func TestMathEvidenceAccessFailsClosedOnAssignmentLookupError(t *testing.T) {
	lookup := &assignmentLookupStub{allowed: true, err: errors.New("database unavailable")}
	handler := NewHandler(nil, nil, nil, lookup, nil)
	request := httptest.NewRequest("GET", "/", nil)
	if handler.canAccess(request, auth.User{ID: "reviewer-a", TenantID: "tenant-a"}, "segment-a") {
		t.Fatal("store error expanded math evidence access")
	}
}

func TestMathEvidenceManagerBypassDoesNotQueryAssignments(t *testing.T) {
	lookup := &assignmentLookupStub{err: errors.New("must not be called")}
	handler := NewHandler(nil, nil, nil, lookup, nil)
	request := httptest.NewRequest("GET", "/", nil)
	if !handler.canAccess(request, auth.User{ID: "manager", TenantID: "tenant-a", Permissions: []string{"review:manage"}}, "segment-a") {
		t.Fatal("review manager bypass changed")
	}
	if lookup.calls != 0 {
		t.Fatalf("manager bypass unexpectedly queried assignments %d times", lookup.calls)
	}
}

package outbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testStore struct {
	events     []Event
	published  []string
	failed     []string
	retryDelay time.Duration
}

func (s *testStore) Claim(context.Context, string, int, time.Duration) ([]Event, error) {
	return append([]Event(nil), s.events...), nil
}

func (s *testStore) MarkPublished(_ context.Context, id, _ string) error {
	s.published = append(s.published, id)
	return nil
}

func (s *testStore) MarkFailed(_ context.Context, id, _, _ string, delay time.Duration) error {
	s.failed = append(s.failed, id)
	s.retryDelay = delay
	return nil
}

type testPublisher struct{ failID string }

func (p testPublisher) Publish(_ context.Context, event Event) error {
	if event.ID == p.failID {
		return errors.New("sink unavailable")
	}
	return nil
}

func TestDispatcherPublishesBatchAndSchedulesFailure(t *testing.T) {
	store := &testStore{events: []Event{{ID: "ok", AttemptCount: 1}, {ID: "retry", AttemptCount: 3}}}
	dispatcher := NewDispatcher(store, testPublisher{failID: "retry"}, Options{
		Owner: "test", BaseBackoff: time.Second, MaxBackoff: time.Minute,
	})
	count, err := dispatcher.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}
	if count != 1 || len(store.published) != 1 || store.published[0] != "ok" {
		t.Fatalf("unexpected published events: count=%d ids=%v", count, store.published)
	}
	if len(store.failed) != 1 || store.failed[0] != "retry" || store.retryDelay != 4*time.Second {
		t.Fatalf("failure was not scheduled with exponential backoff: ids=%v delay=%s", store.failed, store.retryDelay)
	}
}

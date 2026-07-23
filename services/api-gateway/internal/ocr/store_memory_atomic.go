package ocr

import "context"

type memoryTx struct {
	store *MemoryStore
}

type memorySnapshot struct {
	next       int
	tasks      map[string]Task
	results    map[string][]Result
	idempotent map[string]string
}

// runAtomic holds the source store lock and restores all source facts on an
// error or panic. The runtime coordinator nests Worker Runtime's equivalent
// transaction so neither side can become visible alone.
func (s *MemoryStore) runAtomic(fn func(*memoryTx) error) (err error) {
	if fn == nil {
		return ErrInvalidInput
	}
	s.mu.Lock()
	snapshot := s.snapshotLocked()
	committed := false
	defer func() {
		if !committed {
			s.restoreLocked(snapshot)
		}
		s.mu.Unlock()
	}()
	if err := fn(&memoryTx{store: s}); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *MemoryStore) snapshotLocked() memorySnapshot {
	tasks := make(map[string]Task, len(s.tasks))
	for id, task := range s.tasks {
		tasks[id] = cloneOCRTask(task)
	}
	results := make(map[string][]Result, len(s.results))
	for id, values := range s.results {
		results[id] = cloneOCRResults(values)
	}
	idempotent := make(map[string]string, len(s.idempotent))
	for key, id := range s.idempotent {
		idempotent[key] = id
	}
	return memorySnapshot{next: s.next, tasks: tasks, results: results, idempotent: idempotent}
}

func (s *MemoryStore) restoreLocked(snapshot memorySnapshot) {
	s.next = snapshot.next
	s.tasks = snapshot.tasks
	s.results = snapshot.results
	s.idempotent = snapshot.idempotent
}

func (tx *memoryTx) CreateTask(ctx context.Context, tenantID, submissionID, actorID string, input CreateTaskInput) (Task, error) {
	return tx.store.createTaskLocked(ctx, tenantID, submissionID, actorID, input)
}

func (tx *memoryTx) GetTask(_ context.Context, tenantID, taskID string) (Task, error) {
	return tx.store.getTaskLocked(tenantID, taskID)
}

func (tx *memoryTx) CompleteTask(ctx context.Context, tenantID, taskID string, input CompleteTaskInput) (Task, error) {
	return tx.store.completeTaskLocked(ctx, tenantID, taskID, input)
}

func (tx *memoryTx) FailTask(ctx context.Context, tenantID, taskID, errorMessage string) (Task, error) {
	return tx.store.failTaskLocked(ctx, tenantID, taskID, errorMessage)
}

func cloneOCRTask(task Task) Task {
	task.Results = cloneOCRResults(task.Results)
	return task
}

func cloneOCRResults(values []Result) []Result {
	out := make([]Result, len(values))
	for index, value := range values {
		out[index] = value
		out[index].BBox = append([]float64{}, value.BBox...)
	}
	return out
}

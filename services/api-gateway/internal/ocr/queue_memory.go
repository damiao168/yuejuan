package ocr

import (
	"context"
	"sync"
)

type MemoryQueue struct {
	mu    sync.RWMutex
	tasks []Task
}

func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{tasks: []Task{}}
}

func (q *MemoryQueue) Enqueue(_ context.Context, task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, task)
	return nil
}

func (q *MemoryQueue) Tasks() []Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]Task, len(q.tasks))
	copy(out, q.tasks)
	return out
}

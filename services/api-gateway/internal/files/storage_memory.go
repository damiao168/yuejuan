package files

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type MemoryObjectStorage struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMemoryObjectStorage() *MemoryObjectStorage {
	return &MemoryObjectStorage{objects: map[string][]byte{}}
}

func (s *MemoryObjectStorage) Put(_ context.Context, bucket string, key string, body io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[bucket+"/"+key] = data
	return nil
}

func (s *MemoryObjectStorage) Get(_ context.Context, bucket string, key string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.objects[bucket+"/"+key]
	if !ok {
		return nil, ErrStorageFailure
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *MemoryObjectStorage) Remove(_ context.Context, bucket string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, bucket+"/"+key)
	return nil
}

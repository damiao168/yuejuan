package files

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"sync"
)

type MemoryObjectStorage struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func (s *MemoryObjectStorage) Stat(_ context.Context, bucket string, key string) (ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.objects[bucket+"/"+key]
	if !ok {
		return ObjectInfo{}, ErrObjectNotFound
	}
	return ObjectInfo{Bucket: bucket, Key: key, SizeBytes: int64(len(data))}, nil
}

func (s *MemoryObjectStorage) List(_ context.Context, bucket string, prefix string, limit int) ([]ObjectInfo, error) {
	if limit <= 0 {
		limit = 1000
	}
	storagePrefix := bucket + "/" + prefix
	s.mu.RLock()
	keys := make([]string, 0)
	for storedKey := range s.objects {
		if strings.HasPrefix(storedKey, storagePrefix) {
			keys = append(keys, strings.TrimPrefix(storedKey, bucket+"/"))
		}
	}
	s.mu.RUnlock()
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	objects := make([]ObjectInfo, 0, len(keys))
	for _, key := range keys {
		info, err := s.Stat(context.Background(), bucket, key)
		if err != nil {
			continue
		}
		objects = append(objects, info)
	}
	return objects, nil
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

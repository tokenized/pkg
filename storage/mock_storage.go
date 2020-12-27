package storage

import (
	"context"
	"strings"
)

// MockStorage implements the Storage interface for but just holds the data in memory.
type MockStorage struct {
	Data map[string][]byte
}

// MockStorage creates a new mock storage.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		Data: make(map[string][]byte),
	}
}

// Write will write the data to the key in the S3 Bucket.
func (s *MockStorage) Write(ctx context.Context, key string, body []byte, options *Options) error {
	s.Data[key] = body
	return nil
}

// Read reads the data from a file on the local filesystem.
func (s *MockStorage) Read(ctx context.Context, key string) ([]byte, error) {
	result, exists := s.Data[key]
	if !exists {
		return nil, ErrNotFound
	}
	return result, nil
}

// Remove removes the object stored at key, in the S3 Bucket.
func (s *MockStorage) Remove(ctx context.Context, key string) error {
	_, exists := s.Data[key]
	if !exists {
		return ErrNotFound
	}
	delete(s.Data, key)
	return nil
}

// All returns all objects in the store, from a given path.
//
// The path can be empty.
func (s *MockStorage) Search(ctx context.Context, query map[string]string) ([][]byte, error) {
	result := make([][]byte, 0)
	path := query["path"]

	for key, b := range s.Data {
		if !strings.HasPrefix(key, path) {
			continue
		}

		result = append(result, b)
	}

	return result, nil
}

func (s *MockStorage) Clear(ctx context.Context, query map[string]string) error {
	path := query["path"]

	toRemove := make([]string, 0)
	for key, _ := range s.Data {
		if !strings.HasPrefix(key, path) {
			continue
		}

		toRemove = append(toRemove, key)
	}

	for _, key := range toRemove {
		delete(s.Data, key)
	}

	return nil
}

func (s *MockStorage) List(ctx context.Context, path string) ([]string, error) {
	result := make([]string, 0)

	for key, _ := range s.Data {
		if !strings.HasPrefix(key, path) {
			continue
		}

		result = append(result, key)
	}

	return result, nil
}

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// MockStorage implements the Storage interface for but just holds the data in memory.
type MockStorage struct {
	Data map[string][]byte

	readDelay time.Duration

	sync.Mutex
}

// MockStorage creates a new mock storage.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		Data: make(map[string][]byte),
	}
}

func (s *MockStorage) SetReadDelay(delay time.Duration) {
	s.Lock()
	defer s.Unlock()

	s.readDelay = delay
}

// Write will write the data to the key in the S3 Bucket.
func (s *MockStorage) Write(ctx context.Context, key string, body []byte, options *Options) error {
	s.Lock()
	defer s.Unlock()

	s.Data[key] = body
	return nil
}

func (s *MockStorage) StreamWrite(ctx context.Context, key string, r io.ReadSeeker) error {
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, r); err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.Data[key] = buf.Bytes()
	return nil
}

// Read reads the data from a file on the local filesystem.
func (s *MockStorage) Read(ctx context.Context, key string) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	if s.readDelay > 0 {
		time.Sleep(s.readDelay)
	}

	result, exists := s.Data[key]
	if !exists {
		return nil, ErrNotFound
	}
	return result, nil
}

func (s *MockStorage) ReadRange(ctx context.Context, key string, start, end int64) ([]byte, error) {
	r, err := s.StreamReadRange(ctx, key, start, end)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *MockStorage) StreamRead(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.StreamReadRange(ctx, key, 0, 0)
}

func (s *MockStorage) StreamReadRange(ctx context.Context, key string,
	start, end int64) (io.ReadCloser, error) {
	s.Lock()
	defer s.Unlock()

	if s.readDelay > 0 {
		time.Sleep(s.readDelay)
	}

	result, exists := s.Data[key]
	if !exists {
		return nil, ErrNotFound
	}

	if start != 0 && start > int64(len(result)) {
		return nil, fmt.Errorf("Start offset past end: offset: %d, end: %d", start, len(result))
	}

	if end != 0 && end > int64(len(result)) {
		return nil, fmt.Errorf("End offset past end: offset: %d, end: %d", end, len(result))
	}

	if end == 0 {
		end = int64(len(result))
	}

	return &nopCloser{
		r: bytes.NewReader(result[start:end]),
	}, nil
}

type nopCloser struct {
	r io.Reader
}

func (rc *nopCloser) Read(b []byte) (int, error) {
	return rc.r.Read(b)
}

func (rc *nopCloser) Close() error {
	return nil
}

// Remove removes the object stored at key, in the S3 Bucket.
func (s *MockStorage) Remove(ctx context.Context, key string) error {
	s.Lock()
	defer s.Unlock()

	_, exists := s.Data[key]
	if !exists {
		return ErrNotFound
	}
	delete(s.Data, key)
	return nil
}

func (s *MockStorage) Copy(ctx context.Context, fromKey, toKey string) error {
	s.Lock()
	defer s.Unlock()

	item, exists := s.Data[fromKey]
	if !exists {
		return ErrNotFound
	}

	s.Data[toKey] = item
	return nil
}

// All returns all objects in the store, from a given path.
//
// The path can be empty.
func (s *MockStorage) Search(ctx context.Context, query map[string]string) ([][]byte, error) {
	s.Lock()
	defer s.Unlock()

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
	s.Lock()
	defer s.Unlock()

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
	s.Lock()
	defer s.Unlock()

	result := make([]string, 0)

	for key, _ := range s.Data {
		if !strings.HasPrefix(key, path) {
			continue
		}

		result = append(result, key)
	}

	return result, nil
}

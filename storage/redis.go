package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/gomodule/redigo/redis"
)

var (
	// ErrUnknownPayload is returned if an expected payload is returned by the store.
	ErrUnknownPayload = errors.New("Unknown payload")

	// ErrUnsupported is returned if the Storage does not implemnent a feature.
	ErrUnsupported = errors.New("Unsupported")
)

// RedisStorage implements a Storage backed by Redis.
type RedisStorage struct {
	Conn redis.Conn
}

// NewRedisStorage return a new RedisStorage.
func NewRedisStorage(conn redis.Conn) *RedisStorage {
	return &RedisStorage{
		Conn: conn,
	}
}

// Reader implemented the Reader interface.
func (r *RedisStorage) Read(ctx context.Context, key string) ([]byte, error) {
	resp, err := r.Conn.Do("GET", key)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, ErrNotFound
	}

	b, ok := resp.([]byte)
	if !ok {
		return nil, ErrUnknownPayload
	}

	return b, nil
}

// Write implements the Writer interface.
func (r *RedisStorage) Write(ctx context.Context, key string, b []byte, opts *Options) error {
	if _, err := r.Conn.Do("SET", key, b); err != nil {
		return err
	}

	return r.Conn.Flush()
}

// Remove implements the Remover interface.
func (r *RedisStorage) Remove(ctx context.Context, key string) error {
	if _, err := r.Conn.Do("DEL", key); err != nil {
		return err
	}

	return r.Conn.Flush()
}

// Search imlements the Searcher interface.
//
// This is not implemented as Redis isn't really intended for this kind of use.
func (r *RedisStorage) Search(ctx context.Context, query map[string]string) ([][]byte, error) {
	return nil, ErrUnsupported
}

// Clear implements the Clearer interface.
//
// Although Redis supports deleting all keys with FLUSHALL, I'm not comfortable implemnting it.
func (r *RedisStorage) Clear(ctx context.Context, query map[string]string) error {
	return ErrUnsupported
}

// List implements the List interface.
func (r *RedisStorage) List(ctx context.Context, key string) ([]string, error) {
	k := fmt.Sprintf("%s*", key)

	resp, err := r.Conn.Do("KEYS", k)
	if err != nil {
		return nil, err
	}

	data, ok := resp.([]interface{})
	if !ok {
		fmt.Printf("%#+v\n", resp)

		return nil, ErrUnknownPayload
	}

	keys := make([]string, len(data), len(data))

	for i := range data {
		keys[i] = fmt.Sprintf("%s", data[i])
	}

	// sort the keys
	sort.Strings(keys)

	return keys, nil
}

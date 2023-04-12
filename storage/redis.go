package storage

import (
	"context"
	"fmt"
	"sort"

	"github.com/gomodule/redigo/redis"

	"github.com/pkg/errors"
)

var (
	// ErrUnknownPayload is returned if an expected payload is returned by the store.
	ErrUnknownPayload = errors.New("Unknown payload")

	// ErrUnsupported is returned if the Storage does not implemnent a feature.
	ErrUnsupported = errors.New("Unsupported")
)

// RedisStorage implements a Storage backed by Redis.
type RedisStorage struct {
	Pool *redis.Pool
}

// NewRedisStorage return a new RedisStorage.
func NewRedisStorage(pool *redis.Pool) *RedisStorage {
	return &RedisStorage{
		Pool: pool,
	}
}

// Reader implemented the Reader interface.
func (r *RedisStorage) Read(ctx context.Context, key string) ([]byte, error) {
	conn := r.Pool.Get()
	defer conn.Close()

	resp, err := conn.Do("GET", key)
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
//
// If Options.TTL is set, the key will be set to expire in the given number of seconds.
func (r *RedisStorage) Write(ctx context.Context, key string, b []byte, opts *Options) error {
	conn := r.Pool.Get()
	defer conn.Close()

	if _, err := conn.Do("SET", key, b); err != nil {
		return err
	}

	if opts != nil && opts.TTL > 0 {
		// Redis expects TTL in seconds
		if _, err := conn.Do("EXPIRE", key, opts.TTL); err != nil {
			return err
		}
	}

	return conn.Flush()
}

func (r *RedisStorage) Copy(ctx context.Context, fromKey, toKey string) error {
	b, err := r.Read(ctx, fromKey)
	if err != nil {
		return errors.Wrap(err, "read")
	}

	if err := r.Write(ctx, toKey, b, nil); err != nil {
		return errors.Wrap(err, "write")
	}

	return nil
}

// Remove implements the Remover interface.
func (r *RedisStorage) Remove(ctx context.Context, key string) error {
	conn := r.Pool.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", key); err != nil {
		return err
	}

	return conn.Flush()
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
	conn := r.Pool.Get()
	defer conn.Close()

	k := fmt.Sprintf("%s*", key)

	resp, err := conn.Do("KEYS", k)
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

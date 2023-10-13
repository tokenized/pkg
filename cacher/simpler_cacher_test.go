package cacher

import (
	"context"
	"testing"
	"time"

	"github.com/tokenized/logger"
	"github.com/tokenized/pkg/storage"
)

func Test_NotFound(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")
	store := storage.NewMockStorage()
	cache := NewSimpleCache(store)

	RunTest_NotFound(ctx, t, cache)
}

func Test_Add(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")
	store := storage.NewMockStorage()
	cache := NewSimpleCache(store)

	RunTest_Add(ctx, t, cache)
}

// Test_Add_Lock tests a lot of concurrent add requests and ensures only one item is added and later
// returned.
func Test_Add_Lock(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")
	store := storage.NewMockStorage()
	cache := NewSimpleCache(store)

	RunTest_Add_Lock(ctx, t, cache)
}

func Test_Sets_Basic(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")
	store := storage.NewMockStorage()
	cache := NewSimpleCache(store)

	RunTest_Sets_Basic(ctx, t, cache)
}

func Test_Sets_Multi(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")
	store := storage.NewMockStorage()
	cache := NewSimpleCache(store)

	RunTest_Sets_Multi(ctx, t, cache, time.Millisecond)
}

func Test_ListSets(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")
	store := storage.NewMockStorage()
	cache := NewSimpleCache(store)

	RunTest_ListSets(ctx, t, cache)
}

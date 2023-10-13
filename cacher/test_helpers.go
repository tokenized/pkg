package cacher

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	mathRand "math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

type TestItem struct {
	Value string

	isModified atomic.Value
	sync.Mutex
}

func RunTest_NotFound(ctx context.Context, t *testing.T, cache Cacher) {
	typ := reflect.TypeOf(&TestItem{})

	var hash bitcoin.Hash32
	rand.Read(hash[:])
	notFound, err := cache.Get(ctx, typ, GetTestItemPath(hash))
	if err != nil {
		t.Fatalf("Failed to get item : %s", err)
	}

	if notFound != nil {
		t.Fatalf("Item should be nil")
	}
}

func RunTest_Add(ctx context.Context, t *testing.T, cache Cacher) {
	typ := reflect.TypeOf(&TestItem{})

	item := &TestItem{
		Value: "test value",
	}
	item.isModified.Store(true)
	path := item.path()

	addedCacheItem, err := cache.Add(ctx, typ, path, item)
	if err != nil {
		t.Fatalf("Failed to add item : %s", err)
	}

	if addedCacheItem == nil {
		t.Fatalf("Added item should not be nil")
	}

	addedItem, ok := addedCacheItem.(*TestItem)
	if !ok {
		t.Fatalf("Added item not a TestItem")
	}

	if addedItem != item {
		t.Errorf("Wrong added item : got %s, want %s", addedItem.Value, item.Value)
	}

	cache.Release(ctx, path)

	gotCacheItem, err := cache.Get(ctx, typ, path)
	if err != nil {
		t.Fatalf("Failed to get item : %s", err)
	}

	if gotCacheItem == nil {
		t.Fatalf("Item not found")
	}

	gotItem, ok := gotCacheItem.(*TestItem)
	if !ok {
		t.Fatalf("Got item not a TestItem")
	}

	if gotItem.Value != item.Value {
		t.Errorf("Wrong item found : got %s, want %s", gotItem.Value, item.Value)
	}

	cache.Release(ctx, path)

	duplicateItem := &TestItem{
		Value: "test value",
	}
	duplicateItem.isModified.Store(true)
	path = duplicateItem.path()

	addedCacheItem, err = cache.Add(ctx, typ, path, duplicateItem)
	if err != nil {
		t.Fatalf("Failed to add item : %s", err)
	}

	if addedCacheItem == nil {
		t.Fatalf("Added item should not be nil")
	}

	addedItem, ok = addedCacheItem.(*TestItem)
	if !ok {
		t.Fatalf("Added item not a TestItem")
	}

	if addedItem.Value != item.Value {
		t.Errorf("Wrong added item : got %s, want %s", addedItem.Value, item.Value)
	}

	cache.Release(ctx, path)
}

// RunTest_Add_Lock tests a lot of concurrent add requests and ensures only one item is added and later
// returned.
func RunTest_Add_Lock(ctx context.Context, t *testing.T, cache Cacher) {
	typ := reflect.TypeOf(&TestItem{})

	testItem := &TestItem{
		Value: "test value",
	}
	path := testItem.path()

	count := 50
	items := make([]*TestItem, count)
	addedItems := make([]*TestItem, count)
	matchingCount := 0
	var matchingItem *TestItem
	for i := 0; i < count; i++ {
		index := i
		go func() {
			item := &TestItem{
				Value: "test value",
			}
			items[index] = item

			time.Sleep(time.Millisecond * time.Duration(mathRand.Intn(10)))

			addedCacheItem, err := cache.Add(ctx, typ, item.path(), item)
			if err != nil {
				t.Fatalf("Failed to add item : %s", err)
			}

			if addedCacheItem == nil {
				t.Fatalf("Added item should not be nil")
			}

			addedItem, ok := addedCacheItem.(*TestItem)
			if !ok {
				t.Fatalf("Added item not a TestItem")
			}
			addedItems[index] = addedItem

			if addedItem == item {
				t.Logf("Item %d matches", index)
				matchingCount++
				matchingItem = item
			}

			time.Sleep(time.Millisecond * 100)
			cache.Release(ctx, path)
		}()
	}

	time.Sleep(100 * time.Millisecond)

	if matchingCount != 1 {
		t.Errorf("Wrong matching count : got %d, want %d", matchingCount, 1)
	}

	gotCacheItem, err := cache.Get(ctx, typ, path)
	if err != nil {
		t.Fatalf("Failed to get item : %s", err)
	}

	if gotCacheItem == nil {
		t.Fatalf("Item not found")
	}

	gotItem, ok := gotCacheItem.(*TestItem)
	if !ok {
		t.Fatalf("Got item not a TestItem")
	}

	if gotItem != matchingItem {
		t.Errorf("Wrong item found : got %s, want %s", gotItem.Value, matchingItem.Value)
	}

	cache.Release(ctx, path)

	duplicateItem := &TestItem{
		Value: "test value",
	}
	path = duplicateItem.path()

	addedCacheItem, err := cache.Add(ctx, typ, path, duplicateItem)
	if err != nil {
		t.Fatalf("Failed to add item : %s", err)
	}

	if addedCacheItem == nil {
		t.Fatalf("Added item should not be nil")
	}

	addedItem, ok := addedCacheItem.(*TestItem)
	if !ok {
		t.Fatalf("Added item not a TestItem")
	}

	if addedItem != matchingItem {
		t.Errorf("Wrong added item : got %s, want %s", addedItem.Value, matchingItem.Value)
	}

	cache.Release(ctx, path)
}

func GetTestItemPath(id bitcoin.Hash32) string {
	return fmt.Sprintf("items/%s", id)
}

func (i *TestItem) Initialize() {
	i.isModified.Store(false)
}

func (i *TestItem) path() string {
	return fmt.Sprintf("items/%s", bitcoin.Hash32(sha256.Sum256([]byte(i.Value))))
}

func (i *TestItem) IsModified() bool {
	return i.isModified.Load().(bool)
}

func (i *TestItem) MarkModified() {
	i.isModified.Store(true)
}

func (i *TestItem) GetModified() bool {
	return i.isModified.Swap(false).(bool)
}

func (i *TestItem) CacheCopy() Value {
	result := &TestItem{
		Value: i.Value,
	}
	result.isModified.Store(true)
	return result
}

func (i *TestItem) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(i.Value))); err != nil {
		return errors.Wrap(err, "size")
	}

	if _, err := w.Write([]byte(i.Value)); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func (i *TestItem) Deserialize(r io.Reader) error {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return errors.Wrap(err, "size")
	}

	b := make([]byte, size)
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "value")
	}
	i.Value = string(b)

	return nil
}

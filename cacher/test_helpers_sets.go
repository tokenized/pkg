package cacher

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	mathRand "math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type TestSetValue struct {
	Name  string
	Value string

	markModified MarkModified
	sync.Mutex   `bsor:"-"`
}

func RunTest_Sets_Basic(ctx context.Context, t *testing.T, cache Cacher) {
	typ := reflect.TypeOf(&TestSetValue{})
	pathPrefix := "sets"

	value := &TestSetValue{
		Name:  "Value 1",
		Value: "Test Value",
	}
	hash := value.Hash()

	addedValue, err := cache.AddSetValue(ctx, typ, pathPrefix, value)
	if err != nil {
		t.Fatalf("Failed to add set value : %s", err)
	}

	if addedValue.(*TestSetValue) != value {
		t.Errorf("Added value does not match")
	}

	cache.ReleaseSetValue(ctx, typ, pathPrefix, hash)

	// Wait for object to expire from the cache
	time.Sleep(time.Millisecond * 100)

	gotValue, err := cache.GetSetValue(ctx, typ, pathPrefix, hash)
	if err != nil {
		t.Fatalf("Failed to get set value : %s", err)
	}

	if gotValue == nil {
		t.Fatalf("Got set value is nil")
	}

	gotV := gotValue.(*TestSetValue)

	if gotV.Name != "Value 1" {
		t.Errorf("Wrong set name : got \"%s\", want \"%s\"", gotV.Name, "Value 1")
	}

	if gotV.Value != "Test Value" {
		t.Errorf("Wrong set value : got \"%s\", want \"%s\"", gotV.Value, "Test Value")
	}

	// Modify value
	gotV.Value = "Test Value 2"
	gotV.MarkModified()

	cache.ReleaseSetValue(ctx, typ, pathPrefix, hash)

	// Wait for object to expire from the cache
	time.Sleep(time.Millisecond * 100)

	gotValue2, err := cache.GetSetValue(ctx, typ, pathPrefix, hash)
	if err != nil {
		t.Fatalf("Failed to get set value : %s", err)
	}

	if gotValue2 == nil {
		t.Fatalf("Got set value is nil")
	}

	gotV2 := gotValue2.(*TestSetValue)

	if gotV2.Name != "Value 1" {
		t.Errorf("Wrong set name : got \"%s\", want \"%s\"", gotV2.Name, "Value 1")
	}

	if gotV2.Value != "Test Value 2" {
		t.Errorf("Wrong set value : got \"%s\", want \"%s\"", gotV2.Value, "Test Value 2")
	}

	cache.ReleaseSetValue(ctx, typ, pathPrefix, hash)
}

// expireWait is used to wait between releasing items and when to get them again to ensure they are
// released from cachers that expire items after a given period of time.
func RunTest_Sets_Multi(ctx context.Context, t *testing.T, cache Cacher, expireWait time.Duration) {
	typ := reflect.TypeOf(&TestSetValue{})
	pathPrefix := "sets"

	roundCount := 1000
	var allItems []*TestSetValue
	alternateAdd := false
	alternateRelease := false
	for g := 0; g < roundCount; g++ {
		count := mathRand.Intn(10) + 1
		var values []SetValue
		var hashes []bitcoin.Hash32
		var isModified []bool
		for c := 0; c < count; c++ {
			id := uuid.New()

			item := &TestSetValue{
				Name:  id.String(),
				Value: "Test Value " + id.String(),
			}
			allItems = append(allItems, item)

			value := &TestSetValue{
				Name:  id.String(),
				Value: "Test Value " + id.String(),
			}
			values = append(values, value)

			hash := value.Hash()
			t.Logf("Adding value : %s", hash)
			hashes = append(hashes, hash)
			isModified = append(isModified, false)
		}

		if alternateAdd {
			addedValues, err := cache.AddMultiSetValue(ctx, typ, pathPrefix, values)
			if err != nil {
				t.Fatalf("Failed to add set values : %s", err)
			}

			for i, addedValue := range addedValues {
				if addedValue != values[i] {
					t.Errorf("Added value %d does not match", i)
				}
			}

			alternateAdd = false
		} else {
			for i, value := range values {
				addedValue, err := cache.AddSetValue(ctx, typ, pathPrefix, value)
				if err != nil {
					t.Fatalf("Failed to add set value : %s", err)
				}

				if addedValue != values[i] {
					t.Errorf("Added value %d does not match", i)
				}
			}

			alternateAdd = true
		}

		if alternateRelease {
			cache.ReleaseMultiSetValue(ctx, typ, pathPrefix, hashes)
			alternateRelease = false
		} else {
			for _, hash := range hashes {
				cache.ReleaseSetValue(ctx, typ, pathPrefix, hash)
			}
			alternateRelease = true
		}
	}

	totalCount := len(allItems)
	t.Logf("Created %d items", totalCount)

	offset := 0
	for {
		count := mathRand.Intn(10) + 1
		if offset+count >= totalCount {
			break
		}

		items := allItems[offset : offset+count]
		offset += count

		hashes := make([]bitcoin.Hash32, count)
		for i, item := range items {
			hashes[i] = item.Hash()
		}

		values, err := cache.GetMultiSetValue(ctx, typ, pathPrefix, hashes)
		if err != nil {
			t.Fatalf("Failed to get set value : %s", err)
		}

		var toRelease []bitcoin.Hash32
		for i, value := range values {
			if value == nil {
				t.Errorf("Missing value : %s", hashes[i])
				continue
			}

			hash := value.Hash()
			toRelease = append(toRelease, hash)
			if !hashes[i].Equal(&hash) {
				t.Errorf("Wrong hash on value %d : got %s, want %s", offset+i, hash, hashes[i])
			}

			v := value.(*TestSetValue)
			if items[i].Value != v.Value {
				t.Errorf("Wrong item value %d : got %s, want %s", offset+i, items[i].Value, v.Value)
			}
		}

		cache.ReleaseMultiSetValue(ctx, typ, pathPrefix, toRelease)
	}

	t.Logf("Fetched %d items before expiration", offset)

	// Wait for object to expire from the cache
	time.Sleep(expireWait)

	offset = 0
	for {
		count := mathRand.Intn(10) + 1
		if offset+count >= totalCount {
			break
		}

		items := allItems[offset : offset+count]
		offset += count

		var toRelease []bitcoin.Hash32
		for i, item := range items {
			hash := item.Hash()

			value, err := cache.GetSetValue(ctx, typ, pathPrefix, hash)
			if err != nil {
				t.Fatalf("Failed to get set value : %s", err)
			}

			if value == nil {
				t.Errorf("Missing value : %s", hash)
				continue
			}

			gotHash := value.Hash()
			toRelease = append(toRelease, gotHash)
			if !hash.Equal(&gotHash) {
				t.Errorf("Wrong hash on value %d : got %s, want %s", offset+i, gotHash, hash)
			}

			v := value.(*TestSetValue)
			if item.Value != v.Value {
				t.Errorf("Wrong item value %d : got %s, want %s", offset+i, item.Value, v.Value)
			}
		}

		for _, hash := range toRelease {
			cache.ReleaseSetValue(ctx, typ, pathPrefix, hash)
		}
	}

	t.Logf("Fetched %d items after expiration", offset)

	// Modify some items.
	var modifiedItems []*TestSetValue
	offset = 0
	for {
		// Don't modify all items
		skip := mathRand.Intn(10) + 1
		offset += skip

		count := mathRand.Intn(10) + 1
		if offset+count >= totalCount {
			break
		}

		items := allItems[offset : offset+count]
		offset += count

		hashes := make([]bitcoin.Hash32, count)
		for i, item := range items {
			hashes[i] = item.Hash()
		}

		values, err := cache.GetMultiSetValue(ctx, typ, pathPrefix, hashes)
		if err != nil {
			t.Fatalf("Failed to get set value : %s", err)
		}

		var toRelease []bitcoin.Hash32
		for i, value := range values {
			if value == nil {
				t.Errorf("Missing value : %s", hashes[i])
				continue
			}

			hash := value.Hash()
			toRelease = append(toRelease, hash)
			if !hashes[i].Equal(&hash) {
				t.Errorf("Wrong hash on value %d : got %s, want %s", offset+i, hash, hashes[i])
			}

			v := value.(*TestSetValue)
			if items[i].Value != v.Value {
				t.Errorf("Wrong item value %d : got %s, want %s", offset+i, items[i].Value, v.Value)
			}

			newValue := uuid.New()
			v.Value = newValue.String()
			v.MarkModified()

			modifiedItems = append(modifiedItems, &TestSetValue{
				Name:  v.Name,
				Value: newValue.String(),
			})
		}

		cache.ReleaseMultiSetValue(ctx, typ, pathPrefix, toRelease)
	}

	// Wait for objects to expire from the cache.
	time.Sleep(expireWait)

	// Check the modified items have the new values.
	offset = 0
	modifiedCount := len(modifiedItems)
	for {
		count := mathRand.Intn(10) + 1
		if offset+count >= modifiedCount {
			break
		}

		items := modifiedItems[offset : offset+count]
		offset += count

		hashes := make([]bitcoin.Hash32, count)
		for i, item := range items {
			hashes[i] = item.Hash()
		}

		values, err := cache.GetMultiSetValue(ctx, typ, pathPrefix, hashes)
		if err != nil {
			t.Fatalf("Failed to get set value : %s", err)
		}

		var toRelease []bitcoin.Hash32
		for i, value := range values {
			if value == nil {
				t.Errorf("Missing value : %s", hashes[i])
				continue
			}

			hash := value.Hash()
			toRelease = append(toRelease, hash)
			if !hashes[i].Equal(&hash) {
				t.Errorf("Wrong hash on value %d : got %s, want %s", offset+i, hash, hashes[i])
			}

			v := value.(*TestSetValue)
			if items[i].Value != v.Value {
				t.Errorf("Wrong item value %d : got %s, want %s", offset+i, items[i].Value, v.Value)
			}
		}

		cache.ReleaseMultiSetValue(ctx, typ, pathPrefix, toRelease)
	}
}

func RunTest_ListSets(ctx context.Context, t *testing.T, cache Cacher) {
	typ := reflect.TypeOf(&TestSetValue{})
	pathPrefix := "sets"

	count := 5000
	addedValues := make(map[string]bool)
	for i := 0; i < count; i++ {
		value := &TestSetValue{
			Name:  uuid.New().String(),
			Value: uuid.New().String(),
		}
		addedValues[value.Value] = false

		if _, err := cache.AddSetValue(ctx, typ, pathPrefix, value); err != nil {
			t.Fatalf("Failed to add set value : %s", err)
		}

		cache.ReleaseSetValue(ctx, typ, pathPrefix, value.Hash())
	}

	values, err := cache.ListMultiSetValue(ctx, typ, pathPrefix)
	if err != nil {
		t.Fatalf("Failed to list set values : %s", err)
	}
	isModifieds := make([]bool, len(values))
	hashes := make([]bitcoin.Hash32, len(values))

	t.Logf("Listed %d values", len(values))

	i := 0
	for _, value := range values {
		value.Lock()
		item := value.(*TestSetValue)
		wasFound, exists := addedValues[item.Value]
		if !exists {
			t.Errorf("Value wasn't added : %s", item.Value)
		}
		if wasFound {
			t.Errorf("Value was already found : %s", item.Value)
		}
		addedValues[item.Value] = true
		hashes[i] = value.Hash()
		value.Unlock()
		isModifieds[i] = false
		i++
	}

	allFound := true
	for value, found := range addedValues {
		if !found {
			allFound = false
			t.Errorf("Value was not found : %s", value)
		}
	}

	if allFound {
		t.Logf("Found all %d values", len(addedValues))
	}

	cache.ReleaseMultiSetValue(ctx, typ, pathPrefix, hashes)
}

func (v *TestSetValue) ProvideMarkModified(markModified MarkModified) {
	v.markModified = markModified
}

func (v *TestSetValue) MarkModified() {
	if v.markModified == nil {
		panic("mark modified not set")
	}

	v.markModified()
}

func (v *TestSetValue) Hash() bitcoin.Hash32 {
	return bitcoin.Hash32(sha256.Sum256([]byte(v.Name)))
}

func (i *TestSetValue) CacheSetCopy() SetValue {
	return &TestSetValue{
		Name:  i.Name,
		Value: i.Value,
	}
}

func (v *TestSetValue) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(v.Name))); err != nil {
		return errors.Wrap(err, "name size")
	}

	if _, err := w.Write([]byte(v.Name)); err != nil {
		return errors.Wrap(err, "name")
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(v.Value))); err != nil {
		return errors.Wrap(err, "value size")
	}

	if _, err := w.Write([]byte(v.Value)); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func (v *TestSetValue) Deserialize(r io.Reader) error {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return errors.Wrap(err, "name size")
	}

	name := make([]byte, size)
	if _, err := r.Read(name); err != nil {
		return errors.Wrap(err, "name")
	}
	v.Name = string(name)

	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return errors.Wrap(err, "size")
	}

	value := make([]byte, size)
	if _, err := r.Read(value); err != nil {
		return errors.Wrap(err, "value")
	}
	v.Value = string(value)

	return nil
}

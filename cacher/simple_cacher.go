package cacher

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tokenized/logger"
	"github.com/tokenized/pkg/storage"

	"github.com/pkg/errors"
)

// SimpleCacher is the simplest implementation of the Cacher interface. It ensures only one instance
// of each item exists in the cache at once and handles fetching the values from storage and writing
// them back to storage if they are modified.
type SimpleCacher struct {
	items     map[string]*SimpleItem
	itemsLock sync.Mutex

	store storage.Storage

	cacheSetType reflect.Type
}

type SimpleItem struct {
	value Value
	users uint
}

func NewSimpleCache(store storage.Storage) *SimpleCacher {
	return &SimpleCacher{
		items:        make(map[string]*SimpleItem),
		store:        store,
		cacheSetType: reflect.TypeOf(&cacheSet{}),
	}
}

func (c *SimpleCacher) Add(ctx context.Context, typ reflect.Type, path string,
	value Value) (Value, error) {

	emptyTypeValue := reflect.New(typ.Elem())
	emptyValueInterface := emptyTypeValue.Interface()
	emptyValue := emptyValueInterface.(Value)
	emptyValue.Initialize()
	return c.addValue(ctx, typ, path, emptyValue, value)
}

func (c *SimpleCacher) AddMulti(ctx context.Context, typ reflect.Type, paths []string,
	values []Value) ([]Value, error) {

	result := make([]Value, len(values))
	for i, value := range values {
		emptyTypeValue := reflect.New(typ.Elem())
		emptyValueInterface := emptyTypeValue.Interface()
		emptyValue := emptyValueInterface.(Value)
		emptyValue.Initialize()

		v, err := c.addValue(ctx, typ, paths[i], emptyValue, value)
		if err != nil {
			return nil, errors.Wrapf(err, "add %d", i)
		}

		result[i] = v
	}

	return result, nil
}

func (c *SimpleCacher) Get(ctx context.Context, typ reflect.Type, path string) (Value, error) {
	emptyTypeValue := reflect.New(typ.Elem())
	emptyValueInterface := emptyTypeValue.Interface()
	emptyValue := emptyValueInterface.(Value)
	emptyValue.Initialize()
	return c.getValue(ctx, typ, path, emptyValue)
}

func (c *SimpleCacher) GetMulti(ctx context.Context, typ reflect.Type,
	paths []string) ([]Value, error) {

	result := make([]Value, len(paths))
	for i, path := range paths {
		emptyTypeValue := reflect.New(typ.Elem())
		emptyValueInterface := emptyTypeValue.Interface()
		emptyValue := emptyValueInterface.(Value)
		emptyValue.Initialize()

		v, err := c.getValue(ctx, typ, path, emptyValue)
		if err != nil {
			return nil, errors.Wrapf(err, "get %d", i)
		}

		result[i] = v
	}

	return result, nil
}

func (c *SimpleCacher) List(ctx context.Context, pathPrefix, regExPath string) ([]string, error) {
	regex, err := regexp.Compile(regExPath)
	if err != nil {
		return nil, errors.Wrap(err, "regex")
	}

	set := make(map[string]bool)

	// Get any items that are in the cache.
	c.itemsLock.Lock()
	for path, _ := range c.items {
		if !regex.MatchString(path) {
			continue
		}

		set[path] = true
	}
	c.itemsLock.Unlock()

	allPaths, err := c.store.List(ctx, pathPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "list sets")
	}

	if len(allPaths) == 0 && len(set) == 0 {
		return nil, nil
	}

	for _, path := range allPaths {
		if !regex.MatchString(path) {
			continue
		}

		set[path] = true
	}

	result := make([]string, len(set))
	for path, _ := range set {
		result = append(result, path)
	}

	return result, nil
}

func (c *SimpleCacher) CopyRecursive(ctx context.Context, fromPathPrefix,
	toPathPrefix string) error {

	fromPathPrefixLength := len(fromPathPrefix)
	values := make(map[string]Value)
	c.itemsLock.Lock()
	for path, item := range c.items {
		if !strings.HasPrefix(path, fromPathPrefix) {
			continue
		}

		toPath := toPathPrefix + path[fromPathPrefixLength:]
		item.value.Lock()
		copyItem := &SimpleItem{
			value: item.value.CacheCopy(),
			users: 1,
		}
		copyItem.value.MarkModified()
		item.value.Unlock()
		c.items[toPath] = copyItem
		values[path] = copyItem.value
	}
	c.itemsLock.Unlock()

	// Copy any items only in storage.
	paths, err := c.store.List(ctx, fromPathPrefix)
	if err != nil {
		return errors.Wrap(err, "list")
	}

	for _, path := range paths {
		if _, exists := values[path]; exists {
			continue // already copied in cache
		}

		toPath := toPathPrefix + path[fromPathPrefixLength:]
		if err := c.store.Copy(ctx, path, toPath); err != nil {
			return errors.Wrapf(err, "copy: %s", path)
		}
	}

	for path, _ := range values {
		c.release(ctx, path)
	}

	return nil
}

func (c *SimpleCacher) Save(ctx context.Context, path string, value Value) {
	c.saveItem(ctx, path, value)
}

func (c *SimpleCacher) AddUser(ctx context.Context, path string) {
	c.itemsLock.Lock()
	if item, exists := c.items[path]; exists {
		item.users++
	}
	c.itemsLock.Unlock()
}

func (c *SimpleCacher) Release(ctx context.Context, path string) {
	c.release(ctx, path)
}

func (c *SimpleCacher) IsEmpty(ctx context.Context) bool {
	c.itemsLock.Lock()
	count := len(c.items)
	c.itemsLock.Unlock()

	return count == 0
}

func (c *SimpleCacher) getValue(ctx context.Context, typ reflect.Type, path string,
	emptyValue Value) (Value, error) {

	c.itemsLock.Lock()
	if item, exists := c.items[path]; exists {
		// Item already exists in the cache so just increment the user count and return the value.
		item.users++
		value := item.value
		c.itemsLock.Unlock()
		return value, nil
	}
	c.itemsLock.Unlock()

	// Check if the item is in storage.
	var readValue Value
	b, err := c.store.Read(ctx, path)
	if err == nil {
		// Deserialize read value.
		readValue = emptyValue
		if err := readValue.Deserialize(bytes.NewReader(b)); err != nil {
			return nil, errors.Wrap(err, "deserialize")
		}
	} else if errors.Cause(err) != storage.ErrNotFound {
		return nil, errors.Wrap(err, "read")
	}

	c.itemsLock.Lock()
	if item, exists := c.items[path]; exists {
		// Item was added since original check so discard the value read from storage and return
		// the value in the item set.
		item.users++
		value := item.value
		c.itemsLock.Unlock()
		return value, nil
	}

	if readValue != nil {
		// Add new item read from storage.
		newItem := &SimpleItem{
			value: readValue,
			users: 1,
		}
		c.items[path] = newItem
		c.itemsLock.Unlock()

		return readValue, nil
	}

	c.itemsLock.Unlock()

	// Item is not in storage
	return nil, nil
}

func (c *SimpleCacher) addValue(ctx context.Context, typ reflect.Type, path string,
	emptyValue, newValue Value) (Value, error) {

	c.itemsLock.Lock()
	if item, exists := c.items[path]; exists {
		// Item already exists in the cache so just increment the user count and return the value.
		item.users++
		value := item.value
		c.itemsLock.Unlock()
		return value, nil
	}
	c.itemsLock.Unlock()

	// Check if the item is in storage.
	var readValue Value
	b, err := c.store.Read(ctx, path)
	if err == nil {
		// Deserialize read value.
		readValue = emptyValue
		if err := readValue.Deserialize(bytes.NewReader(b)); err != nil {
			return nil, errors.Wrap(err, "deserialize")
		}
	} else if errors.Cause(err) != storage.ErrNotFound {
		return nil, errors.Wrap(err, "read")
	}

	c.itemsLock.Lock()
	if item, exists := c.items[path]; exists {
		// Item was added since original check so discard the value read from storage and return
		// the value in the item set.
		item.users++
		value := item.value
		c.itemsLock.Unlock()
		return value, nil
	}

	if readValue != nil {
		// Add new item read from storage.
		newItem := &SimpleItem{
			value: readValue,
			users: 1,
		}
		c.items[path] = newItem
		c.itemsLock.Unlock()

		return readValue, nil
	}

	// Add new value.
	newValue.Lock()
	newValue.MarkModified()
	newValue.Unlock()
	newItem := &SimpleItem{
		value: newValue,
		users: 1,
	}
	c.items[path] = newItem
	c.itemsLock.Unlock()

	return newValue, nil
}

func (c *SimpleCacher) release(ctx context.Context, path string) {
	c.itemsLock.Lock()

	item, exists := c.items[path]
	if !exists {
		c.itemsLock.Unlock()
		panic(fmt.Sprintf("released item not in set: %s", path))
	}

	item.users--
	if item.users > 0 {
		c.itemsLock.Unlock()
		return
	}

	// Remove item from the set and save the value, if modified.
	value := item.value
	delete(c.items, path)

	c.itemsLock.Unlock()

	// Save item
	c.saveItem(ctx, path, value)
}

func (c *SimpleCacher) saveItem(ctx context.Context, path string, value Value) error {
	value.Lock()
	defer value.Unlock()

	if !value.GetModified() {
		return nil
	}

	if err := saveValue(ctx, c.store, path, value); err != nil {
		logger.ErrorWithFields(ctx, []logger.Field{
			logger.String("path", path),
		}, "Failed to write value to storage : %s", err)
		return err
	}

	return nil
}

func saveValue(ctx context.Context, store storage.Writer, path string,
	s storage.Serializer) error {
	start := time.Now()

	buf := &bytes.Buffer{}
	if err := s.Serialize(buf); err != nil {
		return errors.Wrap(err, "serialize")
	}

	if err := store.Write(ctx, path, buf.Bytes(), nil); err != nil {
		return errors.Wrap(err, "write")
	}

	logger.VerboseWithFields(ctx, []logger.Field{
		logger.String("path", path),
		logger.MillisecondsFromNano("elapsed_ms", time.Since(start).Nanoseconds()),
	}, "Cache value written to storage")
	return nil
}

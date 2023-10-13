package cacher

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

const (
	cacheSetVersion = uint8(0)
)

var (
	endian = binary.LittleEndian
)

// Sets
func (c *SimpleCacher) AddSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
	value SetValue) (SetValue, error) {

	hash := value.Hash()
	pathID := hashPathID(hash)
	emptySet := &cacheSet{
		typ:        typ,
		pathPrefix: pathPrefix,
		pathID:     pathID,
		values:     make(map[bitcoin.Hash32]SetValue),
	}
	emptySet.isModified.Store(true)

	path := setPath(pathPrefix, pathID)

	item, err := c.addValue(ctx, c.cacheSetType, path, emptySet, emptySet)
	if err != nil {
		return nil, errors.Wrap(err, "add")
	}

	set := item.(*cacheSet)
	set.Lock()
	result, exists := set.values[hash]
	if exists {
		// Value already exists so return existing value to be updated.
		set.Unlock()

		value.ProvideMarkModified(set.MarkModified)
		return result, nil
	}

	// Add value to set
	set.values[hash] = value
	value.ProvideMarkModified(set.MarkModified)
	set.MarkModified()
	set.Unlock()

	return value, nil
}

func (c *SimpleCacher) AddMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
	values []SetValue) ([]SetValue, error) {

	var createSets cacheSets
	for _, value := range values {
		createSets.add(pathPrefix, typ, value)
	}

	sets := make(cacheSets, len(createSets))
	paths := make([]string, len(createSets))
	for i, set := range createSets {
		emptySet := &cacheSet{
			typ:        typ,
			pathPrefix: pathPrefix,
			pathID:     set.pathID,
			values:     make(map[bitcoin.Hash32]SetValue),
		}
		emptySet.isModified.Store(true)
		paths[i] = set.path()

		v, err := c.addValue(ctx, c.cacheSetType, paths[i], emptySet, set)
		if err != nil {
			return nil, errors.Wrapf(err, "add %d", i)
		}
		sets[i] = v.(*cacheSet)
	}

	// Build resulting values from sets.
	result := make([]SetValue, len(values))
	setsUsed := make([]bool, len(sets))
	for i, value := range values {
		hash := value.Hash()
		pathID := hashPathID(hash)
		set, setIndex := sets.getSet(pathID)
		if set == nil {
			// This shouldn't be possible if AddMulti is functioning properly.
			return nil, errors.New("Value Set Missing") // value set not within sets
		}

		alreadyUsed := setsUsed[setIndex]
		setsUsed[setIndex] = true

		valueAdded := false
		if createSets[setIndex] == set {
			// The entire set is new and already added so the value doesn't need to be added to the
			// set.
			valueAdded = true
			result[i] = value
		}

		set.Lock()

		if !valueAdded {
			gotValue, exists := set.values[hash]
			if exists {
				// Value exists so return existing value to be modified.
				result[i] = gotValue
				gotValue.ProvideMarkModified(set.MarkModified)
			} else {
				// Add new value to set.
				result[i] = value
				set.values[hash] = value
				value.ProvideMarkModified(set.MarkModified)
				set.MarkModified()
			}
		}

		if alreadyUsed {
			set.extraUsers++
		}

		set.Unlock()
	}

	return result, nil
}

func (c *SimpleCacher) GetSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
	hash bitcoin.Hash32) (SetValue, error) {

	pathID := hashPathID(hash)
	path := setPath(pathPrefix, pathID)
	emptySet := &cacheSet{
		typ:        typ,
		pathPrefix: pathPrefix,
		pathID:     pathID,
		values:     make(map[bitcoin.Hash32]SetValue),
	}
	emptySet.isModified.Store(false)

	setValue, err := c.getValue(ctx, c.cacheSetType, path, emptySet)
	if err != nil {
		return nil, errors.Wrap(err, "response")
	}

	if setValue == nil {
		return nil, nil // set doesn't exist
	}

	set := setValue.(*cacheSet)
	set.Lock()
	value, exists := set.values[hash]
	set.Unlock()
	value.ProvideMarkModified(set.MarkModified)

	if !exists {
		// If a set didn't have the value requested then we aren't returning a value, so there will
		// not be a subsequent call to release that set, so release it now.
		c.release(ctx, path)
		return nil, nil // value not within set
	}

	return value, nil
}

func (c *SimpleCacher) GetMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
	hashes []bitcoin.Hash32) ([]SetValue, error) {

	count := len(hashes)
	pathIDs := make([][2]byte, count)
	var getPaths []string
	var sets cacheSets
	for i, hash := range hashes {
		pathID := hashPathID(hash)
		path := setPath(pathPrefix, pathID)

		pathIDs[i] = pathID
		if !stringExists(getPaths, path) {
			emptySet := &cacheSet{
				typ:        typ,
				pathPrefix: pathPrefix,
				pathID:     pathID,
				values:     make(map[bitcoin.Hash32]SetValue),
			}
			emptySet.isModified.Store(false)

			v, err := c.getValue(ctx, c.cacheSetType, path, emptySet)
			if err != nil {
				for _, path := range getPaths {
					c.release(ctx, path)
				}
				return nil, errors.Wrap(err, "get set")
			}
			getPaths = append(getPaths, path)

			if v != nil {
				sets = append(sets, v.(*cacheSet))
			}
		}
	}

	result := make([]SetValue, count)
	setsUsed := make([]bool, len(sets))
	for i, hash := range hashes {
		set, setIndex := sets.getSet(pathIDs[i])
		if set == nil {
			continue // set doesn't exist so leave result value nil
		}

		set.Lock()
		value, exists := set.values[hash]
		if exists && setsUsed[setIndex] {
			set.extraUsers++
		}
		set.Unlock()

		if !exists {
			continue // value not within set
		}

		setsUsed[setIndex] = true
		value.ProvideMarkModified(set.MarkModified)
		result[i] = value
	}

	// If a set didn't have the value requested then we aren't returning a value, so there will not
	// be a subsequent call to release that set, so release it now.
	for i, setUsed := range setsUsed {
		if !setUsed && sets[i] != nil {
			c.release(ctx, sets[i].path())
		}
	}

	return result, nil
}

func (c *SimpleCacher) ListMultiSetValue(ctx context.Context, typ reflect.Type,
	pathPrefix string) ([]SetValue, error) {

	sets := make(map[string]*cacheSet)

	// Get any items that are in the cache.
	c.itemsLock.Lock()
	for path, item := range c.items {
		if !strings.HasPrefix(path, pathPrefix) {
			continue
		}

		sets[path] = item.value.(*cacheSet)
		item.users++
	}
	c.itemsLock.Unlock()

	allPaths, err := c.store.List(ctx, pathPrefix)
	if err != nil {
		// Release all current items
		for path := range sets {
			c.release(ctx, path)
		}
		return nil, errors.Wrap(err, "list sets")
	}

	if len(allPaths) == 0 && len(sets) == 0 {
		return nil, nil
	}

	for _, path := range allPaths {
		if _, exists := sets[path]; exists {
			continue // don't get paths that are already in sets.
		}

		pathID, err := pathIDFromPath(path)
		if err != nil {
			// Release all current items
			for path := range sets {
				c.release(ctx, path)
			}
			return nil, errors.Wrapf(err, "path id: %s", path)
		}

		emptySet := &cacheSet{
			typ:        typ,
			pathPrefix: path,
			pathID:     pathID,
			values:     make(map[bitcoin.Hash32]SetValue),
		}
		emptySet.isModified.Store(false)

		v, err := c.getValue(ctx, c.cacheSetType, path, emptySet)
		if err != nil {
			for path := range sets {
				c.release(ctx, path)
			}
			return nil, errors.Wrap(err, "get set")
		}

		if v != nil {
			sets[path] = v.(*cacheSet)
		}
	}

	var result []SetValue
	for path, set := range sets {
		first := true
		set.Lock()
		for _, value := range set.values {
			result = append(result, value.(SetValue))
			value.ProvideMarkModified(set.MarkModified)
			if first {
				first = false
			} else {
				set.extraUsers++
			}
		}
		set.Unlock()

		if first { // Set is empty and has no values so is no longer in use.
			c.release(ctx, path)
		}
	}

	return result, nil
}

func (c *SimpleCacher) ReleaseSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
	hash bitcoin.Hash32) error {

	pathID := hashPathID(hash)
	path := setPath(pathPrefix, pathID)

	c.itemsLock.Lock()
	item, exists := c.items[path]
	if !exists {
		c.itemsLock.Unlock()
		panic(fmt.Sprintf("Set item released when not in cache: %s", path))
	}

	set := item.value.(*cacheSet)
	set.Lock()
	// if valueIsModified {
	// 	set.MarkModified()
	// }
	if set.extraUsers > 0 {
		set.extraUsers--
		set.Unlock()
		c.itemsLock.Unlock()
	} else {
		set.Unlock()
		c.itemsLock.Unlock()
		c.release(ctx, path)
	}
	return nil
}

func (c *SimpleCacher) ReleaseMultiSetValue(ctx context.Context, typ reflect.Type,
	pathPrefix string, hashes []bitcoin.Hash32) error {

	for _, hash := range hashes {
		if err := c.ReleaseSetValue(ctx, typ, pathPrefix, hash); err != nil {
			return errors.Wrapf(err, "set value hash: %s", hash)
		}
	}

	return nil
}

// cacheSet represents a set of values that are all stored in one storage object (s3 object, file).
type cacheSet struct {
	typ reflect.Type

	pathPrefix string
	pathID     [2]byte

	values map[bitcoin.Hash32]SetValue

	// extraUsers tracks when there are multiple values from a set used from a single cache user on
	// the set object. For example, when a mulitple get or add has two items in the same set so the
	// set object only had one user added for one get operation, but two values within the set were
	// given out and there will be two release set value calls.
	extraUsers uint

	isModified atomic.Value
	sync.Mutex
}

type cacheSets []*cacheSet

func (set *cacheSet) Initialize() {
	set.isModified.Store(false)
}

func (set *cacheSet) IsModified() bool {
	return set.isModified.Load().(bool)
}

func (set *cacheSet) MarkModified() {
	set.isModified.Store(true)
}

func (set *cacheSet) GetModified() bool {
	return set.isModified.Swap(false).(bool)
}

func (set *cacheSet) path() string {
	return setPath(set.pathPrefix, set.pathID)
}

func (set *cacheSet) CacheCopy() Value {
	copy := &cacheSet{
		typ:        set.typ,
		pathPrefix: set.pathPrefix,
		pathID:     set.pathID,
		values:     make(map[bitcoin.Hash32]SetValue),
		extraUsers: 0,
	}
	copy.isModified.Store(true)

	for hash, value := range set.values {
		value.Lock()
		copy.values[hash] = value.CacheSetCopy()
		value.Unlock()
	}

	return copy
}

func (set *cacheSet) Serialize(w io.Writer) error {
	if err := binary.Write(w, endian, cacheSetVersion); err != nil {
		return errors.Wrap(err, "version")
	}

	if err := binary.Write(w, endian, uint64(len(set.values))); err != nil {
		return errors.Wrap(err, "size")
	}

	for _, value := range set.values {
		if err := value.Serialize(w); err != nil {
			return errors.Wrap(err, "value")
		}
	}

	return nil
}

func (set *cacheSet) Deserialize(r io.Reader) error {
	var version uint8
	if err := binary.Read(r, endian, &version); err != nil {
		return errors.Wrap(err, "version")
	}

	if version != 0 {
		return fmt.Errorf("Unsupported version : %d", version)
	}

	var count uint64
	if err := binary.Read(r, endian, &count); err != nil {
		return errors.Wrap(err, "count")
	}

	set.values = make(map[bitcoin.Hash32]SetValue)
	for i := uint64(0); i < count; i++ {
		itemValue := reflect.New(set.typ.Elem())
		valueInterface := itemValue.Interface()
		value := valueInterface.(SetValue)

		if err := value.Deserialize(r); err != nil {
			return errors.Wrap(err, "value")
		}

		hash := value.Hash()
		set.values[hash] = value
	}

	return nil
}

func (sets *cacheSets) add(pathPrefix string, typ reflect.Type, value SetValue) {
	hash := value.Hash()
	pathID := hashPathID(hash)

	for _, set := range *sets {
		set.Lock()
		if pathPrefix != set.pathPrefix {
			set.Unlock()
			continue
		}
		if !bytes.Equal(set.pathID[:], pathID[:]) {
			set.Unlock()
			continue
		}

		set.values[hash] = value
		set.Unlock()
		return
	}

	set := &cacheSet{
		typ:        typ,
		pathPrefix: pathPrefix,
		pathID:     pathID,
		values:     make(map[bitcoin.Hash32]SetValue),
	}
	set.isModified.Store(true)
	value.ProvideMarkModified(set.MarkModified)

	set.values[hash] = value
	*sets = append(*sets, set)
}

func (sets *cacheSets) getSet(pathID [2]byte) (*cacheSet, int) {
	for i, set := range *sets {
		set.Lock()
		if !bytes.Equal(set.pathID[:], pathID[:]) {
			set.Unlock()
			continue
		}

		set.Unlock()
		return set, i
	}

	return nil, 0
}

func hashPathID(hash bitcoin.Hash32) [2]byte {
	var pathID [2]byte
	copy(pathID[:], hash[:])
	return pathID
}

func setPath(pathPrefix string, pathID [2]byte) string {
	return fmt.Sprintf("%s/%x", pathPrefix, pathID)
}

func pathIDFromPath(path string) ([2]byte, error) {
	var result [2]byte

	parts := strings.Split(path, "/")
	l := len(parts)
	if l < 2 {
		return result, fmt.Errorf("Not enough parts: %s", path)
	}

	lastPart := parts[l-1]
	b, err := hex.DecodeString(lastPart)
	if err != nil {
		return result, errors.Wrap(err, "hex")
	}

	if len(b) != 2 {
		fmt.Errorf("Wrong size path ID: %s", path)
	}

	copy(result[:], b)
	return result, nil
}

func stringExists(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}

	return false
}

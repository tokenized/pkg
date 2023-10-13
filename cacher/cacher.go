package cacher

import (
	"context"
	"io"
	"reflect"

	"github.com/tokenized/pkg/bitcoin"
)

// Cacher is an interface for a system that pulls items from storage and retains them in the cache
// while they are being used, then writes them back to storage. It ensures only one instance of each
// item exists in the cache at once.
type Cacher interface {
	Add(ctx context.Context, typ reflect.Type, path string, value Value) (Value, error)
	AddMulti(ctx context.Context, typ reflect.Type, paths []string, values []Value) ([]Value, error)
	Get(ctx context.Context, typ reflect.Type, path string) (Value, error)
	GetMulti(ctx context.Context, typ reflect.Type, paths []string) ([]Value, error)
	List(ctx context.Context, pathPrefix, regExPath string) ([]string, error)
	CopyRecursive(ctx context.Context, fromPathPrefix, toPathPrefix string) error
	Save(ctx context.Context, path string, value Value)
	AddUser(ctx context.Context, path string)
	Release(ctx context.Context, path string)
	IsEmpty(ctx context.Context) bool // true if there are not items retained in "memory"

	// Sets
	AddSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		value SetValue) (SetValue, error)
	AddMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		values []SetValue) ([]SetValue, error)
	GetSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		hash bitcoin.Hash32) (SetValue, error)
	GetMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		hashes []bitcoin.Hash32) ([]SetValue, error)
	ListMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string) ([]SetValue, error)
	ReleaseSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		hash bitcoin.Hash32) error
	ReleaseMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		hashes []bitcoin.Hash32) error
}

// Value represents the value of an item that can be used via a cacher.
type Value interface {
	// Initializes any values that must be initialized.
	Initialize()

	// IsModified returns true if the value has been marked modified, but does not clear the
	// modified flag.
	IsModified() bool

	// MarkModified sets a modified flag so that a value will be saved to storage before being
	// removed from the cache.
	MarkModified()

	// GetModified returns true if the value has been modified and clears the modified flag.
	GetModified() bool

	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error

	// CacheCopy creates an independent copy of the value. IsModified should be initialized to false
	// because the item will be new, so will be saved.
	CacheCopy() Value

	// All functions will be called by cache while the object is locked.
	Lock()
	Unlock()
}

type MarkModified func()

// SetValue represents the value of an item that belongs to a larger set of items that can be used
// via a cacher.
type SetValue interface {
	// Object has been modified since last write to storage.
	// IsModified() bool
	// ClearModified()

	// ProvideMarkModified is called on each set value before being returned from the cache. It
	// provides a function that must be used to mark when the set value is modified.
	ProvideMarkModified(markModified MarkModified)

	// Storage path and serialization.
	Hash() bitcoin.Hash32
	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error

	CacheSetCopy() SetValue

	// All functions will be called by cache while the object is locked.
	Lock()
	Unlock()
}

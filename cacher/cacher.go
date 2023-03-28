package cacher

import (
	"context"
	"io"
	"reflect"

	"github.com/tokenized/pkg/bitcoin"
)

// Cacher is an interface for a system that pulls items from storage and retains them in the cache
// while they are being used, then writes them back to storage. It is responsible for ensuring only
// one instance of each item exists in the cache at once.
type Cacher interface {
	Add(ctx context.Context, typ reflect.Type, path string, value Value) (Value, error)
	AddMulti(ctx context.Context, typ reflect.Type, paths []string, values []Value) ([]Value, error)
	Get(ctx context.Context, typ reflect.Type, path string) (Value, error)
	GetMulti(ctx context.Context, typ reflect.Type, paths []string) ([]Value, error)
	CopyRecursive(ctx context.Context, fromPathPrefix, toPathPrefix string) error
	Save(ctx context.Context, path string, value Value)
	AddUser(ctx context.Context, path string)
	Release(ctx context.Context, path string)

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
	ReleaseSetValue(ctx context.Context, typ reflect.Type, pathPrefix string, hash bitcoin.Hash32,
		valueIsModified bool) error
	ReleaseMultiSetValue(ctx context.Context, typ reflect.Type, pathPrefix string,
		hashes []bitcoin.Hash32, valueIsModified []bool) error
}

// Value represents the value of an item that can be used via a cacher.
type Value interface {
	// Object has been modified since last write to storage.
	IsModified() bool
	MarkModified()
	ClearModified()

	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error

	// CacheCopy creates an independent copy of the value. IsModified should be initialized to false
	// because the item will be new, so will be saved.
	CacheCopy() Value

	// All functions will be called by cache while the object is locked.
	Lock()
	Unlock()
}

// SetValue represents the value of an item that belongs to a larger set of items that can be used
// via a cacher.
type SetValue interface {
	// Object has been modified since last write to storage.
	IsModified() bool
	ClearModified()

	// Storage path and serialization.
	Hash() bitcoin.Hash32
	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error

	CacheSetCopy() SetValue

	// All functions will be called by cache while the object is locked.
	Lock()
	Unlock()
}

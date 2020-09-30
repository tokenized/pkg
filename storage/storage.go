package storage

import (
	"context"
	"strings"
)

// Storage is the interface combining all storage interfaces.
type Storage interface {
	ReadWriter
	Remover
	Searcher
	Clearer
	List
}

// ReadWriter interface combines the Reader and Writer interface.
type ReadWriter interface {
	Reader
	Writer
}

// Reader interface is for retrieving items from the store.
type Reader interface {
	Read(context.Context, string) ([]byte, error)
}

// Writer interface is for adding or updating an item to the store.
type Writer interface {
	Write(context.Context, string, []byte, *Options) error
}

// Remover interface is for removing an item from storage.
type Remover interface {
	Remove(context.Context, string) error
}

// Searcher interface is for retrieving multiple items.
type Searcher interface {
	Search(context.Context, map[string]string) ([][]byte, error)
}

type Clearer interface {
	Clear(context.Context, map[string]string) error
}

type List interface {
	List(context.Context, string) ([]string, error)
}

func CreateStorage(bucket, root string, maxRetries, retryDelay int) Storage {
	config := Config{
		Bucket:     bucket,
		Root:       root,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
	}

	if strings.ToLower(config.Bucket) == "standalone" {
		return NewFilesystemStorage(config)
	} else if strings.ToLower(config.Bucket) == "mock" {
		return NewMockStorage()
	} else {
		return NewS3Storage(config)
	}
}

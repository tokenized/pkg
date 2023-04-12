package storage

import (
	"path/filepath"
	"testing"
)

func Test_FileSystem_Interface(t *testing.T) {
	store := NewFilesystemStorage(Config{
		Root:   "/tmp",
		Bucket: "test-xxxx",
	})

	testIsStorage(store)
}

func TestFileSystem_buildPath(t *testing.T) {
	config := Config{
		Root:   "/tmp",
		Bucket: "test-xxxx",
	}

	store := NewFilesystemStorage(config)

	key := "foo"

	got := store.buildPath(key)

	want := filepath.FromSlash("/tmp/test-xxxx/foo")

	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

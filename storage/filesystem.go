package storage

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// FilesystemStorage implements the Storage interface for interacting with
// the local filesystem.
type FilesystemStorage struct {
	Config Config
}

// NewFilesystemStorage implements the Storage interface for simple S3 like
// file system interactions.
func NewFilesystemStorage(config Config) *FilesystemStorage {
	return &FilesystemStorage{
		Config: config,
	}
}

// Write will write the data to the key in the S3 Bucket.
func (f *FilesystemStorage) Write(ctx context.Context,
	key string,
	body []byte,
	options *Options) error {

	// make sure that the Options argument is valid
	if options == nil {
		opts := NewOptions()
		options = &opts
	}

	filename := f.buildPath(key)

	// make sure directory exists.
	dir := filepath.Dir(filename)

	if err := f.ensureExists(dir, nil); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	if _, err := file.Write(body); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return nil
}

func (f *FilesystemStorage) StreamWrite(ctx context.Context, key string, r io.ReadSeeker) error {
	filename := f.buildPath(key)

	// make sure directory exists.
	dir := filepath.Dir(filename)

	if err := f.ensureExists(dir, nil); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, r); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return nil
}

// Read reads the data from a file on the local filesystem.
func (f *FilesystemStorage) Read(ctx context.Context, key string) ([]byte, error) {
	filename := f.buildPath(key)

	// check for existence of file
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, ErrNotFound
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return data, err
	}

	return data, nil
}

func (f *FilesystemStorage) ReadRange(ctx context.Context, key string,
	start, end int64) ([]byte, error) {

	r, err := f.StreamReadRange(ctx, key, start, end)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (f *FilesystemStorage) StreamRead(ctx context.Context, key string) (io.ReadCloser, error) {
	return f.StreamReadRange(ctx, key, 0, 0)
}

func (f *FilesystemStorage) StreamReadRange(ctx context.Context, key string,
	start, end int64) (io.ReadCloser, error) {

	filename := f.buildPath(key)

	// check for existence of file
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, ErrNotFound
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	if start != 0 {
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return nil, err
		}
	}

	if end == 0 {
		return file, nil
	}

	return &limitedReadCloser{
		lr: &io.LimitedReader{
			R: file,
			N: end - start,
		},
		closer: file,
	}, nil
}

type limitedReadCloser struct {
	lr     *io.LimitedReader
	closer io.Closer
}

func (lrc *limitedReadCloser) Read(b []byte) (int, error) {
	return lrc.lr.Read(b)
}

func (lrc *limitedReadCloser) Close() error {
	return lrc.closer.Close()
}

// Remove removes the object stored at key, in the S3 Bucket.
func (f *FilesystemStorage) Remove(ctx context.Context, key string) error {
	filename := f.buildPath(key)

	err := os.RemoveAll(filename)
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func (f *FilesystemStorage) Copy(ctx context.Context, fromKey, toKey string) error {

	fromFilename := f.buildPath(fromKey)

	// check for existence of file
	if _, err := os.Stat(fromFilename); os.IsNotExist(err) {
		return ErrNotFound
	}

	fromFile, err := os.Open(fromFilename)
	if err != nil {
		return errors.Wrap(err, "open source")
	}

	toFilename := f.buildPath(toKey)

	// make sure directory exists.
	dir := filepath.Dir(toFilename)

	if err := f.ensureExists(dir, nil); err != nil {
		return errors.Wrap(err, "destination directory")
	}

	toFile, err := os.Create(toFilename)
	if err != nil {
		return errors.Wrap(err, "open destination")
	}

	chunk := make([]byte, 1024)
	for {
		n, err := fromFile.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "read")
		}

		if _, err := toFile.Write(chunk[:n]); err != nil {
			return errors.Wrap(err, "write")
		}
	}

	// Added in go1.15
	// if _, err := toFile.ReadFrom(fromFile); err != nil {
	// 	return errors.Wrap(err, "write")
	// }

	if err := toFile.Close(); err != nil {
		return errors.Wrap(err, "close destination")
	}

	if err := fromFile.Close(); err != nil {
		return errors.Wrap(err, "close source")
	}

	return nil
}

// All returns all objects in the store, from a given path.
//
// The path can be empty.
func (f *FilesystemStorage) Search(ctx context.Context,
	query map[string]string) ([][]byte, error) {

	path := query["path"]

	dir := f.buildPath(path)

	if err := f.ensureExists(dir, nil); err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	objects := [][]byte{}

	for _, info := range files {
		var filePath string
		if len(path) > 0 {
			filePath = strings.Join([]string{path, info.Name()}, "/")
		} else {
			filePath = info.Name()
		}
		b, err := f.Read(ctx, filePath)
		if err != nil {
			return nil, err
		}

		objects = append(objects, b)
	}

	return objects, nil
}

func (f *FilesystemStorage) Clear(ctx context.Context, query map[string]string) error {
	path := query["path"]

	dir := f.buildPath(path)

	if err := f.ensureExists(dir, nil); err != nil {
		return err
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, info := range files {
		var filePath string
		if len(path) > 0 {
			filePath = strings.Join([]string{path, info.Name()}, "/")
		} else {
			filePath = info.Name()
		}
		err := f.Remove(ctx, filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *FilesystemStorage) List(ctx context.Context, path string) ([]string, error) {

	dir := f.buildPath(path)

	if err := f.ensureExists(dir, nil); err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(files), len(files))

	for i, info := range files {
		var filePath string
		if len(path) > 0 {
			filePath = strings.Join([]string{path, info.Name()}, "/")
		} else {
			filePath = info.Name()
		}

		keys[i] = filePath
	}

	return keys, nil
}

func (f *FilesystemStorage) buildPath(key string) string {
	parts := []string{
		f.Config.Root,
		f.Config.Bucket,
	}

	if len(key) > 0 {
		parts = append(parts, key)
	}

	s := strings.Join(parts, "/")

	return filepath.FromSlash(s)
}

func (f *FilesystemStorage) ensureExists(dir string, options *Options) error {
	if options == nil {
		opts := NewOptions()
		options = &opts
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, options.DirMode); err != nil {
			return err
		}
	}

	return nil
}

package threads

import (
	"io"
	"sync"
)

type ReadCloser struct {
	reader   io.Reader
	isClosed bool

	sync.Mutex
}

func NewReadCloser(r io.Reader) *ReadCloser {
	return &ReadCloser{
		reader: r,
	}
}

func (r *ReadCloser) Read(b []byte) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.isClosed {
		return 0, io.EOF
	}

	return r.reader.Read(b)
}

func (r *ReadCloser) Close() error {
	r.Lock()
	defer r.Unlock()

	r.isClosed = true
	return nil
}

package storage

import (
	"io"
	"sync"
)

// Buffer implements io.Writer, io.Reader, io.Seeker, and io.Closer.
type Buffer struct {
	data     []byte
	offset   int64
	isClosed bool

	lock sync.Mutex
}

func NewBuffer() *Buffer {
	return &Buffer{}
}

func (b *Buffer) Read(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	n := copy(p, b.data[b.offset:])
	b.offset += int64(n)

	if n == 0 && len(p) > 0 {
		if b.isClosed {
			return n, io.EOF
		}
	}

	return n, nil
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.isClosed {
		return 0, io.ErrClosedPipe
	}

	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *Buffer) Seek(offset int64, whence int) (int64, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	length := int64(len(b.data))

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = b.offset + offset
	case io.SeekEnd:
		newOffset = length + offset
	}

	if newOffset < 0 {
		return b.offset, io.ErrShortBuffer
	}

	if newOffset > length {
		return b.offset, io.ErrShortBuffer
	}

	b.offset = newOffset

	return b.offset, nil
}

func (b *Buffer) Close() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.isClosed = true
	return nil
}

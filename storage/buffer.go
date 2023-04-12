package storage

import (
	"errors"
	"io"
	"sync"
)

// Buffer implements io.Writer, io.Reader, io.Seeker, and io.Closer.
type Buffer struct {
	data         []byte
	offset       int64
	isClosed     bool
	closeChannel chan interface{}

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

	var length int64
	var newOffset int64
	switch whence {
	case io.SeekStart:
		length = int64(len(b.data))
		newOffset = offset
	case io.SeekCurrent:
		length = int64(len(b.data))
		newOffset = b.offset + offset
	case io.SeekEnd:
		b.lock.Unlock()
		if err := b.waitForClose(); err != nil {
			return 0, err
		}
		b.lock.Lock()

		length = int64(len(b.data))
		newOffset = length + offset
	}

	defer b.lock.Unlock()

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
	if b.closeChannel != nil {
		close(b.closeChannel)
		b.closeChannel = nil
	}
	return nil
}

func (b *Buffer) waitForClose() error {
	b.lock.Lock()
	if b.isClosed {
		b.lock.Unlock()
		return nil
	}
	if b.closeChannel != nil {
		b.lock.Unlock()
		return errors.New("already waiting for close")
	}
	closeChannel := make(chan interface{})
	b.closeChannel = closeChannel
	b.lock.Unlock()

	select {
	case <-closeChannel:
		return nil
	}
}

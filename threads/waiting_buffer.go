package threads

import (
	"io"
	"sync"
	"time"
)

// WaitingBuffer is a buffer that can be written to from one thread and read from another thread.
// Reads will wait for enough data to be written or for the buffer to be closed. It will
// automatically discard data that has already been read.
type WaitingBuffer struct {
	buffer    [][]byte
	isClosed  bool
	byteCount uint64
	lock      sync.Mutex
}

func NewWaitingBuffer() *WaitingBuffer {
	return &WaitingBuffer{}
}

func (wb WaitingBuffer) ByteCount() uint64 {
	wb.lock.Lock()
	defer wb.lock.Unlock()

	return wb.byteCount
}

func (wb *WaitingBuffer) Read(b []byte) (int, error) {
	offset := 0
	l := len(b)
	for {
		wb.lock.Lock()

		byteCount, err := wb.read(b)
		if err == nil {
			offset += byteCount
			if offset == l {
				wb.lock.Unlock()
				return offset, nil
			}
			b = b[byteCount:]
		} else if err == io.EOF {
			offset += byteCount
			b = b[byteCount:]
		} else {
			wb.lock.Unlock()
			return offset, err
		}

		if wb.isClosed {
			wb.lock.Unlock()
			return offset, io.EOF
		}

		wb.lock.Unlock()
		time.Sleep(1) // sleep to wait for more writes
	}
}

func (wb *WaitingBuffer) read(b []byte) (int, error) {
	readOffset := 0
	writeOffset := 0
	for {
		if len(wb.buffer) == 0 {
			return readOffset, io.EOF
		}

		destLen := len(b) - writeOffset
		sourceLen := len(wb.buffer[0])
		copyLen := copy(b[writeOffset:], wb.buffer[0])
		readOffset += copyLen

		if copyLen == sourceLen {
			// byte slice is consumed so remove it
			wb.buffer = wb.buffer[1:]
		} else {
			// byte slice is partially consumed so truncate it
			wb.buffer[0] = wb.buffer[0][copyLen:]
		}

		if copyLen == destLen {
			// destination is full so finish
			return readOffset, nil
		}

		// move byte slice up for the next write
		writeOffset += copyLen
		// b = b[copyLen:]
	}
}

func (wb *WaitingBuffer) Write(b []byte) (n int, err error) {
	wb.lock.Lock()
	defer wb.lock.Unlock()

	l := len(b)
	if wb.isClosed {
		return l, io.ErrClosedPipe
	}

	// Copy byte slice
	wb.byteCount += uint64(l)
	newBytes := make([]byte, l)
	copy(newBytes, b)

	// Append byte slice to list
	wb.buffer = append(wb.buffer, newBytes)

	return l, nil
}

func (wb *WaitingBuffer) Close() error {
	wb.lock.Lock()
	defer wb.lock.Unlock()

	wb.isClosed = true
	return nil
}

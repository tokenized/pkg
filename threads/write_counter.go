package threads

// WriteCounter counts the number of bytes written to it.
type WriteCounter struct {
	count uint64
}

func NewWriteCounter() *WriteCounter {
	return &WriteCounter{
		count: 0,
	}
}

// Write implements the io.Writer interface.
func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.count += uint64(n)
	return n, nil
}

func (wc *WriteCounter) Count() uint64 {
	return wc.count
}

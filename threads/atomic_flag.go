package threads

import (
	"sync/atomic"
)

// AtomicFlag is a flag that can be set and read in different threads safely. It's main use is to
// stop functions in threads that are looking for it.
type AtomicFlag struct {
	value atomic.Value
}

// NewAtomicFlag creates an unset atomic flag.
func NewAtomicFlag() *AtomicFlag {
	result := &AtomicFlag{}
	result.value.Store(uint64(0))
	return result
}

func (f *AtomicFlag) Set() {
	f.value.Store(uint64(1))
}

func (f *AtomicFlag) Clear() {
	f.value.Store(uint64(0))
}

func (f AtomicFlag) IsSet() bool {
	return f.value.Load().(uint64) != 0
}

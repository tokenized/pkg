package logger

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type WaitingWarning struct {
	active    bool
	interrupt chan interface{}

	sync.Mutex
}

// NewWaitingWarning creates a repeated warning message when waiting for something to complete.
// name is displayed in the log entry that is repeated until the process finishes.
// frequency is the number of seconds between log entries.
func NewWaitingWarning(ctx context.Context, frequency time.Duration, format string,
	values ...interface{}) *WaitingWarning {

	result := &WaitingWarning{
		active:    true,
		interrupt: make(chan interface{}),
	}

	// start thread
	go func() {
		runWaitWarning(ctx, fmt.Sprintf(format, values...), GetCaller(1), frequency,
			result.interrupt)
	}()

	return result
}

func runWaitWarning(ctx context.Context, name, caller string, frequency time.Duration,
	interrupt <-chan interface{}) {

	start := time.Now()
	for {
		select {
		case <-time.After(frequency):
			LogDepthWithFields(ctx, LevelWarn, caller, []Field{
				Timestamp("start", start.UnixNano()),
				MillisecondsFromNano("elapsed_ms", time.Since(start).Nanoseconds()),
			}, "Waiting for: %s", name)

		case <-interrupt:
			return
		}
	}
}

func (w *WaitingWarning) Cancel() {
	w.Lock()
	defer w.Unlock()

	if !w.active {
		return
	}

	close(w.interrupt)
	w.active = false
}

package logger

import (
	"context"
	"sync"
	"time"
)

type WaitingWarning struct {
	active    bool
	start     time.Time
	last      time.Time
	frequency float64 // seconds
	name      string
	caller    string

	lock sync.Mutex
}

// NewWaitingWarning creates a repeated warning message when waiting for something to complete.
// name is displayed in the log entry that is repeated until the process finishes.
// frequency is the number of seconds between log entries.
func NewWaitingWarning(ctx context.Context, name string, frequency float64) *WaitingWarning {
	result := &WaitingWarning{
		active:    true,
		start:     time.Now().UTC(),
		name:      name,
		caller:    GetCaller(1), // get caller now so the log entries show the warning creation
		frequency: frequency,
	}

	result.last = result.start

	// start thread
	go func() {
		result.run(ctx)
	}()

	return result
}

func (w *WaitingWarning) run(ctx context.Context) {
	for {
		if !w.check(ctx) {
			return
		}
		time.Sleep(100)
	}
}

func (w *WaitingWarning) check(ctx context.Context) bool {
	w.lock.Lock()
	defer w.lock.Unlock()

	if !w.active {
		return false
	}

	now := time.Now()
	us := time.Since(w.start).Nanoseconds()
	s := now.Sub(w.last).Seconds()
	if s > w.frequency {
		LogDepthWithFields(ctx, LevelWarn, w.caller, []Field{
			Timestamp("start", w.start.UnixNano()),
			MillisecondsFromNano("elapsed_ms", us),
		}, "Waiting for: %s", w.name)
		w.last = now
	}

	return true
}

func (w *WaitingWarning) Cancel() {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.active = false
}

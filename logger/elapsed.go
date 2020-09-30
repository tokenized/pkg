package logger

import (
	"context"
	"time"
)

// Elapsed write elapsed time in milliseconds to the Logger.
// Must be called with "defer" in front of it and with time.Now() as the time.
func Elapsed(ctx context.Context, start time.Time, format string, values ...interface{}) {
	// get the elapsed time in milliseconds
	ms := float64(time.Since(start).Nanoseconds()) / float64(time.Millisecond)

	values = append(values, ms)

	LogDepth(ctx, LevelInfo, 1, format+" : %0.3f ms", values...)
}

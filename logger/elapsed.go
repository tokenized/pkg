package logger

import (
	"context"
	"time"
)

// Elapsed write elapsed time in milliseconds to the Logger.
// Should be called at the beginning of a function with "defer" in front of it and with time.Now()
// as the "start" time.
func Elapsed(ctx context.Context, start time.Time, format string, values ...interface{}) {
	LogDepthWithFields(ctx, LevelInfo, GetCaller(1), []Field{
		MillisecondsFromNano("elapsed_ms", time.Since(start).Nanoseconds()),
	}, format, values...)
}

func ElapsedWithFields(ctx context.Context, start time.Time, fields []Field, format string,
	values ...interface{}) {

	fields = append(fields, MillisecondsFromNano("elapsed_ms", time.Since(start).Nanoseconds()))
	LogDepthWithFields(ctx, LevelInfo, GetCaller(1), fields, format, values...)
}

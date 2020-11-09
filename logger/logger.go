package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Logger allows you to control logging with message levels and subsystem controls.
// Use the "Include" flags in the Format field to specify which fields should be included in each
//   log message.
// Subsystem log entries can be enabled per subsystem.
// For example the parent package can specify if they want to see logs from a subsystem and how
//   they want to see them.
//
// Sample Setup:
// // Create a log config and set it up.
// logConfig := logger.NewDevelopmentConfig()
// // Log to stderr (default) and main.log.
// // To only log to main.log call SetFile instead of AddFile.
// logConfig.Main.AddFile("./tmp/main.log")
// logConfig.Main.Format |= logger.IncludeSystem
// logConfig.EnableSubSystem(spynode.SubSystem)
//
// // Attach the log config to the context.
// ctx := logger.ContextWithLogConfig(context.Background(), logConfig)
//

// Keys for context key/pairs
type loggerkey int

const (
	key loggerkey = 1
)

type Level int

const (
	LevelDebug   Level = -2
	LevelVerbose Level = -1
	LevelInfo    Level = 0
	LevelWarn    Level = 1
	LevelError   Level = 2
	LevelFatal   Level = 3 // Calls exit
	LevelPanic   Level = 4 // Calls panic
)

// Log entry formatting (which prefix fields to include)
const (
	IncludeDate   = 0x01 // date in the local time zone: 2018/01/01
	IncludeTime   = 0x02 // time in the local time zone: 06:54:32
	IncludeMicro  = 0x04 // microseconds .123123
	IncludeFile   = 0x08 // file name and line number
	IncludeSystem = 0x10 // system name
	IncludeLevel  = 0x20 // level of log entry
)

// ContextWithLogConfig returns a context with the specified logging config attached.
func ContextWithLogConfig(ctx context.Context, config *Config) context.Context {
	return context.WithValue(ctx, key, *config)
}

// ContextWithNoLogger returns a context with no logging
func ContextWithNoLogger(ctx context.Context) context.Context {
	return context.WithValue(ctx, key, *NewEmptyConfig())
}

// ContextWithLogSubSystem returns a context with the logging subsystem attached.
func ContextWithLogSubSystem(ctx context.Context, subsystem string) context.Context {
	configValue := ctx.Value(key)
	if configValue == nil {
		return context.WithValue(ctx, key, *NewEmptyConfig())
	}

	config, ok := configValue.(Config)
	if !ok {
		return context.WithValue(ctx, key, *NewEmptyConfig())
	}

	include, includeExists := config.IncludedSubSystems[subsystem]
	if !includeExists || !include {
		// Empty logger for this subsystem, but leave the rest of the configuration so it can pop
		// back up to the main config if it calls through to ContextWithOutLogSubSystem.
		n, _ := newEmptySystemConfig()
		config.Active = *n
		return context.WithValue(ctx, key, config)
	}

	// Log to subsystem specific config
	subConfig, subExists := config.SubSystems[subsystem]
	if subExists {
		config.Active = *subConfig
		return context.WithValue(ctx, key, config)
	}

	config.Active.logger = config.Active.logger.Named(subsystem)
	return context.WithValue(ctx, key, config)
}

// ContextWithOutLogSubSystem returns a context with the logging subsystem cleared. Used when a
// context is passed back from a subsystem.
func ContextWithOutLogSubSystem(ctx context.Context) context.Context {
	configValue := ctx.Value(key)
	if configValue == nil {
		// Config not specified. Use default config.
		return context.WithValue(ctx, key, *NewConfig(false, false, ""))
	}

	config, ok := configValue.(Config)
	if !ok {
		// Config invalid. Use default config.
		return context.WithValue(ctx, key, *NewConfig(false, false, ""))
	}

	config.Active = *config.Main
	return context.WithValue(ctx, key, config)
}

// ContextWithLogTrace returns a context with a trace field added to the logger.
func ContextWithLogTrace(ctx context.Context, trace string) context.Context {
	var config *Config

	configValue := ctx.Value(key)
	if configValue != nil {
		contextConfig, ok := configValue.(Config)
		if ok {
			config = &contextConfig
		}
	}

	if config == nil {
		config = NewConfig(false, false, "")
	}

	if config.Active.logger == nil {
		return ctx
	}

	config.Active.logger = config.Active.logger.With(zap.String("trace", trace))
	return context.WithValue(ctx, key, *config)
}

func GetLogger(ctx context.Context) *zap.Logger {
	configValue := ctx.Value(key)
	if configValue == nil {
		// Config not specified. Use default config.
		sc, _ := newSystemConfig(false, false, "")
		return sc.logger
	}

	config, ok := configValue.(Config)
	if !ok {
		// Config invalid. Use default config.
		sc, _ := newSystemConfig(false, false, "")
		return sc.logger
	}

	return config.Active.logger
}

// Debug adds a debug level entry to the log.
func Debug(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelDebug, 1, format, values...)
}

// Verbose adds a verbose level entry to the log.
func Verbose(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelVerbose, 1, format, values...)
}

// Info adds a info level entry to the log.
func Info(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelInfo, 1, format, values...)
}

// Warn adds a warn level entry to the log.
func Warn(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelWarn, 1, format, values...)
}

// Error adds a error level entry to the log.
func Error(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelError, 1, format, values...)
}

// Fatal adds a fatal level entry to the log and then calls os.Exit(1).
func Fatal(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelFatal, 1, format, values...)
}

// Panic adds a panic level entry to the log and then calls panic().
func Panic(ctx context.Context, format string, values ...interface{}) error {
	return LogDepth(ctx, LevelPanic, 1, format, values...)
}

// Log an entry to the main Outputs if:
//   There is no subsystem specified or if the current subsystem is included in the attached
//     Config.IncludedSubSystems.
//   And the level is equal to or above the specified minimum logging level.
// Logs to the Config.SubSystems if the level is above minimum.
func Log(ctx context.Context, level Level, format string, values ...interface{}) error {
	return LogDepth(ctx, level, 1, format, values...)
}

// LogDepth is the same as Log, but the number of levels above the current call in the stack from
// which to get the file name/line of code can be specified as depth.
func LogDepth(ctx context.Context, level Level, depth int, format string, values ...interface{}) error {
	l := GetLogger(ctx).WithOptions(zap.AddCallerSkip(depth + 1)).Sugar()

	switch level {
	case LevelDebug:
		l.Debugf(format, values...)
		return nil
	case LevelVerbose:
		l.Debugf(format, values...) // No zap verbose level
		return nil
	case LevelInfo:
		l.Infof(format, values...)
		return nil
	case LevelWarn:
		l.Warnf(format, values...)
		return nil
	case LevelError:
		l.Errorf(format, values...)
		return nil
	case LevelFatal:
		l.Fatalf(format, values...)
		return nil
	case LevelPanic:
		l.Panicf(format, values...)
		return nil
	}

	return fmt.Errorf("Unknown log level %d", level)
}

// DebugWithZapFields adds a debug level entry to the log with the included zap fields.
func DebugWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelDebug, 1, fields, format, values...)
}

// VerboseWithZapFields adds a verbose level entry to the log with the included zap fields.
func VerboseWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelVerbose, 1, fields, format, values...)
}

// InfoWithZapFields adds a info level entry to the log with the included zap fields.
func InfoWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelInfo, 1, fields, format, values...)
}

// WarnWithZapFields adds a warn level entry to the log with the included zap fields.
func WarnWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelWarn, 1, fields, format, values...)
}

// ErrorWithZapFields adds a error level entry to the log with the included zap fields.
func ErrorWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelError, 1, fields, format, values...)
}

// FatalWithZapFields adds a fatal level entry to the log with the included zap fields.
func FatalWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelFatal, 1, fields, format, values...)
}

// PanicWithZapFields adds a panic level entry to the log with the included zap fields.
func PanicWithZapFields(ctx context.Context, fields []zap.Field, format string,
	values ...interface{}) error {

	return LogDepthWithZapFields(ctx, LevelPanic, 1, fields, format, values...)
}

// LogDepth is the same as Log, but the number of levels above the current call in the stack from
// which to get the file name/line of code can be specified as depth with the included zap fields.
func LogDepthWithZapFields(ctx context.Context, level Level, depth int, fields []zap.Field,
	format string, values ...interface{}) error {

	l := GetLogger(ctx).WithOptions(zap.AddCallerSkip(depth + 1))
	for _, field := range fields {
		l = l.With(field)
	}

	ls := l.Sugar()

	switch level {
	case LevelDebug:
		ls.Debugf(format, values...)
		return nil
	case LevelVerbose:
		ls.Debugf(format, values...) // No zap verbose level
		return nil
	case LevelInfo:
		ls.Infof(format, values...)
		return nil
	case LevelWarn:
		ls.Warnf(format, values...)
		return nil
	case LevelError:
		ls.Errorf(format, values...)
		return nil
	case LevelFatal:
		ls.Fatalf(format, values...)
		return nil
	case LevelPanic:
		ls.Panicf(format, values...)
		return nil
	}

	return fmt.Errorf("Unknown log level %d", level)
}

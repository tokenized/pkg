package logger

import (
	"context"

	"github.com/pkg/errors"
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
	IncludeLevel     = 0x01 // level of log entry
	IncludeCaller    = 0x02 // file name and line number
	IncludeDate      = 0x04 // date in the local time zone: 2018/01/01
	IncludeTime      = 0x08 // time in the local time zone: 06:54:32
	IncludeMicro     = 0x10 // microseconds .123123
	IncludeTimeStamp = 0x20 // unix timestamp with microseconds
)

// ContextWithLogConfig returns a context with the specified logging config attached.
func ContextWithLogConfig(ctx context.Context, config Config) context.Context {
	return context.WithValue(ctx, key, config)
}

// ContextWithLogger returns a context with the specified logging attached.
func ContextWithLogger(ctx context.Context, isDevelopment, isText bool,
	filePath string) context.Context {
	return context.WithValue(ctx, key, NewConfig(isDevelopment, isText, filePath))
}

// ContextWithNoLogger returns a context with no logging
func ContextWithNoLogger(ctx context.Context) context.Context {
	return context.WithValue(ctx, key, NewEmptyConfig())
}

// ContextWithLogSubSystem returns a context with the logging subsystem attached.
func ContextWithLogSubSystem(ctx context.Context, subsystem string) context.Context {
	configValue := ctx.Value(key)
	if configValue == nil {
		return context.WithValue(ctx, key, NewEmptyConfig())
	}

	config, ok := configValue.(Config)
	if !ok {
		return context.WithValue(ctx, key, NewEmptyConfig())
	}

	include, includeExists := config.IncludedSubSystems[subsystem]
	if !includeExists || !include {
		// Empty logger for this subsystem, but leave the rest of the configuration so it can pop
		// back up to the main config if it calls through to ContextWithOutLogSubSystem.
		n, _ := newEmptySystemConfig()
		config = config.Copy()
		config.Active = n
		return context.WithValue(ctx, key, config)
	}

	// Log to subsystem specific config
	subConfig, subExists := config.SubSystems[subsystem]
	if subExists {
		config.Active = subConfig.Copy()
		return context.WithValue(ctx, key, config)
	}

	config.Active = config.Main.Copy()
	config.Active.addSubSystem(subsystem)
	return context.WithValue(ctx, key, config)
}

// ContextWithOutLogSubSystem returns a context with the logging subsystem cleared. Used when a
// context is passed back from a subsystem.
func ContextWithOutLogSubSystem(ctx context.Context) context.Context {
	configValue := ctx.Value(key)
	if configValue == nil {
		// Config not specified. Use default config.
		return context.WithValue(ctx, key, NewConfig(false, false, ""))
	}

	config, ok := configValue.(Config)
	if !ok {
		// Config invalid. Use default config.
		return context.WithValue(ctx, key, NewConfig(false, false, ""))
	}

	config.Active = config.Main.Copy()
	config.Active.removeSubSystem()
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
		newConfig := NewConfig(false, false, "")
		config = &newConfig
	}

	config.Active.addField(String("trace", trace))
	return context.WithValue(ctx, key, *config)
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
func LogDepth(ctx context.Context, level Level, depth int, format string,
	values ...interface{}) error {

	var config *systemConfig

	configValue := ctx.Value(key)
	if configValue != nil {
		contextConfig, ok := configValue.(Config)
		if ok {
			config = &contextConfig.Active
		}
	}

	if config == nil {
		newConfig, err := newSystemConfig(false, false, "")
		if err != nil {
			return errors.Wrap(err, "create default config")
		}
		config = &newConfig
	}

	return config.writeEntry(level, depth+1, nil, format, values...)
}

// DebugWithFields adds a debug level entry to the log with the included zap fields.
func DebugWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelDebug, 1, fields, format, values...)
}

// VerboseWithFields adds a verbose level entry to the log with the included zap fields.
func VerboseWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelVerbose, 1, fields, format, values...)
}

// InfoWithFields adds a info level entry to the log with the included zap fields.
func InfoWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelInfo, 1, fields, format, values...)
}

// WarnWithFields adds a warn level entry to the log with the included zap fields.
func WarnWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelWarn, 1, fields, format, values...)
}

// ErrorWithFields adds a error level entry to the log with the included zap fields.
func ErrorWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelError, 1, fields, format, values...)
}

// FatalWithFields adds a fatal level entry to the log with the included zap fields.
func FatalWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelFatal, 1, fields, format, values...)
}

// PanicWithFields adds a panic level entry to the log with the included zap fields.
func PanicWithFields(ctx context.Context, fields []Field, format string,
	values ...interface{}) error {

	return LogDepthWithFields(ctx, LevelPanic, 1, fields, format, values...)
}

// LogDepth is the same as Log, but the number of levels above the current call in the stack from
// which to get the file name/line of code can be specified as depth with the included zap fields.
func LogDepthWithFields(ctx context.Context, level Level, depth int, fields []Field,
	format string, values ...interface{}) error {

	var config *systemConfig

	configValue := ctx.Value(key)
	if configValue != nil {
		contextConfig, ok := configValue.(Config)
		if ok {
			config = &contextConfig.Active
		}
	}

	if config == nil {
		newConfig, err := newSystemConfig(false, false, "")
		if err != nil {
			return errors.Wrap(err, "create default config")
		}
		config = &newConfig
	}

	return config.writeEntry(level, depth+1, fields, format, values...)
}

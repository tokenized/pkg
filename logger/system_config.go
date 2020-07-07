package logger

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SystemConfig defines the configuration the main system or a subsystem with custom settings.
type SystemConfig struct {
	logger *zap.Logger
}

// NewProductionLogger creates a new logger with default production values.
// Logs info level and above to stderr.
func NewProductionLogger() (*SystemConfig, error) {
	l, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &SystemConfig{l}, nil
}

// NewDevelopmentLogger creates a new logger with default development values.
// Logs verbose level and above to stderr.
func NewDevelopmentLogger() (*SystemConfig, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)

	// Turn off stack trace logging for Warn level entries.
	l, err := config.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, err
	}

	return &SystemConfig{l}, nil
}

// NewProductionTextLogger a new logger with default production values.
// Logs info level and above to stderr.
func NewProductionTextLogger() (*SystemConfig, error) {
	l, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &SystemConfig{l}, nil
}

// NewDevelopmentTextLogger creates a new logger with default development values.
// Logs verbose level and above to stderr.
func NewDevelopmentTextLogger() (*SystemConfig, error) {
	l, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &SystemConfig{l}, nil
}

// NewEmptyLogger a new logger that doesn't log.
func NewEmptyLogger() (*SystemConfig, error) {
	return &SystemConfig{zap.NewNop()}, nil
}

// AddField adds a zap field to the existing log outputs
func (s *SystemConfig) AddField(f zapcore.Field) error {
	s.logger = s.logger.With(f)
	return nil
}

// AddName adds a name to the existing log outputs
func (s *SystemConfig) AddName(name string) error {
	s.logger = s.logger.Named(name)
	return nil
}

// AddFile adds a file to the existing log outputs
func (s *SystemConfig) AddFile(filePath string) error {
	w, _, err := zap.Open(filePath) // (zapcore.WriteSyncer, func(), error)
	if err != nil {
		return errors.Wrap(err, "open file")
	}

	s.logger = s.logger.WithOptions(zap.ErrorOutput(w))
	return nil
}

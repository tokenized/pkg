package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SystemConfig defines the configuration the main system or a subsystem with custom settings.
type systemConfig struct {
	logger *zap.Logger
}

// newSystemConfig creates a new logger system config.
// NOTE: isText doesn't work yet, but is meant to change from JSON to tab delimited.
func newSystemConfig(isDevelopment, isText bool, filePath string) (*systemConfig, error) {
	config := zap.NewProductionConfig()

	if len(filePath) > 0 {
		config.OutputPaths = []string{filePath}
	}

	if isDevelopment {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)

		// Turn off stack trace logging for Warn level entries.
		l, err := config.Build(zap.AddStacktrace(zapcore.ErrorLevel))
		if err != nil {
			return nil, err
		}

		return &systemConfig{l}, nil
	}

	l, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &systemConfig{l}, nil
}

// newEmptySystemConfig a new logger system config that doesn't log.
func newEmptySystemConfig() (*systemConfig, error) {
	return &systemConfig{zap.NewNop()}, nil
}

// AddField adds a zap field to the existing log outputs
func (s *systemConfig) addField(f zapcore.Field) error {
	s.logger = s.logger.With(f)
	return nil
}

// AddName adds a name to the existing log outputs
func (s *systemConfig) addName(name string) error {
	s.logger = s.logger.Named(name)
	return nil
}

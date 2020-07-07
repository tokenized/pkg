package logger

import (
	"sync"
)

// Config defines the logging configuration for the context it is attached to.
type Config struct {
	Active             *SystemConfig
	Main               *SystemConfig
	IncludedSubSystems map[string]bool          // If true, log in main log
	SubSystems         map[string]*SystemConfig // SubSystem specific loggers

	lock sync.Mutex
}

// Creates a new config with default production values.
//   Logs info level and above to stderr.
func NewProductionConfig() *Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]*SystemConfig),
	}

	result.Main, _ = NewProductionLogger()
	result.Active = result.Main
	return &result
}

// Creates a new config with default development values.
//   Logs debug level and above to stderr.
func NewDevelopmentConfig() *Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]*SystemConfig),
	}

	result.Main, _ = NewDevelopmentLogger()
	result.Active = result.Main
	return &result
}

// Creates a new config that doesn't log.
//   Logs info level and above to stderr.
func NewEmptyConfig() *Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]*SystemConfig),
	}

	result.Main, _ = NewEmptyLogger()
	result.Active = result.Main
	return &result
}

// Enables a subsytem to log to the main log
func (config *Config) EnableSubSystem(subsystem string) {
	config.lock.Lock()
	defer config.lock.Unlock()

	config.IncludedSubSystems[subsystem] = true
}

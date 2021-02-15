package logger

import (
	"fmt"
)

// Config defines the logging configuration for the context it is attached to.
type Config struct {
	Active             systemConfig
	Main               *systemConfig
	IncludedSubSystems map[string]bool          // If true, log in main log
	SubSystems         map[string]*systemConfig // SubSystem specific loggers
}

// NewConfig creates a new config with the specified values.
func NewConfig(isDevelopment, isText bool, filePath string) *Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]*systemConfig),
	}

	var err error
	result.Main, err = newSystemConfig(isDevelopment, isText, filePath)
	if err != nil {
		fmt.Printf("Failed to create log config : %s\n", err)
		return nil
	}

	result.Active = *result.Main
	return &result
}

// NewEmptyConfig creates a new config that doesn't log.
func NewEmptyConfig() *Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]*systemConfig),
	}

	var err error
	result.Main, err = newEmptySystemConfig()
	if err != nil {
		fmt.Printf("Failed to create log config : %s\n", err)
		return nil
	}

	result.Active = *result.Main
	return &result
}

// EnableSubSystem enables a subsytem to log to the main log
func (config *Config) EnableSubSystem(subsystem string) {
	config.IncludedSubSystems[subsystem] = true
}

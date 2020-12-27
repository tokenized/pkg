package logger

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

	result.Main, _ = newSystemConfig(isDevelopment, isText, filePath)
	result.Active = *result.Main
	return &result
}

// NewEmptyConfig creates a new config that doesn't log.
func NewEmptyConfig() *Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]*systemConfig),
	}

	result.Main, _ = newEmptySystemConfig()
	result.Active = *result.Main
	return &result
}

// EnableSubSystem enables a subsytem to log to the main log
func (config *Config) EnableSubSystem(subsystem string) {
	config.IncludedSubSystems[subsystem] = true
}

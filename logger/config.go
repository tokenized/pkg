package logger

// Config defines the logging configuration for the context it is attached to.
type Config struct {
	Main               systemConfig
	Active             systemConfig
	IncludedSubSystems map[string]bool         // If true, log in main log
	SubSystems         map[string]systemConfig // SubSystem specific loggers
}

func (c Config) Copy() Config {
	result := c
	result.IncludedSubSystems = make(map[string]bool)
	for k, v := range c.IncludedSubSystems {
		result.IncludedSubSystems[k] = v
	}
	result.SubSystems = make(map[string]systemConfig)
	for k, v := range c.SubSystems {
		result.SubSystems[k] = v
	}
	return result
}

// NewConfig creates a new config with the specified values.
func NewConfig(isDevelopment, isText bool, filePath string) Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]systemConfig),
	}

	var err error
	result.Main, err = newSystemConfig(isDevelopment, isText, filePath)
	if err != nil {
		return result
	}

	result.Active = result.Main.Copy()
	return result
}

// NewConfigFromSetup creates a new config from a setup config.
func NewConfigFromSetup(setup SetupConfig) Config {
	result := Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]systemConfig),
	}

	var err error
	result.Main, err = newSystemConfigFromSetup(setup)
	if err != nil {
		return result
	}

	result.Active = result.Main.Copy()
	return result
}

// NewEmptyConfig creates a new config that doesn't log.
func NewEmptyConfig() Config {
	return Config{
		IncludedSubSystems: make(map[string]bool),
		SubSystems:         make(map[string]systemConfig),
	}
}

// EnableSubSystem enables a subsytem to log to the main log
func (config *Config) EnableSubSystem(subsystem string) {
	config.IncludedSubSystems[subsystem] = true
}

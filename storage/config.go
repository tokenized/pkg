package storage

import "fmt"

const (
	// DefaultMaxRetries is the number of retries for a write operation
	DefaultMaxRetries = 2

	// DefaultRetryDelay is the number of milliseconds to wait before attempting a retry after a
	// failure.
	DefaultRetryDelay = 200
)

// Config holds all configuration for the Storage.
//
// Config is geared towards "bucket" style storage, where you have a
// specific root (the Bucket).
type Config struct {
	Bucket     string `default:"standalone" envconfig:"STORAGE_BUCKET" json:"bucket"`
	Root       string `envconfig:"STORAGE_ROOT" json:"root"`
	MaxRetries int    `default:"10" envconfig:"STORAGE_MAX_RETRIES" json:"max_retries"`
	RetryDelay int    `default:"2000" envconfig:"STORAGE_RETRY_DELAY" json:"retry_delay"`
}

// NewConfig returns a new Config with AWS style options.
func NewConfig(bucket, root string) Config {
	return Config{
		Bucket:     bucket,
		Root:       root,
		MaxRetries: DefaultMaxRetries,
		RetryDelay: DefaultRetryDelay,
	}
}

func (c *Config) SetupRetry(max, delay int) {
	c.MaxRetries = max
	c.RetryDelay = delay
}

func (c Config) String() string {
	root := ""
	if len(c.Root) > 0 {
		root = fmt.Sprintf("Root:%s", c.Root)
	}

	return fmt.Sprintf("{Bucket:%v %s MaxRetries:%v RetryDelay:%v ms}",
		c.Bucket,
		root,
		c.MaxRetries,
		c.RetryDelay)
}

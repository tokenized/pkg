package rpcnode

import (
	"fmt"
)

type Config struct {
	Host     string `envconfig:"RPC_HOST" json:"RPC_HOST"`
	Username string `envconfig:"RPC_USERNAME" json:"RPC_USERNAME"`
	Password string `envconfig:"RPC_PASSWORD" json:"RPC_PASSWORD"`

	// Retry attempts when calls fail.
	MaxRetries int `default:"10" envconfig:"RPC_MAX_RETRIES" json:"RPC_MAX_RETRIES"`
	RetryDelay int `default:"2000" envconfig:"RPC_RETRY_DELAY" json:"RPC_RETRY_DELAY"`

	IgnoreAlreadyInMempool bool `default:"false" envconfig:"RPC_IGNORE_ALREADY_IN_MEMPOOL" json:"RPC_IGNORE_ALREADY_IN_MEMPOOL"`
}

// String returns a custom string representation.
//
// This is important so we don't log sensitive config values.
func (c Config) String() string {
	return fmt.Sprintf("{Host:%v Username:%v Password:%v MaxRetries:%d RetryDelay:%d ms}", c.Host,
		c.Username, "****", c.MaxRetries, c.RetryDelay)
}

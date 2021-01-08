package client

import (
	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

type EnvConfig struct {
	ServerAddress    string `envconfig:"SERVER_ADDRESS" json:"SERVER_ADDRESS"`
	ServerKey        string `envconfig:"SERVER_KEY" json:"SERVER_KEY"`
	ClientKey        string `envconfig:"CLIENT_KEY" json:"CLIENT_KEY" masked:"true"`
	StartBlockHeight uint32 `default:"478559" envconfig:"START_BLOCK_HEIGHT" json:"START_BLOCK_HEIGHT"`
}

type Config struct {
	ServerAddress    string            `envconfig:"SERVER_ADDRESS" json:"SERVER_ADDRESS"`
	ServerKey        bitcoin.PublicKey `envconfig:"SERVER_KEY" json:"SERVER_KEY"`
	ClientKey        bitcoin.Key       `envconfig:"CLIENT_KEY" json:"CLIENT_KEY" masked:"true"`
	StartBlockHeight uint32            `default:"478559" envconfig:"START_BLOCK_HEIGHT" json:"START_BLOCK_HEIGHT"`
}

func (e *EnvConfig) Convert() (*Config, error) {
	cfg := &Config{
		ServerAddress:    e.ServerAddress,
		StartBlockHeight: e.StartBlockHeight,
	}

	if err := cfg.ServerKey.SetString(e.ServerKey); err != nil {
		return nil, errors.Wrap(err, "server key")
	}

	if err := cfg.ClientKey.SetString(e.ClientKey); err != nil {
		return nil, errors.Wrap(err, "key")
	}

	return cfg, nil
}

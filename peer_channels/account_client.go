package peer_channels

import (
	"context"
)

type AccountClient interface {
	// CreatePublicChannel creates a new peer channel that can be written to by anyone.
	CreatePublicChannel(ctx context.Context) (*Channel, error)

	// CreateChannel creates a new peer channel that can only be written to by someone that knows
	// the write token.
	CreateChannel(ctx context.Context) (*Channel, error)

	// Notify receives incoming messages for the peer channel account.
	Notify(ctx context.Context, autosend bool, incoming chan MessageNotification,
		interrupt <-chan interface{}) error

	// Listen receives incoming messages for the peer channel account.
	Listen(ctx context.Context, autosend bool, incoming chan Message,
		interrupt <-chan interface{}) error

	BaseURL() string
}

type StandardAccountClient struct {
	client    Client
	accountID string
	token     string
}

func NewAccountClient(client Client, accountID, token string) *StandardAccountClient {
	return &StandardAccountClient{
		client:    client,
		accountID: accountID,
		token:     token,
	}
}

func (c *StandardAccountClient) CreatePublicChannel(ctx context.Context) (*Channel, error) {
	return c.client.CreatePublicChannel(ctx, c.accountID, c.token)
}

func (c *StandardAccountClient) CreateChannel(ctx context.Context) (*Channel, error) {
	return c.client.CreateChannel(ctx, c.accountID, c.token)
}

func (c *StandardAccountClient) Notify(ctx context.Context, autosend bool,
	incoming chan MessageNotification, interrupt <-chan interface{}) error {
	return c.client.AccountNotify(ctx, c.accountID, c.token, autosend, incoming, interrupt)
}

func (c *StandardAccountClient) Listen(ctx context.Context, autosend bool, incoming chan Message,
	interrupt <-chan interface{}) error {
	return c.client.AccountListen(ctx, c.accountID, c.token, autosend, incoming, interrupt)
}

func (c *StandardAccountClient) BaseURL() string {
	return c.client.BaseURL()
}

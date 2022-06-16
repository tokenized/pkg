package peer_channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
)

type HTTPAccountClient struct {
	baseURL   string
	accountID string
	token     string

	lock sync.Mutex
}

// HTTPCreateAccount creates a new account on an HTTP peer channel service.
// Note: This is a non-standard endpoint and might only be implemented by the Tokenized Service.
func HTTPCreateAccount(ctx context.Context, baseURL, token string) (*Account, error) {
	response := &Account{}
	if err := post(ctx, baseURL+apiURLPart+"account", token, "", nil, response); err != nil {
		return nil, err
	}

	return response, nil
}

func NewHTTPAccountClient(baseURL, accountID, token string) *HTTPAccountClient {
	return &HTTPAccountClient{
		baseURL:   baseURL,
		accountID: accountID,
		token:     token,
	}
}

func (c *HTTPAccountClient) BaseURL() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.baseURL
}

func (c *HTTPAccountClient) AccountID() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.accountID
}

func (c *HTTPAccountClient) Token() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.token
}

// Note: This is a non-standard endpoint and might only be implemented by the Tokenized Service.
func (c *HTTPAccountClient) CreatePublicChannel(ctx context.Context) (*Channel, error) {
	url := fmt.Sprintf("%s%saccount/%s/channel?public", c.BaseURL(), apiURLPart, c.AccountID())

	response := &Channel{}
	if err := post(ctx, url, c.Token(), "", nil, response); err != nil {
		return nil, err
	}

	if len(response.WriteToken) != 0 {
		return response, errors.New("Channel should be public")
	}

	return response, nil
}

// Note: This is a non-standard endpoint and might only be implemented by the Tokenized Service.
func (c *HTTPAccountClient) CreateChannel(ctx context.Context) (*Channel, error) {
	url := fmt.Sprintf("%s%saccount/%s/channel", c.BaseURL(), apiURLPart, c.AccountID())

	response := &Channel{}
	if err := post(ctx, url, c.Token(), "", nil, response); err != nil {
		return nil, err
	}

	if len(response.WriteToken) == 0 {
		return response, errors.New("Channel should not be public")
	}

	return response, nil
}

func (c *HTTPAccountClient) GetChannel(ctx context.Context, channelID string) (*Channel, error) {
	url := fmt.Sprintf("%s%saccount/%s/channel/%s", c.BaseURL(), apiURLPart, c.AccountID(),
		channelID)

	response := &Channel{}
	if err := get(ctx, url, c.Token(), response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *HTTPAccountClient) ListChannels(ctx context.Context) ([]*Channel, error) {
	url := fmt.Sprintf("%s%saccount/%s/channels", c.BaseURL(), apiURLPart, c.AccountID())

	var response []*Channel
	if err := get(ctx, url, c.Token(), &response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *HTTPAccountClient) Notify(ctx context.Context, autosend bool,
	incoming chan MessageNotification, interrupt <-chan interface{}) error {

	client := NewHTTPClient(c.BaseURL())
	return client.Notify(ctx, c.Token(), autosend, incoming, interrupt)
}

func (c *HTTPAccountClient) Listen(ctx context.Context, autosend bool, incoming chan Message,
	interrupt <-chan interface{}) error {

	client := NewHTTPClient(c.BaseURL())
	return client.Listen(ctx, c.Token(), autosend, incoming, interrupt)
}

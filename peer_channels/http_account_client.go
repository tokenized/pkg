package peer_channels

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	url := appendPath(baseURL, apiURLAccountPart)
	if err := post(ctx, url, token, "", nil, response); err != nil {
		return nil, err
	}

	response.BaseURL = baseURL
	return response, nil
}

func NewHTTPAccountClient(account Account) *HTTPAccountClient {
	return &HTTPAccountClient{
		baseURL:   account.BaseURL,
		accountID: account.AccountID,
		token:     account.Token,
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
func (c *HTTPAccountClient) CreatePublicChannel(ctx context.Context) (*FullChannel, error) {
	url := fmt.Sprintf("%s/%s/account/%s/channel?public", c.BaseURL(), apiURLPart, c.AccountID())

	response := &FullChannel{}
	if err := post(ctx, url, c.Token(), "", nil, response); err != nil {
		return nil, err
	}

	if len(response.WriteToken) != 0 {
		return response, errors.New("Channel should be public")
	}

	return response, nil
}

// Note: This is a non-standard endpoint and might only be implemented by the Tokenized Service.
func (c *HTTPAccountClient) CreateChannel(ctx context.Context) (*FullChannel, error) {
	url := fmt.Sprintf("%s/%s/account/%s/channel", c.BaseURL(), apiURLPart, c.AccountID())

	response := &FullChannel{}
	if err := post(ctx, url, c.Token(), "", nil, response); err != nil {
		return nil, err
	}

	if len(response.WriteToken) == 0 {
		return response, errors.New("Channel should not be public")
	}

	return response, nil
}

func (c *HTTPAccountClient) GetChannel(ctx context.Context,
	channelID string) (*FullChannel, error) {
	url := fmt.Sprintf("%s/%s/account/%s/channel/%s", c.BaseURL(), apiURLPart, c.AccountID(),
		channelID)

	response := &FullChannel{}
	if err := get(ctx, url, c.Token(), response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *HTTPAccountClient) ListChannels(ctx context.Context) ([]*FullChannel, error) {
	url := fmt.Sprintf("%s/%s/account/%s/channels", c.BaseURL(), apiURLPart, c.AccountID())

	var response []*FullChannel
	if err := get(ctx, url, c.Token(), &response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *HTTPAccountClient) MarkMessages(ctx context.Context, channelID string, sequence uint64,
	read, older bool) error {

	url := fmt.Sprintf("%s/%s/%s/%d?read=%t&older=%t", c.BaseURL(), apiURLChannelPart, channelID,
		sequence, read, older)

	if err := post(ctx, url, c.Token(), "", nil, nil); err != nil {
		return err
	}

	return nil
}

func (c *HTTPAccountClient) DeleteMessage(ctx context.Context, channelID string, sequence uint64,
	older bool) error {

	url := fmt.Sprintf("%s/%s/%s/%d?older=%t", c.BaseURL(), apiURLChannelPart, channelID, sequence,
		older)

	if err := httpDelete(ctx, url, c.Token()); err != nil {
		return err
	}

	return nil
}

func (c *HTTPAccountClient) Notify(ctx context.Context, autosend bool, channelTimeout time.Duration,
	incoming chan<- MessageNotification, interrupt <-chan interface{}) error {

	client := NewHTTPClient(c.BaseURL())
	return client.Notify(ctx, c.Token(), autosend, channelTimeout, incoming, interrupt)
}

func (c *HTTPAccountClient) Listen(ctx context.Context, autosend bool, channelTimeout time.Duration,
	incoming chan<- Message, interrupt <-chan interface{}) error {

	client := NewHTTPClient(c.BaseURL())
	return client.Listen(ctx, c.Token(), autosend, channelTimeout, incoming, interrupt)
}

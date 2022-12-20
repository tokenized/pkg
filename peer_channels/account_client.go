package peer_channels

import (
	"context"
	"strings"

	"github.com/pkg/errors"
)

type AccountClient interface {
	BaseURL() string
	AccountID() string
	Token() string

	// CreatePublicChannel creates a new peer channel that can be written to by anyone.
	CreatePublicChannel(ctx context.Context) (*AccountChannel, error)

	// CreateChannel creates a new peer channel that can only be written to by someone that knows
	// the write token.
	CreateChannel(ctx context.Context) (*AccountChannel, error)

	GetChannel(ctx context.Context, channelID string) (*AccountChannel, error)
	ListChannels(ctx context.Context) ([]*AccountChannel, error)

	MarkMessages(ctx context.Context, channelID string, sequence uint64,
		read, older bool) error
	DeleteMessage(ctx context.Context, channelID string, sequence uint64, older bool) error

	// Notify receives incoming messages for the peer channel account.
	Notify(ctx context.Context, sendUnread bool, incoming chan<- MessageNotification,
		interrupt <-chan interface{}) error

	// Listen receives incoming messages for the peer channel account.
	Listen(ctx context.Context, sendUnread bool, incoming chan<- Message,
		interrupt <-chan interface{}) error
}

type AccountClientFactory interface {
	NewAccountClient(accountID, token string) (AccountClient, error)
}

// Note: This is a non-standard structure and might only be implemented by the Tokenized Service.
type AccountChannel struct {
	ID         string `bsor:"1" json:"id"`
	AccountID  string `bsor:"2" json:"account_id"`
	ReadToken  string `bsor:"3" json:"read_token"`
	WriteToken string `bsor:"4" json:"write_token"`
}

type AccountChannels []AccountChannel

func (f *Factory) NewAccountClient(account Account) (AccountClient, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if strings.HasPrefix(account.BaseURL, "mock://") {
		if f.mockClient == nil {
			f.mockClient = NewMockClient()
		}

		return f.mockClient.NewAccountClient(account.AccountID, account.Token)
	}

	if strings.HasPrefix(account.BaseURL, "internal://") {
		if f.internalAccountClientFactory == nil {
			return nil, errors.New("No internal account client factory set")
		}

		return f.internalAccountClientFactory.NewAccountClient(account.AccountID, account.Token)
	}

	if !strings.HasPrefix(account.BaseURL, "https://") && !strings.HasPrefix(account.BaseURL, "http://") {
		return nil, errors.New("Unsupported URL protocol")
	}

	return NewHTTPAccountClient(account), nil
}

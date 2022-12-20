package peer_channels

import (
	"context"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

type AccountClient interface {
	BaseURL() string
	AccountID() string
	Token() string

	// CreatePublicChannel creates a new peer channel that can be written to by anyone.
	CreatePublicChannel(ctx context.Context) (*Channel, error)

	// CreateChannel creates a new peer channel that can only be written to by someone that knows
	// the write token.
	CreateChannel(ctx context.Context) (*Channel, error)

	GetChannel(ctx context.Context, channelID string) (*Channel, error)
	ListChannels(ctx context.Context) ([]*Channel, error)

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

type Account struct {
	BaseURL   string `bsor:"1" json:"base_url"`
	AccountID string `bsor:"2" json:"account_id"`
	Token     string `bsor:"3" json:"token"`
}

// Note: This is a non-standard structure and might only be implemented by the Tokenized Service.
type Channel struct {
	ID         string `bsor:"1" json:"id"`
	AccountID  string `bsor:"2" json:"account_id"`
	ReadToken  string `bsor:"3" json:"read_token"`
	WriteToken string `bsor:"4" json:"write_token"`
}

type Channels []Channel

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

func NewAccount(baseURL, accountID, token string) (*Account, error) {
	return &Account{
		BaseURL:   baseURL,
		AccountID: accountID,
		Token:     token,
	}, nil
}

func NewAccountFromString(s string) (*Account, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	query := u.Query()

	accountID := query.Get("account")
	query.Del("account")

	token := query.Get("token")
	query.Del("token")

	u.RawQuery = query.Encode()

	return &Account{
		BaseURL:   u.String(),
		AccountID: accountID,
		Token:     token,
	}, nil
}

func (v Account) MarshalText() ([]byte, error) {
	fullURL, err := url.Parse(v.BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	query := fullURL.Query()
	query.Add("account", url.PathEscape(v.AccountID))
	query.Add("token", url.PathEscape(v.Token))
	fullURL.RawQuery = query.Encode()

	return []byte(fullURL.String()), nil
}

func (v *Account) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *Account) SetString(s string) error {
	fullURL, err := url.Parse(s)
	if err != nil {
		return errors.Wrap(err, "url")
	}

	query := fullURL.Query()

	account := query.Get("account")
	query.Del("account")

	token := query.Get("token")
	query.Del("token")

	fullURL.RawQuery = query.Encode()

	v.BaseURL = fullURL.String()
	v.AccountID = account
	v.Token = token
	return nil
}

func (v Account) String() string {
	b, err := v.MarshalText()
	if err != nil {
		return ""
	}

	return string(b)
}

func (v Account) MarshalBinary() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *Account) UnmarshalBinary(data []byte) error {
	return v.SetString(string(data))
}

// Scan converts from a database column.
func (v *Account) Scan(data interface{}) error {
	s, ok := data.(string)
	if !ok {
		return errors.New("Peer Channel Account value not string")
	}

	if err := v.SetString(s); err != nil {
		return errors.Wrap(err, "set string")
	}

	return nil
}

func (v Account) MarshalJSONMasked() ([]byte, error) {
	fullURL, err := url.Parse(v.BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	query := fullURL.Query()
	query.Add("account", url.PathEscape(v.AccountID))
	fullURL.RawQuery = query.Encode()

	return []byte("\"URL:" + fullURL.String() + "\""), nil
}

package peer_channels

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

const (
	ContentTypeText   = "text/plain"
	ContentTypeJSON   = "application/json"
	ContentTypeBinary = "application/octet-stream"

	InternalBaseURL = "internal://"
)

var (
	ErrWrongContentType = errors.New("Wrong Content Type")
)

type Client interface {
	BaseURL() string

	GetChannelMetaData(ctx context.Context, channelID, token string) (*ChannelData, error)
	WriteMessage(ctx context.Context, channelID, token string, contentType string,
		payload io.Reader) error

	GetMessages(ctx context.Context, channelID, token string, unread bool,
		maxCount uint) (Messages, error)
	GetMaxMessageSequence(ctx context.Context, channelID, token string) (uint64, error)
	MarkMessages(ctx context.Context, channelID, token string, sequence uint64,
		read, older bool) error
	DeleteMessage(ctx context.Context, channelID, token string, sequence uint64, older bool) error

	// Notify writes message notifications to `incoming` as they are posted to the service.
	// `incoming` will not be closed by this function.
	Notify(ctx context.Context, token string, sendUnread bool, incoming chan<- MessageNotification,
		interrupt <-chan interface{}) error

	// Listen writes messages to `incoming` as they are posted to the service.
	// `incoming` will not be closed by this function.
	Listen(ctx context.Context, token string, sendUnread bool, incoming chan<- Message,
		interrupt <-chan interface{}) error
}

type PeerChannel struct {
	URL   string
	Token string
}

type PeerChannels []*PeerChannel

type ChannelData struct {
	// When a write token is provided
	MaxMessagePayloadSize *uint64 `bsor:"1" json:"max_message_payload_size,omitempty"`

	// When a read token is provided.
	AutoDeleteReadMessages *bool `bsor:"2" json:"auto_delete_read_messages,omitempty"`
}

type Message struct {
	Sequence    uint64      `bsor:"1" json:"sequence"`
	Received    time.Time   `bsor:"2" json:"received"`
	ContentType string      `bsor:"3" json:"content_type"`
	ChannelID   string      `bsor:"4" json:"channel_id"`
	Payload     bitcoin.Hex `bsor:"5" json:"payload"`
}

type Messages []*Message

type MessageNotification struct {
	Sequence    uint64    `bsor:"1" json:"sequence"`
	Received    time.Time `bsor:"2" json:"received"`
	ContentType string    `bsor:"3" json:"content_type"`
	ChannelID   string    `bsor:"4" json:"channel_id"`
}

func (m Message) Hash() bitcoin.Hash32 {
	return bitcoin.Hash32(sha256.Sum256(m.Payload))
}

// ParseChannelURL returns the base URL and the channel ID from a peer channels URL.
func ParseChannelURL(url string) (string, string, error) {
	parts := strings.Split(url, apiURLChannelPart)
	if len(parts) != 2 {
		return "", "", errors.New("Missing api channel part")
	}

	if len(parts[0]) == 0 {
		return "", "", errors.New("Missing base URL")
	}

	if len(parts[1]) == 0 {
		return "", "", errors.New("Missing channel id")
	}

	channelParts := strings.Split(parts[1], "/")
	return parts[0], channelParts[0], nil
}

// ChannelURL returns a full peer channels URL for the provided base URL and channel ID.
func ChannelURL(baseURL, channelID string) string {
	return fmt.Sprintf("%s%s%s", baseURL, apiURLChannelPart, channelID)
}

type Factory struct {
	mockClient     *MockClient
	internalClient Client

	internalAccountClientFactory AccountClientFactory

	lock sync.Mutex
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) SetInternalClient(client Client) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.internalClient = client
}

func (f *Factory) SetInternalAccountClientFactory(factory AccountClientFactory) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.internalAccountClientFactory = factory
}

func (f *Factory) MockClient() *MockClient {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.mockClient == nil {
		f.mockClient = NewMockClient()
	}

	return f.mockClient
}

func (f *Factory) NewClient(baseURL string) (Client, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if strings.HasPrefix(baseURL, "mock://") {
		if f.mockClient == nil {
			f.mockClient = NewMockClient()
		}

		return f.mockClient, nil
	}

	if strings.HasPrefix(baseURL, "internal://") {
		if f.internalClient == nil {
			return nil, errors.New("No internal client set")
		}

		return f.internalClient, nil
	}

	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		return nil, errors.New("Unsupported URL protocol")
	}

	return NewHTTPClient(baseURL), nil
}

func NewPeerChannel(channelURL, token string) (*PeerChannel, error) {
	if _, err := url.Parse(channelURL); err != nil {
		return nil, errors.Wrap(err, "url")
	}

	return &PeerChannel{
		URL:   channelURL,
		Token: token,
	}, nil
}

func NewPeerChannelFromString(s string) (*PeerChannel, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	query := u.Query()
	token := query.Get("token")
	query.Del("token")

	u.RawQuery = query.Encode()

	return &PeerChannel{
		URL:   u.String(),
		Token: token,
	}, nil
}

func (v PeerChannel) MarshalText() ([]byte, error) {
	fullURL, err := url.Parse(v.URL)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	query := fullURL.Query()
	query.Add("token", url.PathEscape(v.Token))
	fullURL.RawQuery = query.Encode()

	return []byte(fullURL.String()), nil
}

func (v *PeerChannel) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *PeerChannel) SetString(s string) error {
	fullURL, err := url.Parse(s)
	if err != nil {
		return errors.Wrap(err, "url")
	}

	query := fullURL.Query()
	token := query.Get("token")
	query.Del("token")

	fullURL.RawQuery = query.Encode()

	v.URL = fullURL.String()
	v.Token = token
	return nil
}

func (v PeerChannel) String() string {
	b, err := v.MarshalText()
	if err != nil {
		return ""
	}

	return string(b)
}

func (v PeerChannel) MarshalBinary() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *PeerChannel) UnmarshalBinary(data []byte) error {
	return v.SetString(string(data))
}

// Scan converts from a database column.
func (v *PeerChannel) Scan(data interface{}) error {
	s, ok := data.(string)
	if !ok {
		return errors.New("Peer Channel value not string")
	}

	if err := v.SetString(s); err != nil {
		return errors.Wrap(err, "set string")
	}

	return nil
}

func (v PeerChannel) MarshalJSONMasked() ([]byte, error) {
	return []byte("\"URL:" + v.URL + "\""), nil
}

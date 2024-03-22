package peer_channels

import (
	"context"
	"crypto/sha256"
	"io"
	"mime"
	"strings"
	"sync/atomic"
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

const (
	apiURLChannelPart = "api/v1/channel"
	apiURLAccountPart = "api/v1/account"
	apiURLPart        = "api/v1"
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

func (m Message) BaseContentType() string {
	baseType, _, err := mime.ParseMediaType(m.ContentType)
	if err != nil {
		return ""
	}

	return baseType
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

// ChannelURL returns a full peer channels URL for the provided base URL and channel ID.
func ChannelURL(baseURL, channelID string) string {
	return appendPath(baseURL, appendPath(apiURLChannelPart, channelID))
}

type Factory struct {
	mockClient     atomic.Value // *MockClient
	internalClient atomic.Value // Client

	internalAccountClientFactory atomic.Value // AccountClientFactory
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) SetInternalClient(client Client) {
	f.internalClient.Store(client)
}

func (f *Factory) InternalClient() Client {
	v := f.internalClient.Load()
	if v == nil {
		return nil
	}

	return v.(Client)
}

func (f *Factory) SetInternalAccountClientFactory(factory AccountClientFactory) {
	f.internalAccountClientFactory.Store(factory)
}

func (f *Factory) InternalAccountClientFactory() AccountClientFactory {
	v := f.internalAccountClientFactory.Load()
	if v == nil {
		return nil
	}

	return v.(AccountClientFactory)
}

func (f *Factory) MockClient() *MockClient {
	if v := f.mockClient.Load(); v != nil {
		return v.(*MockClient)
	} else {
		mockClient := f.mockClient.Load()
		if mockClient == nil {
			newMockClient := NewMockClient()
			f.mockClient.Store(newMockClient)
			return newMockClient
		}

		return mockClient.(*MockClient)
	}
}

func (f *Factory) NewClient(baseURL string) (Client, error) {
	if strings.HasPrefix(baseURL, "mock://") {
		return f.MockClient(), nil
	}

	if strings.HasPrefix(baseURL, "internal://") {
		c := f.InternalClient()
		if c == nil {
			return nil, errors.New("No internal client set")
		}

		return c, nil
	}

	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		return nil, errors.New("Unsupported URL protocol")
	}

	return NewHTTPClient(baseURL), nil
}

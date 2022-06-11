package peer_channels

import (
	"context"
	"crypto/sha256"
	"fmt"
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

type Client interface {
	CreateAccount(ctx context.Context, token string) (*string, *string, error)
	CreateChannel(ctx context.Context, accountID, token string) (*Channel, error)
	CreatePublicChannel(ctx context.Context, accountID, token string) (*Channel, error)
	PostTextMessage(ctx context.Context, channelID, token string, message string) (*Message, error)
	PostJSONMessage(ctx context.Context, channelID, token string,
		message interface{}) (*Message, error)
	PostBinaryMessage(ctx context.Context, channelID, token string,
		message []byte) (*Message, error)
	PostBSORMessage(ctx context.Context, channelID, token string,
		message interface{}) (*Message, error)
	GetMessages(ctx context.Context, channelID, token string, unread bool) (Messages, error)
	GetMaxMessageSequence(ctx context.Context, channelID, token string) (uint32, error)
	MarkMessages(ctx context.Context, channelID, token string, sequence uint32,
		read, older bool) error

	AccountNotify(ctx context.Context, accountID, token string, autosend bool,
		incoming chan<- MessageNotification, interrupt <-chan interface{}) error
	AccountListen(ctx context.Context, accountID, token string, autosend bool,
		incoming chan<- Message, interrupt <-chan interface{}) error
	ChannelNotify(ctx context.Context, channelID, token string, autosend bool,
		incoming chan<- MessageNotification, interrupt <-chan interface{}) error
	ChannelListen(ctx context.Context, channelID, token string, autosend bool,
		incoming chan<- Message, interrupt <-chan interface{}) error

	BaseURL() string
}

func ParseChannelURL(url string) (string, string, error) {
	parts := strings.Split(url, apiURLPart)
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

func ChannelURL(baseURL, channelID string) string {
	return fmt.Sprintf("%s%s%s", baseURL, apiURLPart, channelID)
}

type Message struct {
	Sequence    uint32      `bsor:"1" json:"sequence"`
	Received    time.Time   `bsor:"2" json:"received"`
	ContentType string      `bsor:"3" json:"content_type"`
	Payload     bitcoin.Hex `bsor:"4" json:"payload"`
	ChannelID   string      `bsor:"5" json:"channel_id"`
}

type Messages []*Message

type MessageNotification struct {
	Sequence  uint32    `bsor:"1" json:"sequence"`
	Received  time.Time `bsor:"2" json:"received"`
	ChannelID string    `bsor:"5" json:"channel_id"`
}

func (m Message) Hash() bitcoin.Hash32 {
	return bitcoin.Hash32(sha256.Sum256(m.Payload))
}

type Factory struct {
	mockClient     *MockClient
	internalClient Client

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

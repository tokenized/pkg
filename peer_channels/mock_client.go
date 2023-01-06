package peer_channels

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/tokenized/logger"
	"github.com/tokenized/threads"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	MockClientURL = "mock://mock_peer_channels"
)

type MockClient struct {
	baseURL  string
	accounts sync.Map
	channels sync.Map

	notifiers []*mockNotifier
	listeners []*mockListener

	lock sync.Mutex
}

type MockAccountClient struct {
	client *MockClient

	accountID string
	token     string

	lock sync.Mutex
}

type mockNotifier struct {
	token    string
	incoming chan<- MessageNotification
}

type mockListener struct {
	token    string
	incoming chan<- Message
}

type mockAccount struct {
	id    string
	token string

	lock sync.Mutex
}

type mockChannel struct {
	id             string
	accountID      string
	readToken      string
	writeToken     string
	nextSequence   uint64
	unreadSequence uint64

	messages Messages

	lock sync.Mutex
}

func NewMockClient() *MockClient {
	return &MockClient{
		baseURL: MockClientURL,
	}
}

func NewMockAccountClient(client *MockClient, accountID, token string) *MockAccountClient {
	return &MockAccountClient{
		client:    client,
		accountID: accountID,
		token:     token,
	}
}

func (c *MockClient) NewAccountClient(accountID, token string) (AccountClient, error) {
	return NewMockAccountClient(c, accountID, token), nil
}

func (c *MockClient) CreateAccount(ctx context.Context) (*string, *string, error) {
	account := &mockAccount{
		id:    uuid.New().String(),
		token: uuid.New().String(),
	}
	account.lock.Lock()
	defer account.lock.Unlock()

	c.accounts.Store(account.id, account)

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", account.id),
	}, "Created peer channel account")

	return &account.id, &account.token, nil
}

func (c *MockClient) BaseURL() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.baseURL
}

func (c *MockClient) GetChannelMetaData(ctx context.Context,
	channelID, token string) (*ChannelData, error) {

	ch, exists := c.channels.Load(channelID)
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()
	defer channel.lock.Unlock()

	if channel.readToken == token {
		autoDeleteReadMessages := true
		return &ChannelData{
			AutoDeleteReadMessages: &autoDeleteReadMessages,
		}, nil
	}

	if channel.writeToken == token {
		maxMessagePayloadSize := uint64(1e9)
		return &ChannelData{
			MaxMessagePayloadSize: &maxMessagePayloadSize,
		}, nil
	}

	return nil, HTTPError{Status: http.StatusUnauthorized}
}

func (c *MockClient) WriteMessage(ctx context.Context, channelID, token string, contentType string,
	payload io.Reader) error {

	b, err := ioutil.ReadAll(payload)
	if err != nil {
		return errors.Wrap(err, "read payload")
	}

	return c.addMessage(ctx, channelID, token, contentType, b)
}

func (c *MockClient) GetMessages(ctx context.Context, channelID, token string, unread bool,
	maxCount uint) (Messages, error) {

	ch, exists := c.channels.Load(channelID)
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.readToken != token {
		return nil, HTTPError{Status: http.StatusUnauthorized}
	}

	if len(channel.messages) == 0 {
		return nil, nil
	}

	if int(channel.unreadSequence) >= len(channel.messages) {
		return nil, nil
	}

	var result Messages
	for _, message := range channel.messages[channel.unreadSequence:] {
		msg := *message // copy
		result = append(result, &msg)
		channel.unreadSequence++
	}

	return result, nil
}

func (c *MockClient) GetMaxMessageSequence(ctx context.Context,
	channelID, token string) (uint64, error) {

	ch, exists := c.channels.Load(channelID)
	if !exists {
		return 0, HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.readToken != token {
		return 0, HTTPError{Status: http.StatusUnauthorized}
	}

	if len(channel.messages) == 0 {
		return 0, nil
	}

	return channel.messages[len(channel.messages)-1].Sequence, nil
}

func (c *MockClient) MarkMessages(ctx context.Context, channelID, token string, sequence uint64,
	read, older bool) error {

	ch, exists := c.channels.Load(channelID)
	if !exists {
		return HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()
	defer channel.lock.Unlock()

	if channel.readToken != token {
		ac, exists := c.accounts.Load(channel.accountID)
		if !exists {
			return HTTPError{Status: http.StatusUnauthorized}
		}
		account := ac.(*mockAccount)

		account.lock.Lock()
		if account.token != token {
			account.lock.Unlock()
			return HTTPError{Status: http.StatusUnauthorized}
		}
		account.lock.Unlock()
	}

	if !read || !older {
		return errors.New("Only read=true and older=true is supported")
	}

	if sequence >= channel.nextSequence {
		channel.unreadSequence = channel.nextSequence - 1
	} else {
		channel.unreadSequence = sequence + 1
	}

	return nil
}

func (c *MockClient) DeleteMessage(ctx context.Context, channelID, token string, sequence uint64,
	older bool) error {

	ch, exists := c.channels.Load(channelID)
	if !exists {
		return HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()
	defer channel.lock.Unlock()

	if channel.readToken != token {
		ac, exists := c.accounts.Load(channel.accountID)
		if !exists {
			return HTTPError{Status: http.StatusUnauthorized}
		}
		account := ac.(*mockAccount)

		account.lock.Lock()
		if account.token != token {
			account.lock.Unlock()
			return HTTPError{Status: http.StatusUnauthorized}
		}
		account.lock.Unlock()
	}

	if !older {
		return errors.New("Only older=true is supported")
	}

	if len(channel.messages) == 0 {
		return HTTPError{Status: http.StatusNotFound, Message: "No messages"}
	}

	if channel.messages[0].Sequence > sequence {
		return HTTPError{Status: http.StatusNotFound, Message: "First message after sequence"}
	}

	if channel.messages[len(channel.messages)-1].Sequence < sequence {
		return HTTPError{Status: http.StatusNotFound, Message: "Last message before sequence"}
	}

	countToDelete := sequence - channel.messages[0].Sequence
	channel.messages = channel.messages[countToDelete:]
	return nil
}

func (c *MockClient) Notify(ctx context.Context, token string, sendUnread bool,
	incoming chan<- MessageNotification, interrupt <-chan interface{}) error {

	var notifier *mockNotifier
	var newMessages Messages

	c.accounts.Range(func(key, value interface{}) bool {
		account := value.(*mockAccount)
		if account.Token() != token {
			return true
		}

		logger.InfoWithFields(ctx, []logger.Field{
			logger.String("account", account.ID()),
		}, "Listening to account")

		notifier = &mockNotifier{
			token:    token,
			incoming: incoming,
		}

		if sendUnread {
			newMessages = c.getUnreadMessagesForAccount(account.ID())
		}

		return false
	})

	if notifier == nil {
		c.channels.Range(func(key, value interface{}) bool {
			channel := value.(*mockChannel)
			if channel.ReadToken() != token {
				return true
			}

			logger.InfoWithFields(ctx, []logger.Field{
				logger.String("channel", channel.id),
			}, "Listening to channel")

			notifier = &mockNotifier{
				token:    token,
				incoming: incoming,
			}

			if sendUnread {
				newMessages = channel.getUnreadMessages()
			}

			return false
		})
	}

	if notifier != nil {
		c.lock.Lock()
		c.notifiers = append(c.notifiers, notifier)
		c.lock.Unlock()
	}

	if notifier == nil {
		logger.ErrorWithFields(ctx, []logger.Field{
			logger.String("token", token),
		}, "No accounts or channels found for token")
		return HTTPError{Status: http.StatusNotFound}
	}

	for _, message := range newMessages {
		notifier.incoming <- MessageNotification{
			Sequence:    message.Sequence,
			Received:    message.Received,
			ContentType: message.ContentType,
			ChannelID:   message.ChannelID,
		}
	}

	select {
	case <-interrupt:
		// remove notifier and close channel
		c.lock.Lock()
		for i, item := range c.notifiers {
			if item == notifier {
				c.notifiers = append(c.notifiers[:i], c.notifiers[i+1:]...)
				break
			}
		}
		c.lock.Unlock()
	}

	return nil
}

func (c *MockClient) Listen(ctx context.Context, token string, sendUnread bool,
	incoming chan<- Message, interrupt <-chan interface{}) error {

	var listener *mockListener
	var newMessages Messages

	c.accounts.Range(func(key, value interface{}) bool {
		account := value.(*mockAccount)
		if account.Token() != token {
			return true
		}

		logger.InfoWithFields(ctx, []logger.Field{
			logger.String("account", account.ID()),
		}, "Listening to account")

		listener = &mockListener{
			token:    token,
			incoming: incoming,
		}

		if sendUnread {
			newMessages = c.getUnreadMessagesForAccount(account.ID())
		}

		return false
	})

	if listener == nil {
		c.channels.Range(func(key, value interface{}) bool {
			channel := value.(*mockChannel)
			if channel.ReadToken() != token {
				return true
			}

			logger.InfoWithFields(ctx, []logger.Field{
				logger.String("channel", channel.id),
			}, "Listening to channel")

			listener = &mockListener{
				token:    token,
				incoming: incoming,
			}

			if sendUnread {
				newMessages = channel.getUnreadMessages()
			}

			return false
		})
	}

	if listener != nil {
		c.lock.Lock()
		c.listeners = append(c.listeners, listener)
		c.lock.Unlock()
	}

	if listener == nil {
		logger.ErrorWithFields(ctx, []logger.Field{
			logger.String("token", token),
		}, "No accounts or channels found for token")
		return HTTPError{Status: http.StatusNotFound}
	}

	for _, message := range newMessages {
		listener.incoming <- *message
	}

	select {
	case <-interrupt:
		// remove listener and close channel
		c.lock.Lock()
		for i, item := range c.listeners {
			if item == listener {
				c.listeners = append(c.listeners[:i], c.listeners[i+1:]...)
				break
			}
		}
		c.lock.Unlock()

		return threads.Interrupted
	}
}

func (c *MockAccountClient) BaseURL() string {
	return c.client.BaseURL()
}

func (c *MockAccountClient) AccountID() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.accountID
}

func (c *MockAccountClient) Token() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.token
}

// CreatePublicChannel creates a new peer channel that can be written to by anyone.
func (c *MockAccountClient) CreatePublicChannel(ctx context.Context) (*AccountChannel, error) {
	ac, exists := c.client.accounts.Load(c.AccountID())
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	account := ac.(*mockAccount)

	account.lock.Lock()
	defer account.lock.Unlock()

	if account.token != c.Token() {
		return nil, HTTPError{Status: http.StatusUnauthorized}
	}

	channel := &mockChannel{
		id:         uuid.New().String(),
		accountID:  c.AccountID(),
		readToken:  uuid.New().String(),
		writeToken: "", // no write token means anyone can write
	}
	channel.lock.Lock()
	defer channel.lock.Unlock()

	c.client.channels.Store(channel.id, channel)

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", account.id),
		logger.String("channel_id", channel.id),
	}, "Created public peer channel")

	return &AccountChannel{
		ID:         channel.id,
		AccountID:  channel.accountID,
		ReadToken:  channel.readToken,
		WriteToken: channel.writeToken,
	}, nil
}

// CreateChannel creates a new peer channel that can only be written to by someone that knows
// the write token.
func (c *MockAccountClient) CreateChannel(ctx context.Context) (*AccountChannel, error) {
	ac, exists := c.client.accounts.Load(c.AccountID())
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	account := ac.(*mockAccount)

	account.lock.Lock()
	defer account.lock.Unlock()

	if account.token != c.Token() {
		return nil, HTTPError{Status: http.StatusUnauthorized}
	}

	channel := &mockChannel{
		id:         uuid.New().String(),
		accountID:  c.AccountID(),
		readToken:  uuid.New().String(),
		writeToken: uuid.New().String(),
	}
	channel.lock.Lock()
	defer channel.lock.Unlock()

	c.client.channels.Store(channel.id, channel)

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", account.id),
		logger.String("channel_id", channel.id),
	}, "Created peer channel")

	return &AccountChannel{
		ID:         channel.id,
		AccountID:  channel.accountID,
		ReadToken:  channel.readToken,
		WriteToken: channel.writeToken,
	}, nil
}

func (c *MockAccountClient) GetChannel(ctx context.Context, channelID string) (*AccountChannel, error) {
	ac, exists := c.client.accounts.Load(c.AccountID())
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	account := ac.(*mockAccount)

	account.lock.Lock()
	if account.token != c.Token() {
		account.lock.Unlock()
		return nil, HTTPError{Status: http.StatusUnauthorized}
	}
	account.lock.Unlock()

	ch, exists := c.client.channels.Load(channelID)
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()
	defer channel.lock.Unlock()

	if c.AccountID() != channel.accountID {
		return nil, HTTPError{Status: http.StatusUnauthorized}
	}

	return &AccountChannel{
		ID:         channel.id,
		AccountID:  channel.accountID,
		ReadToken:  channel.readToken,
		WriteToken: channel.writeToken,
	}, nil
}

func (c *MockAccountClient) ListChannels(ctx context.Context) ([]*AccountChannel, error) {
	accountID := c.AccountID()
	ac, exists := c.client.accounts.Load(accountID)
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}
	account := ac.(*mockAccount)

	account.lock.Lock()
	if account.token != c.Token() {
		account.lock.Unlock()
		return nil, HTTPError{Status: http.StatusUnauthorized}
	}
	account.lock.Unlock()

	var result []*AccountChannel
	c.client.channels.Range(func(key, value interface{}) bool {
		channel := value.(*mockChannel)
		channel.lock.Lock()
		defer channel.lock.Unlock()

		if channel.accountID != accountID {
			return true
		}

		result = append(result, &AccountChannel{
			ID:         channel.id,
			AccountID:  channel.accountID,
			ReadToken:  channel.readToken,
			WriteToken: channel.writeToken,
		})

		return true
	})

	return result, nil
}

func (c *MockAccountClient) MarkMessages(ctx context.Context, channelID string, sequence uint64,
	read, older bool) error {

	return c.client.MarkMessages(ctx, channelID, c.Token(), sequence, read, older)
}

func (c *MockAccountClient) DeleteMessage(ctx context.Context, channelID string, sequence uint64,
	older bool) error {

	return c.client.DeleteMessage(ctx, channelID, c.Token(), sequence, older)
}

// Notify receives incoming messages for the peer channel account.
func (c *MockAccountClient) Notify(ctx context.Context, sendUnread bool,
	incoming chan<- MessageNotification, interrupt <-chan interface{}) error {

	return c.client.Notify(ctx, c.Token(), sendUnread, incoming, interrupt)
}

// Listen receives incoming messages for the peer channel account.
func (c *MockAccountClient) Listen(ctx context.Context, sendUnread bool, incoming chan<- Message,
	interrupt <-chan interface{}) error {

	return c.client.Listen(ctx, c.Token(), sendUnread, incoming, interrupt)
}

func (a *mockAccount) ID() string {
	a.lock.Lock()
	defer a.lock.Unlock()

	return a.id
}

func (a *mockAccount) Token() string {
	a.lock.Lock()
	defer a.lock.Unlock()

	return a.token
}

func (c *mockChannel) ID() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.id
}

func (c *mockChannel) ReadToken() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.readToken
}

func (c *MockClient) addMessage(ctx context.Context, channelID, token string, contentType string,
	payload []byte) error {

	ch, exists := c.channels.Load(channelID)
	if !exists {
		return HTTPError{Status: http.StatusNotFound}
	}
	channel := ch.(*mockChannel)

	channel.lock.Lock()

	ac, accountExists := c.accounts.Load(channel.accountID)
	if !accountExists {
		channel.lock.Unlock()
		return HTTPError{Status: http.StatusNotFound}
	}
	account := ac.(*mockAccount)

	accountToken := account.Token()

	if channel.writeToken != token {
		channel.lock.Unlock()
		return HTTPError{Status: http.StatusUnauthorized}
	}

	message := &Message{
		Received:    time.Now(),
		ContentType: contentType,
		Payload:     payload,
		ChannelID:   channelID,
	}

	accountID := channel.accountID
	readToken := channel.readToken
	message.Sequence = channel.nextSequence
	channel.nextSequence++
	channel.messages = append(channel.messages, message)
	channel.lock.Unlock()

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", accountID),
		logger.String("channel_id", channelID),
		logger.String("content_type", contentType),
		logger.Uint64("sequence", message.Sequence),
		logger.Int("bytes", len(payload)),
	}, "Added peer channel message")

	var notifiers []*mockNotifier
	var listeners []*mockListener
	c.lock.Lock()
	for _, notifier := range c.notifiers {
		if notifier.token != readToken && notifier.token != accountToken {
			continue
		}

		logger.Info(ctx, "Found notifier for message")
		notifiers = append(notifiers, notifier)
	}

	for _, listener := range c.listeners {
		if listener.token != readToken && listener.token != accountToken {
			continue
		}

		logger.Info(ctx, "Found listener for message")
		listeners = append(listeners, listener)
	}
	c.lock.Unlock()

	for _, notifier := range notifiers {
		notifier.incoming <- MessageNotification{
			Sequence:    message.Sequence,
			Received:    message.Received,
			ContentType: message.ContentType,
			ChannelID:   message.ChannelID,
		}
	}

	for _, listener := range listeners {
		listener.incoming <- *message
	}

	return nil
}

func (c *MockClient) getUnreadMessagesForAccount(accountID string) Messages {
	var result Messages
	c.channels.Range(func(key, value interface{}) bool {
		channel := value.(*mockChannel)
		channel.lock.Lock()
		defer channel.lock.Unlock()

		if channel.accountID != accountID {
			return true
		}

		if int(channel.unreadSequence) < len(channel.messages) {
			for _, message := range channel.messages[channel.unreadSequence:] {
				msg := *message // copy
				result = append(result, &msg)
				channel.unreadSequence++
			}
		}

		return true
	})

	return result
}

func (channel *mockChannel) getUnreadMessages() Messages {
	channel.lock.Lock()
	defer channel.lock.Unlock()

	var result Messages
	if int(channel.unreadSequence) < len(channel.messages) {
		for _, message := range channel.messages[channel.unreadSequence:] {
			msg := *message // copy
			result = append(result, &msg)
			channel.unreadSequence++
		}
	}

	return result
}

package peer_channels

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tokenized/pkg/bsor"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/logger"
	"github.com/tokenized/pkg/threads"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	MockClientURL = "mock://mock_peer_channels"
)

type MockClient struct {
	baseURL  string
	accounts map[string]*mockAccount
	channels map[string]*mockChannel

	lock sync.Mutex
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
	nextSequence   uint32
	unreadSequence uint32

	messages Messages

	lock sync.Mutex
}

func NewMockClient() *MockClient {
	return &MockClient{
		baseURL:  MockClientURL,
		accounts: make(map[string]*mockAccount),
		channels: make(map[string]*mockChannel),
	}
}

func (c *MockClient) CreateAccount(ctx context.Context, token string) (*string, *string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	account := &mockAccount{
		id:    uuid.New().String(),
		token: uuid.New().String(),
	}
	c.accounts[account.id] = account

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", account.id),
	}, "Created peer channel account")

	return &account.id, &account.token, nil
}

func (c *MockClient) CreateChannel(ctx context.Context, accountID, token string) (*Channel, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	account, exists := c.accounts[accountID]
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}

	account.lock.Lock()
	defer account.lock.Unlock()
	if account.token != token {
		return nil, HTTPError{Status: http.StatusForbidden}
	}

	channel := &mockChannel{
		id:         uuid.New().String(),
		accountID:  accountID,
		readToken:  uuid.New().String(),
		writeToken: uuid.New().String(),
	}

	c.channels[channel.id] = channel

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", account.id),
		logger.String("channel_id", channel.id),
	}, "Created peer channel")

	return c.convertMockChannel(channel), nil
}

func (c *MockClient) CreatePublicChannel(ctx context.Context,
	accountID, token string) (*Channel, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	account, exists := c.accounts[accountID]
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}

	account.lock.Lock()
	defer account.lock.Unlock()
	if account.token != token {
		return nil, HTTPError{Status: http.StatusForbidden}
	}

	channel := &mockChannel{
		id:         uuid.New().String(),
		accountID:  accountID,
		readToken:  uuid.New().String(),
		writeToken: "", // no write token means anyone can write
	}

	c.channels[channel.id] = channel

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", account.id),
		logger.String("channel_id", channel.id),
	}, "Created public peer channel")

	return c.convertMockChannel(channel), nil
}

func (c *MockClient) convertMockChannel(channel *mockChannel) *Channel {
	result := &Channel{
		ID:          channel.id,
		Path:        fmt.Sprintf("%s/api/v1/channel/%s/notify", c.baseURL, channel.id),
		PublicRead:  false,
		PublicWrite: false,
		Sequenced:   true,
		Locked:      false,
		Head:        0,
		Retention: Retention{
			MinAgeDays: 0,
			MaxAgeDays: 0,
			AutoPrune:  true,
		},
	}

	if len(channel.readToken) > 0 {
		result.AccessTokens = append(result.AccessTokens, AccessToken{
			ID:       channel.readToken,
			Token:    channel.readToken,
			CanRead:  true,
			CanWrite: false,
		})
	}

	if len(channel.writeToken) > 0 {
		result.AccessTokens = append(result.AccessTokens, AccessToken{
			ID:       channel.writeToken,
			Token:    channel.writeToken,
			CanRead:  false,
			CanWrite: true,
		})
	} else {
		result.PublicWrite = true
	}

	return result
}

func (c *MockClient) addMessage(ctx context.Context, channelID, token string, contentType string,
	payload []byte) (*Message, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	channel, exists := c.channels[channelID]
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}

	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.writeToken != token {
		return nil, HTTPError{Status: http.StatusForbidden}
	}

	message := &Message{
		Received:    time.Now(),
		ContentType: contentType,
		Payload:     payload,
		ChannelID:   channelID,
	}

	message.Sequence = channel.nextSequence
	channel.nextSequence++
	channel.messages = append(channel.messages, message)

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account_id", channel.accountID),
		logger.String("channel_id", channel.id),
		logger.String("content_type", contentType),
		logger.Uint32("sequence", message.Sequence),
		logger.Int("bytes", len(payload)),
	}, "Added peer channel message")

	return message, nil
}

func (c *MockClient) PostTextMessage(ctx context.Context, channelID, token string,
	message string) (*Message, error) {

	return c.addMessage(ctx, channelID, token, ContentTypeText, []byte(message))
}

func (c *MockClient) PostJSONMessage(ctx context.Context, channelID, token string,
	message interface{}) (*Message, error) {

	js, err := json.Marshal(message)
	if err != nil {
		return nil, errors.Wrap(err, "json")
	}

	return c.addMessage(ctx, channelID, token, ContentTypeJSON, js)
}

func (c *MockClient) PostBinaryMessage(ctx context.Context, channelID, token string,
	message []byte) (*Message, error) {

	return c.addMessage(ctx, channelID, token, ContentTypeBinary, message)
}

func (c *MockClient) PostBSORMessage(ctx context.Context, channelID, token string,
	message interface{}) (*Message, error) {

	scriptItems, err := bsor.Marshal(message)
	if err != nil {
		return nil, errors.Wrap(err, "bsor")
	}

	script, err := scriptItems.Script()
	if err != nil {
		return nil, errors.Wrap(err, "script")
	}

	return c.addMessage(ctx, channelID, token, ContentTypeBinary, script)
}

func (c *MockClient) GetMessages(ctx context.Context, channelID, token string,
	unread bool) (Messages, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	channel, exists := c.channels[channelID]
	if !exists {
		return nil, HTTPError{Status: http.StatusNotFound}
	}

	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.readToken != token {
		return nil, HTTPError{Status: http.StatusForbidden}
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
	channelID, token string) (uint32, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	channel, exists := c.channels[channelID]
	if !exists {
		return 0, HTTPError{Status: http.StatusNotFound}
	}

	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.readToken != token {
		return 0, HTTPError{Status: http.StatusForbidden}
	}

	if len(channel.messages) == 0 {
		return 0, nil
	}

	return channel.messages[len(channel.messages)-1].Sequence, nil
}

func (c *MockClient) MarkMessages(ctx context.Context, channelID, token string, sequence uint32,
	read, older bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	channel, exists := c.channels[channelID]
	if !exists {
		return HTTPError{Status: http.StatusNotFound}
	}

	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.readToken != token {
		return HTTPError{Status: http.StatusForbidden}
	}

	if !read || !older {
		return errors.New("Only read=true and older=true is accepted")
	}

	if sequence >= channel.nextSequence {
		channel.unreadSequence = channel.nextSequence - 1
	} else {
		channel.unreadSequence = sequence + 1
	}

	return nil
}

func (c *MockClient) getUnreadMessagesForAccount(accountID string) Messages {
	c.lock.Lock()
	defer c.lock.Unlock()

	var result Messages
	for _, channel := range c.channels {
		if channel.accountID != accountID {
			continue
		}

		channel.lock.Lock()
		if int(channel.unreadSequence) < len(channel.messages) {
			for _, message := range channel.messages[channel.unreadSequence:] {
				msg := *message // copy
				result = append(result, &msg)
				channel.unreadSequence++
			}
		}
		channel.lock.Unlock()
	}

	return result
}

func (c *MockClient) AccountListen(ctx context.Context, accountID, token string,
	incoming chan Message, interrupt <-chan interface{}) error {

	c.lock.Lock()
	account, exists := c.accounts[accountID]
	if !exists {
		c.lock.Unlock()
		return HTTPError{Status: http.StatusNotFound}
	}

	account.lock.Lock()
	if account.token != token {
		account.lock.Unlock()
		c.lock.Unlock()
		return HTTPError{Status: http.StatusForbidden}
	}
	account.lock.Unlock()
	c.lock.Unlock()

	stop := threads.NewAtomicFlag()
	done := make(chan interface{})
	wait := &sync.WaitGroup{}

	wait.Add(1)
	go func() {
		defer close(done)
		for !stop.IsSet() {
			account.lock.Lock()
			newMessages := c.getUnreadMessagesForAccount(accountID)
			account.lock.Unlock()

			for _, message := range newMessages {
				incoming <- *message
			}

			time.Sleep(time.Millisecond * 100)
		}
		wait.Done()
	}()

	wait.Add(1)
	go func() {
		select {
		case <-interrupt:
			stop.Set()
		case <-done:
		}
		wait.Done()
	}()

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("account", accountID),
	}, "Listening to account")

	wait.Wait()
	return nil
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

func (c *MockClient) ChannelListen(ctx context.Context, channelID, token string,
	incoming chan Message, interrupt <-chan interface{}) error {

	c.lock.Lock()
	channel, exists := c.channels[channelID]
	if !exists {
		c.lock.Unlock()
		return HTTPError{Status: http.StatusNotFound}
	}

	channel.lock.Lock()
	if channel.readToken != token {
		channel.lock.Unlock()
		c.lock.Unlock()
		return HTTPError{Status: http.StatusForbidden}
	}
	channel.lock.Unlock()
	c.lock.Unlock()

	stop := threads.NewAtomicFlag()
	done := make(chan interface{})
	wait := &sync.WaitGroup{}

	wait.Add(1)
	go func() {
		defer close(done)
		for {
			newMessages := channel.getUnreadMessages()

			for _, message := range newMessages {
				incoming <- *message
			}

			time.Sleep(time.Millisecond * 100)
		}
		wait.Done()
	}()

	wait.Add(1)
	go func() {
		select {
		case <-interrupt:
			stop.Set()
		case <-done:
		}
		wait.Done()
	}()

	wait.Wait()
	return nil
}

package peer_channels

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/bsor"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/logger"
	"github.com/tokenized/pkg/threads"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	apiURLChannelPart = "/api/v1/channel/"
	apiURLPart        = "/api/v1/"
)

type HTTPError struct {
	Status  int
	Message string
}

func (err HTTPError) Error() string {
	if len(err.Message) > 0 {
		return fmt.Sprintf("HTTP Status %d : %s", err.Status, err.Message)
	}

	return fmt.Sprintf("HTTP Status %d", err.Status)
}

type HTTPClient struct {
	baseURL string

	lock sync.Mutex
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
	}
}

func (c *HTTPClient) BaseURL() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.baseURL
}

func (c *HTTPClient) GetChannelMetaData(ctx context.Context,
	channelID, token string) (*ChannelData, error) {

	url := c.BaseURL() + apiURLChannelPart + channelID + "/metadata"

	response := &ChannelData{}
	if err := get(ctx, url, token, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *HTTPClient) WriteMessage(ctx context.Context, channelID, token string, contentType string,
	payload io.Reader) error {

	url := c.BaseURL() + apiURLChannelPart + channelID

	if err := post(ctx, url, token, contentType, payload, nil); err != nil {
		return err
	}

	return nil
}

func (c *HTTPClient) GetMessages(ctx context.Context, channelID, token string, unread bool,
	maxCount uint) (Messages, error) {

	url := c.BaseURL() + apiURLChannelPart + channelID +
		fmt.Sprintf("?unread=%t&count=%d", unread, maxCount)

	var response Messages
	if err := get(ctx, url, token, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *HTTPClient) GetMaxMessageSequence(ctx context.Context,
	channelID, token string) (uint64, error) {

	url := c.BaseURL() + apiURLChannelPart + channelID

	headers, err := head(ctx, url, token)
	if err != nil {
		return 0, err
	}

	tag := headers.Get("ETag")
	if len(tag) == 0 {
		return 0, errors.New("Missing tag")
	}

	maxSequence, err := strconv.ParseUint(tag, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "parse tag")
	}

	return maxSequence, nil
}

func (c *HTTPClient) MarkMessages(ctx context.Context, channelID, token string, sequence uint64,
	read, older bool) error {

	url := c.BaseURL() + apiURLChannelPart + channelID +
		fmt.Sprintf("/%d?read=%t&older=%t", sequence, read, older)

	if err := post(ctx, url, token, "", nil, nil); err != nil {
		return err
	}

	return nil
}

func (c *HTTPClient) DeleteMessage(ctx context.Context, channelID, token string, sequence uint64,
	older bool) error {

	url := c.BaseURL() + apiURLChannelPart + channelID + fmt.Sprintf("/%d?older=%t", sequence, older)

	if err := httpDelete(ctx, url, token); err != nil {
		return err
	}

	return nil
}

func (c *HTTPClient) Notify(ctx context.Context, token string, sendUnread bool,
	incoming chan<- MessageNotification, interrupt <-chan interface{}) error {

	translator := newNotificationTranslator(incoming)
	defer close(incoming)

	params := url.Values{}
	params.Add("token", token)
	params.Add("sendunread", fmt.Sprintf("%t", sendUnread))
	params.Add("fullmessages", "true")

	token = url.PathEscape(token)
	url := c.BaseURL() + apiURLPart + fmt.Sprintf("/notify?%s", params.Encode())
	url = strings.ReplaceAll(url, "https://", "wss://")
	url = strings.ReplaceAll(url, "http://", "ws://")

	return websocketListen(ctx, url, translator, interrupt)
}

func (c *HTTPClient) Listen(ctx context.Context, token string, sendUnread bool,
	incoming chan<- Message, interrupt <-chan interface{}) error {

	translator := newMessageTranslator(incoming)
	defer close(incoming)

	params := url.Values{}
	params.Add("token", token)
	params.Add("sendunread", fmt.Sprintf("%t", sendUnread))
	params.Add("fullmessages", "true")

	token = url.PathEscape(token)
	url := c.BaseURL() + apiURLPart + fmt.Sprintf("notify?%s", params.Encode())
	url = strings.ReplaceAll(url, "https://", "wss://")
	url = strings.ReplaceAll(url, "http://", "ws://")

	return websocketListen(ctx, url, translator, interrupt)
}

type Translator interface {
	Translate(ctx context.Context, msg websocketMessage) error
}

type messageTranslator struct {
	incoming chan<- Message
}

func newMessageTranslator(incoming chan<- Message) *messageTranslator {
	return &messageTranslator{
		incoming: incoming,
	}
}

func (t *messageTranslator) Translate(ctx context.Context, msg websocketMessage) error {
	switch msg.Type {
	case websocket.TextMessage:
		logger.InfoWithFields(ctx, []logger.Field{
			logger.Int("bytes", len(msg.Bytes)),
		}, "Received text message")

		var message Message
		if err := json.Unmarshal(msg.Bytes, &message); err != nil {
			return errors.Wrap(err, "json unmarshal")
		}

		t.incoming <- message

	case websocket.BinaryMessage:
		logger.InfoWithFields(ctx, []logger.Field{
			logger.Int("bytes", len(msg.Bytes)),
		}, "Received binary message")

		scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(msg.Bytes), -1)
		if err != nil {
			return errors.Wrap(err, "parse script")
		}

		var message Message
		if _, err := bsor.Unmarshal(scriptItems, &message); err != nil {
			return errors.Wrap(err, "bsor unmarshal")
		}

		t.incoming <- message
	}

	return nil
}

type notificationTranslator struct {
	incoming chan<- MessageNotification
}

func newNotificationTranslator(incoming chan<- MessageNotification) *notificationTranslator {
	return &notificationTranslator{
		incoming: incoming,
	}
}

func (t *notificationTranslator) Translate(ctx context.Context, msg websocketMessage) error {
	switch msg.Type {
	case websocket.TextMessage:
		logger.InfoWithFields(ctx, []logger.Field{
			logger.Int("bytes", len(msg.Bytes)),
		}, "Received text message")

		var message MessageNotification
		if err := json.Unmarshal(msg.Bytes, &message); err != nil {
			return errors.Wrap(err, "json unmarshal")
		}

		t.incoming <- message

	case websocket.BinaryMessage:
		logger.InfoWithFields(ctx, []logger.Field{
			logger.Int("bytes", len(msg.Bytes)),
		}, "Received binary message")

		scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(msg.Bytes), -1)
		if err != nil {
			return errors.Wrap(err, "parse script")
		}

		var message MessageNotification
		if _, err := bsor.Unmarshal(scriptItems, &message); err != nil {
			return errors.Wrap(err, "bsor unmarshal")
		}

		t.incoming <- message
	}

	return nil
}

// post sends a request to the HTTP server using the POST method with an authentication bearer token
// header.
func post(ctx context.Context, url, token string, contentType string, request io.Reader,
	response interface{}) error {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	var client = &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, request)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	// Authorization: Bearer <token>
	if len(token) > 0 {
		httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	if request != nil && len(contentType) > 0 {
		httpRequest.Header.Set("Content-Type", contentType)
	}
	if response != nil {
		httpRequest.Header.Set("Accept", ContentTypeBinary)
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil {
				return HTTPError{
					Status:  httpResponse.StatusCode,
					Message: string(b),
				}
			}
		}

		return HTTPError{Status: httpResponse.StatusCode}
	}

	if response != nil {
		if httpResponse.Body == nil {
			return errors.New("No response body")
		}
		defer httpResponse.Body.Close()

		contentType := httpResponse.Header.Get("Content-Type")
		if contentType != ContentTypeBinary {
			return errors.Wrap(ErrWrongContentType, contentType)
		}

		b, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			return errors.Wrap(err, "read")
		}

		scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(b), -1)
		if err != nil {
			return errors.Wrap(err, "parse")
		}

		if _, err := bsor.Unmarshal(scriptItems, response); err != nil {
			return errors.Wrap(err, "unmarshal bsor")
		}
	}

	return nil
}

// get sends a request to the HTTP server using the GET method with an authentication bearer token
// header.
func get(ctx context.Context, url, token string, response interface{}) error {
	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	var client = &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	if response != nil {
		httpRequest.Header.Set("Accept", ContentTypeBinary)
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil {
				return HTTPError{
					Status:  httpResponse.StatusCode,
					Message: string(b),
				}
			}
		}

		return HTTPError{Status: httpResponse.StatusCode}
	}

	if response != nil {
		if httpResponse.Body == nil {
			return errors.New("No response body")
		}
		defer httpResponse.Body.Close()

		contentType := httpResponse.Header.Get("Content-Type")
		if contentType != ContentTypeBinary {
			return errors.Wrap(ErrWrongContentType, contentType)
		}

		b, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			return errors.Wrap(err, "read")
		}

		scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(b), -1)
		if err != nil {
			return errors.Wrap(err, "parse")
		}

		if _, err := bsor.Unmarshal(scriptItems, response); err != nil {
			return errors.Wrap(err, "unmarshal bsor")
		}
	}

	return nil
}

func httpDelete(ctx context.Context, url, token string) error {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	var client = &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	// Authorization: Bearer <token>
	if len(token) > 0 {
		httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil {
				return HTTPError{
					Status:  httpResponse.StatusCode,
					Message: string(b),
				}
			}
		}

		return HTTPError{Status: httpResponse.StatusCode}
	}

	return nil
}

// head sends a request to the HTTP server using the HEAD method with an authentication bearer token
// header.
func head(ctx context.Context, url, token string) (*http.Header, error) {
	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	var client = &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil {
				return nil, HTTPError{
					Status:  httpResponse.StatusCode,
					Message: string(b),
				}
			}
		}

		return nil, HTTPError{Status: httpResponse.StatusCode}
	}

	return &httpResponse.Header, nil
}

type websocketMessage struct {
	Type  int
	Bytes []byte
}

func websocketListen(ctx context.Context, url string, translator Translator,
	interrupt <-chan interface{}) error {

	header := make(http.Header)
	header.Set("Accept", fmt.Sprintf("%s, %s", ContentTypeBinary, ContentTypeJSON))

	conn, response, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		if errors.Cause(err) == websocket.ErrBadHandshake && response != nil {
			b, rerr := ioutil.ReadAll(response.Body)
			if rerr == nil {
				logger.WarnWithFields(ctx, []logger.Field{
					logger.String("body", string(b)),
				}, "Failed to dial websocket : %s", err)
				return errors.Wrap(err, "dial")
			}
		}

		logger.Warn(ctx, "Failed to dial websocket : %s", err)
		return errors.Wrap(err, "dial")
	}

	// Listen for messages in separate thread.
	done := make(chan interface{})
	go func() {
		for {
			messageType, messageBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure,
					websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.Info(ctx, "Websocket close : %s", err)
				} else {
					logger.Info(ctx, "Failed to read websocket message : %s", err)
					if err := conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseAbnormalClosure,
							"read failed")); err != nil {
						logger.Warn(ctx, "Failed to send abnormal websocket close : %s", err)
					}
				}

				conn.Close()
				break
			}

			if err := translator.Translate(ctx, websocketMessage{
				Type:  messageType,
				Bytes: messageBytes,
			}); err != nil {
				conn.Close()
				break
			}
		}

		logger.Info(ctx, "Finished listening for messages")
		close(done)
	}()

	wait := func() {
		start := time.Now()
		for {
			select {
			case <-time.After(time.Second):
				logger.WarnWithFields(ctx, []logger.Field{
					logger.Timestamp("start", start.UnixNano()),
					logger.MillisecondsFromNano("elapsed_ms", time.Since(start).Nanoseconds()),
				}, "Waiting for: Websocket Listen")

			case <-done:
				return
			}
		}
	}

	for {
		select {
		case <-time.After(30 * time.Second): // send ping every 30 seconds to keep alive
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"),
				time.Now().Add(time.Second)); err != nil {
				conn.Close()
				wait()
				return errors.Wrap(err, "send ping")
			}

		case <-done:
			return nil

		case <-interrupt:
			// Cleanly close the connection by sending a close message and then waiting (with timeout)
			// for the server to close the connection.
			if err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				wait()
				return errors.Wrap(err, "send close")
			}

			wait()
			return threads.Interrupted
		}
	}
}

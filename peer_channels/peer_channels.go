package peer_channels

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/bsor"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/logger"
	"github.com/tokenized/pkg/threads"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	ContentTypeText   = "text/plain"
	ContentTypeJSON   = "application/json"
	ContentTypeBinary = "application/octet-stream"
)

type Message struct {
	Sequence    uint32      `bsor:"1" json:"sequence"`
	Received    time.Time   `bsor:"2" json:"received"`
	ContentType string      `bsor:"3" json:"content_type"`
	Payload     bitcoin.Hex `bsor:"4" json:"payload"`
	ChannelID   uuid.UUID   `bsor:"5" json:"channel_id"`
}

type Messages []*Messages

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

// CreateAccount creates a new account on the SPVChannel service.
// Note: This is a non-standard endpoint and is only implemented by the Tokenized Service.
func CreateAccount(ctx context.Context, baseURL, token string) (*uuid.UUID, *uuid.UUID, error) {
	var response struct {
		AccountID uuid.UUID `json:"account_id"`
		Token     uuid.UUID `json:"token"`
	}
	if err := postJSONWithToken(ctx, baseURL+"/api/v1/account", token, nil, &response); err != nil {
		return nil, nil, err
	}

	return &response.AccountID, &response.Token, nil
}

func CreateChannel(ctx context.Context, baseURL, accountID, token string) (*Channel, error) {
	url := fmt.Sprintf("%s/api/v1/account/%s/channel", baseURL, accountID)

	var response Channel
	if err := postJSONWithToken(ctx, url, token, nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func PostTextMessage(ctx context.Context, baseURL, channelID, token string,
	message string) (*Message, error) {

	response := &Message{}
	if err := postTextWithToken(ctx, baseURL+"/api/v1/channel/"+channelID, token, message,
		response); err != nil {
		return nil, err
	}

	return response, nil
}

func PostJSONMessage(ctx context.Context, baseURL, channelID, token string,
	message interface{}) (*Message, error) {

	response := &Message{}
	if err := postJSONWithToken(ctx, baseURL+"/api/v1/channel/"+channelID, token, message,
		response); err != nil {
		return nil, err
	}

	return response, nil
}

func PostBinaryMessage(ctx context.Context, baseURL, channelID, token string,
	message []byte) (*Message, error) {

	response := &Message{}
	if err := postBinaryWithToken(ctx, baseURL+"/api/v1/channel/"+channelID, token, message,
		response); err != nil {
		return nil, err
	}

	return response, nil
}

func PostBSORMessage(ctx context.Context, baseURL, channelID, token string,
	message interface{}) (*Message, error) {

	response := &Message{}
	if err := postBSORWithToken(ctx, baseURL+"/api/v1/channel/"+channelID, token, message,
		response); err != nil {
		return nil, err
	}

	return response, nil
}

func GetMessages(ctx context.Context, baseURL, channelID, token string,
	unread bool) ([]*Message, error) {

	url := baseURL + "/api/v1/channel/" + channelID
	if unread {
		url += "?unread=true"
	} else {
		url += "?unread=false"
	}

	var response []*Message
	if err := getWithToken(ctx, url, token, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func GetMaxMessageSequence(ctx context.Context, baseURL, channelID, token string) (int, error) {
	url := baseURL + "/api/v1/channel/" + channelID

	headers, err := headWithToken(ctx, url, token)
	if err != nil {
		return 0, err
	}

	tag := headers.Get("ETag")
	if len(tag) == 0 {
		return 0, errors.New("Missing tag")
	}

	max, err := strconv.Atoi(tag)
	if err != nil {
		return 0, errors.Wrap(err, "parse tag")
	}

	return max, nil
}

func MarkMessages(ctx context.Context, baseURL, channelID, token string, sequence int,
	read, older bool) error {

	url := fmt.Sprintf("%s/api/v1/channel/%s/%d?older=%t", baseURL, channelID, sequence, older)

	type RequestData struct {
		Read bool `json:"read"`
	}
	requestData := RequestData{
		Read: read,
	}
	if err := postJSONWithToken(ctx, url, token, requestData, nil); err != nil {
		return err
	}

	return nil
}

// AccountListen starts a websocket for push notifications on the account specified. `incoming` is
// the channel new messages will be fed through. `interrupt` will stop listening if something is fed
// into it.
func AccountListen(ctx context.Context, baseURL, accountID, token string, incoming chan Message,
	interrupt <-chan interface{}) error {

	url := fmt.Sprintf("%s/api/v1/account/%s/notify", baseURL, accountID)
	url = strings.ReplaceAll(url, "http", "ws")

	header := make(http.Header)
	header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
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

			if messageType == websocket.TextMessage {
				logger.InfoWithFields(ctx, []logger.Field{
					logger.Int("bytes", len(messageBytes)),
				}, "Received text message")

				var message Message
				if err := json.Unmarshal(messageBytes, &message); err != nil {
					logger.Info(ctx, "Failed to json unmarshal message : %s", err)
					conn.Close()
					break
				}

				incoming <- message
				continue
			}

			if messageType == websocket.BinaryMessage {
				logger.InfoWithFields(ctx, []logger.Field{
					logger.Int("bytes", len(messageBytes)),
				}, "Received binary message")

				scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(messageBytes), -1)
				if err != nil {
					logger.Info(ctx, "Failed to parse message script items : %s", err)
					conn.Close()
					break
				}

				var message Message
				if _, err := bsor.Unmarshal(scriptItems, &message); err != nil {
					logger.Info(ctx, "Failed to bsor unmarshal message : %s", err)
					conn.Close()
					break
				}

				incoming <- message
				continue
			}

			logger.Info(ctx, "Wrong message type : %d", messageType)
			conn.Close()
			break
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
				}, "Waiting for: Listen SPV Channel")

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

// ChannelListen starts a websocket for push notifications on the channel specified. `incoming` is
// the channel new messages will be fed through. `interrupt` will stop listening if something is fed
// into it.
func ChannelListen(ctx context.Context, baseURL, channelID, token string, incoming chan Message,
	interrupt <-chan interface{}) error {

	url := fmt.Sprintf("%s/api/v1/channel/%s/notify", baseURL, channelID)
	url = strings.ReplaceAll(url, "http", "ws")

	header := make(http.Header)
	header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
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

			if messageType == websocket.TextMessage {
				logger.Info(ctx, "Received : %s", string(messageBytes))

				var message Message
				if err := json.Unmarshal(messageBytes, &message); err != nil {
					logger.Info(ctx, "Failed to json unmarshal message : %s", err)
					conn.Close()
					break
				}

				incoming <- message
				continue
			}

			if messageType == websocket.BinaryMessage {
				logger.Info(ctx, "Received : %d bytes", len(messageBytes))

				scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(messageBytes), -1)
				if err != nil {
					logger.Info(ctx, "Failed to parse message script items : %s", err)
					conn.Close()
					break
				}

				var message Message
				if _, err := bsor.Unmarshal(scriptItems, &message); err != nil {
					logger.Info(ctx, "Failed to bsor unmarshal message : %s", err)
					conn.Close()
					break
				}

				incoming <- message
				continue
			}

			logger.Info(ctx, "Wrong message type : %d", messageType)
			conn.Close()
			break
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
				}, "Waiting for: Listen SPV Channel")

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

func postTextWithToken(ctx context.Context, url, token, request string,
	response interface{}) error {

	return postWithToken(ctx, url, token, ContentTypeText, bytes.NewReader([]byte(request)),
		response)
}

func postJSONWithToken(ctx context.Context, url, token string,
	request, response interface{}) error {

	var r io.Reader
	if request != nil {
		var b []byte
		if s, ok := request.(string); ok {
			// request is already a json string, not an object to convert to json
			b = []byte(s)
		} else {
			bt, err := json.Marshal(request)
			if err != nil {
				return errors.Wrap(err, "marshal")
			}
			b = bt
		}
		r = bytes.NewReader(b)
	}

	return postWithToken(ctx, url, token, ContentTypeJSON, r, response)
}

func postBinaryWithToken(ctx context.Context, url, token string, request []byte,
	response interface{}) error {

	return postWithToken(ctx, url, token, ContentTypeBinary, bytes.NewReader(request), response)
}

func postBSORWithToken(ctx context.Context, url, token string,
	request, response interface{}) error {

	var r io.Reader
	if request != nil {
		scriptItems, err := bsor.Marshal(request)
		if err != nil {
			return errors.Wrap(err, "marshal")
		}

		script, err := scriptItems.Script()
		if err != nil {
			return errors.Wrap(err, "script")
		}

		r = bytes.NewReader(script)
	}

	return postWithToken(ctx, url, token, ContentTypeBinary, r, response)
}

// postWithToken sends a request to the HTTP server using the POST method with an authentication
// bearer token header.
func postWithToken(ctx context.Context, url, token string, contentType string, request io.Reader,
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
	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	if request != nil {
		httpRequest.Header.Set("Content-Type", contentType)
	}
	if response != nil {
		httpRequest.Header.Set("Accept", fmt.Sprintf("%s, %s", ContentTypeBinary, ContentTypeJSON))
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
		if len(contentType) == 0 || contentType == ContentTypeJSON {
			if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
				return errors.Wrap(err, "unmarshal json")
			}
		} else if contentType == ContentTypeBinary {
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
		} else {
			return fmt.Errorf("Unknown response content type : %s", contentType)
		}
	}

	return nil
}

// getWithToken sends a request to the HTTP server using the GET method with an authentication
// bearer token header.
func getWithToken(ctx context.Context, url, token string, response interface{}) error {
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
		httpRequest.Header.Set("Accept", fmt.Sprintf("%s, %s", ContentTypeBinary, ContentTypeJSON))
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
		if len(contentType) == 0 || contentType == ContentTypeJSON {
			if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
				return errors.Wrap(err, "unmarshal json")
			}
		} else if contentType == ContentTypeBinary {
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
		} else {
			return fmt.Errorf("Unknown response content type : %s", contentType)
		}
	}

	return nil
}

// headWithToken sends a request to the HTTP server using the HEAD method with an authentication
// bearer token header.
func headWithToken(ctx context.Context, url, token string) (*http.Header, error) {
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
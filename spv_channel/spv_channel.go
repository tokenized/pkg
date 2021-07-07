package spv_channel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/logger"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var (
	ErrHTTPNotFound = errors.New("HTTP Not Found")
)

type SPVMessage struct {
	Sequence    uint32    `json:"sequence"`
	Received    time.Time `json:"received"`
	ContentType string    `json:"content_type"`
	Payload     string    `json:"payload"`
}

// CreateAccount creates a new account on the SPVChannel service.
// Note: This is a non-standard endpoint and is only implemented by the Tokenized Service.
func CreateAccount(ctx context.Context, baseURL, token string) (*uuid.UUID, *uuid.UUID, error) {
	var response struct {
		AccountID uuid.UUID `json:"account_id"`
		Token     uuid.UUID `json:"token"`
	}
	if err := postWithToken(ctx, baseURL+"/api/v1/account", token, nil, &response); err != nil {
		return nil, nil, errors.Wrap(err, "http post")
	}

	return &response.AccountID, &response.Token, nil
}

func CreateChannel(ctx context.Context, baseURL, accountID, token string) (*Channel, error) {

	url := fmt.Sprintf("%s/api/v1/account/%s/channel", baseURL, accountID)

	var response Channel
	if err := postWithToken(ctx, url, token, nil, &response); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	return &response, nil
}

func PostMessage(ctx context.Context, baseURL, channelID, token string,
	message interface{}) (*SPVMessage, error) {

	response := &SPVMessage{}
	if err := postWithToken(ctx, baseURL+"/api/v1/channel/"+channelID, token, message,
		response); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	return response, nil
}

func GetMessages(ctx context.Context, baseURL, channelID, token string,
	unread bool) ([]*SPVMessage, error) {

	url := baseURL + "/api/v1/channel/" + channelID
	if unread {
		url += "?unread=true"
	} else {
		url += "?unread=false"
	}

	var response []*SPVMessage
	if err := getWithToken(ctx, url, token, &response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	return response, nil
}

func GetMaxMessageSequence(ctx context.Context, baseURL, channelID, token string) (int, error) {
	url := baseURL + "/api/v1/channel/" + channelID

	headers, err := headWithToken(ctx, url, token)
	if err != nil {
		return 0, errors.Wrap(err, "http head")
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
	if err := postWithToken(ctx, url, token, requestData, nil); err != nil {
		return errors.Wrap(err, "http post")
	}

	return nil
}

// NotifyMessages starts a websocket for push notifications. `incoming` is the channel new messages
// will be fed through. `interrupt` will stop listening if something is fed into it.
func NotifyMessages(ctx context.Context, baseURL, channelID, token string,
	incoming chan SPVMessage, interrupt chan interface{}) error {

	url := fmt.Sprintf("%s/api/v1/channel/%s/notify", baseURL, channelID)
	url = strings.ReplaceAll(url, "http", "ws")
	// u := url.URL{Scheme: "ws", Host: *addr, Path: "/echo"}

	header := make(http.Header)
	// Authorization: Bearer <token>
	header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		return errors.Wrap(err, "dial")
	}
	defer conn.Close()

	// Listen for messages in separate thread.
	done := make(chan interface{})
	go func() {
		for {
			messageType, messageBytes, err := conn.ReadMessage()
			if err != nil {
				closeError, ok := err.(*websocket.CloseError)
				if ok {
					if closeError.Code != websocket.CloseNormalClosure &&
						closeError.Code != websocket.CloseGoingAway {
						logger.Info(ctx, "Non-normal websocket close message received : %s", err)
					}
				} else {
					logger.Info(ctx, "Failed to read websocket message : %s", err)
					if err := conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseAbnormalClosure,
							"read failed")); err != nil {
						logger.Warn(ctx, "Failed to send abnormal websocket close : %s", err)
					}
				}

				break
			}

			if messageType != websocket.TextMessage {
				logger.Info(ctx, "Wrong message type : got %d, want %d", messageType,
					websocket.TextMessage)
				break
			}

			logger.Info(ctx, "Received : %s", string(messageBytes))

			var message SPVMessage
			if err := json.Unmarshal(messageBytes, &message); err != nil {
				logger.Info(ctx, "Failed to json unmarshal message : %s", err)
				break
			}

			incoming <- message
		}

		close(incoming)
		close(done)
	}()

	// Wait for interrupt or for listening to finish.
	select {
	case <-done:
		logger.Info(ctx, "Finished listening for messages")
		return nil

	case <-interrupt:
		logger.Info(ctx, "Listening stopped")

		// Cleanly close the connection by sending a close message and then waiting (with timeout)
		// for the server to close the connection.
		if err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			return errors.Wrap(err, "send close")
		}

		select {
		case <-done:
			logger.Info(ctx, "Finished listening for messages")
		case <-time.After(time.Second * 3):
			logger.Info(ctx, "Server didn't close connection")
		}

		return nil
	}
}

// postWithToken sends a request to the HTTP server using the POST method with an authentication
// bearer token header.
func postWithToken(ctx context.Context, url, token string, request, response interface{}) error {
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

	var r io.Reader
	if request != nil {
		var b []byte
		if s, ok := request.(string); ok {
			// request is already a json string, not an object to convert to json
			b = []byte(s)
		} else {
			bt, err := json.Marshal(request)
			if err != nil {
				return errors.Wrap(err, "marshal request")
			}
			b = bt
		}
		r = bytes.NewReader(b)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, r)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	// Authorization: Bearer <token>
	httpRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	if request != nil {
		httpRequest.Header.Add("Content-Type", "application/json")
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.StatusCode == 404 {
			return errors.Wrap(ErrHTTPNotFound, httpResponse.Status)
		}
		return fmt.Errorf("%v %s", httpResponse.StatusCode, httpResponse.Status)
	}

	defer httpResponse.Body.Close()

	if response != nil {
		if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
			return errors.Wrap(err, "decode response")
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

	httpRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.StatusCode == 404 {
			return errors.Wrap(ErrHTTPNotFound, httpResponse.Status)
		}
		return fmt.Errorf("%v %s", httpResponse.StatusCode, httpResponse.Status)
	}

	defer httpResponse.Body.Close()

	if response != nil {
		if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
			return errors.Wrap(err, "decode response")
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

	httpRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.StatusCode == 404 {
			return nil, errors.Wrap(ErrHTTPNotFound, httpResponse.Status)
		}
		return nil, fmt.Errorf("%v %s", httpResponse.StatusCode, httpResponse.Status)
	}

	return &httpResponse.Header, nil
}

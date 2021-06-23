package miner

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/tokenized/pkg/json"
)

type SPVMessage struct {
	Sequence    uint32    `json:"sequence"`
	Received    time.Time `json:"received"`
	ContentType string    `json:"content_type"`
	Payload     string    `json:"payload"`
}

func PostMessage(ctx context.Context, baseURL, channelID, token,
	message string) (*SPVMessage, error) {

	response := &SPVMessage{}
	if err := postWithToken(ctx, baseURL+"api/v1/channel/"+channelID, token, message,
		response); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	return response, nil
}

func GetMessages(ctx context.Context, baseURL, channelID, token string,
	unread bool) ([]*SPVMessage, error) {

	url := baseURL + "api/v1/channel/" + channelID
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

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	// Authorization: Bearer <token>
	httpRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	httpRequest.Header.Add("Content-Type", "application/json")

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

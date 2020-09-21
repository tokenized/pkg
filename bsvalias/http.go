package bsvalias

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	ErrNotFound = errors.New("Not Found")
)

// HttpClient represents a client for a paymail/bsvalias service that uses HTTP for requests.
type HttpClient struct {
	Handle   string
	Site     Site
	Alias    string
	Hostname string
}

// HttpFactory is a factory for creating HTTP clients.
type HttpFactory struct{}

// NewHttpFactory creates a new HTTP factory.
func NewHttpFactory() *HttpFactory {
	return &HttpFactory{}
}

// NewClient creates a new client.
func (f *HttpFactory) NewClient(ctx context.Context, handle string) (Client, error) {
	return NewHttpClient(ctx, handle)
}

func NewHttpClient(ctx context.Context, handle string) (*HttpClient, error) {
	result := HttpClient{
		Handle: handle,
	}

	fields := strings.Split(handle, "@")
	if len(fields) != 2 {
		return nil, errors.Wrap(ErrInvalidHandle, "split @ not 2")
	}

	result.Alias = fields[0]
	result.Hostname = fields[1]

	var err error
	result.Site, err = GetSite(ctx, result.Hostname)
	if err != nil {
		return nil, errors.Wrap(err, "get site")
	}

	return &result, nil
}

func (c *HttpClient) GetPublicKey(ctx context.Context) (*bitcoin.PublicKey, error) {

	url, err := c.Site.Capabilities.GetURL(URLNamePKI)
	if err != nil {
		return nil, errors.Wrap(err, URLNamePKI)
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response PublicKeyResponse
	if err := get(url, &response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	result, err := bitcoin.PublicKeyFromStr(response.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "parse public key")
	}

	return &result, nil
}

// GetPaymentDestination gets a locking script that can be used to send bitcoin.
// If senderKey is not nil then it must be associated with senderHandle and will be used to add a
//   signature to the request.
func (c *HttpClient) GetPaymentDestination(senderName, senderHandle, purpose string,
	amount uint64, senderKey *bitcoin.Key) ([]byte, error) {

	url, err := c.Site.Capabilities.GetURL(URLNamePaymentDestination)
	if err != nil {
		return nil, errors.Wrap(err, "payment request")
	}

	request := PaymentDestinationRequest{
		SenderName:   senderName,
		SenderHandle: senderHandle,
		DateTime:     time.Now().UTC().Format("2006-01-02T15:04:05.999Z"),
		Amount:       amount,
		Purpose:      purpose,
	}

	if senderKey != nil {
		sigHash, err := SignatureHashForMessage(request.SenderHandle +
			strconv.FormatUint(request.Amount, 10) + request.DateTime + request.Purpose)
		if err != nil {
			return nil, errors.Wrap(err, "signature hash")
		}

		sig, err := senderKey.Sign(sigHash.Bytes())
		if err != nil {
			return nil, errors.Wrap(err, "sign")
		}

		request.Signature = sig.ToCompact()
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response PaymentDestinationResponse
	if err := post(url, request, &response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	result, err := hex.DecodeString(response.Output)
	if err != nil {
		return nil, errors.Wrap(err, "parse script hex")
	}

	if len(result) == 0 {
		return nil, errors.New("Empty locking script")
	}

	return result, nil
}

// GetPaymentRequest gets a payment request from the identity.
//   senderHandle is required.
//   assetID can be empty or "BSV" to request bitcoin.
// If senderKey is not nil then it must be associated with senderHandle and will be used to add a
//   signature to the request.
func (c *HttpClient) GetPaymentRequest(senderName, senderHandle, purpose, assetID string,
	amount uint64, senderKey *bitcoin.Key) (*PaymentRequest, error) {

	url, err := c.Site.Capabilities.GetURL(URLNamePaymentRequest)
	if err != nil {
		return nil, errors.Wrap(err, "payment request")
	}

	request := PaymentRequestRequest{
		SenderName:   senderName,
		SenderHandle: senderHandle,
		DateTime:     time.Now().UTC().Format("2006-01-02T15:04:05.999Z"),
		AssetID:      assetID,
		Amount:       amount,
		Purpose:      purpose,
	}

	if senderKey != nil {
		sigHash, err := SignatureHashForMessage(request.SenderHandle + request.AssetID +
			strconv.FormatUint(request.Amount, 10) + request.DateTime + request.Purpose)
		if err != nil {
			return nil, errors.Wrap(err, "signature hash")
		}

		sig, err := senderKey.Sign(sigHash.Bytes())
		if err != nil {
			return nil, errors.Wrap(err, "sign")
		}

		request.Signature = sig.ToCompact()
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response PaymentRequestResponse
	if err := post(url, request, &response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	b, err := hex.DecodeString(response.PaymentRequest)
	if err != nil {
		return nil, errors.Wrap(err, "parse tx hex")
	}

	var result PaymentRequest
	if err := result.Tx.Deserialize(bytes.NewReader(b)); err != nil {
		return nil, errors.Wrap(err, "deserialize tx")
	}

	for _, outputHex := range response.Outputs {
		b, err := hex.DecodeString(outputHex)
		if err != nil {
			return nil, errors.Wrap(err, "parse output hex")
		}

		output := &wire.TxOut{}
		if err := output.Deserialize(bytes.NewReader(b), 1, 1); err != nil {
			return nil, errors.Wrap(err, "deserialize output")
		}

		result.Outputs = append(result.Outputs, output)
	}

	return &result, nil
}

func post(url string, request, response interface{}) error {
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

	b, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "marshal request")
	}

	httpResponse, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		if httpResponse.StatusCode == 404 {
			return errors.Wrap(ErrNotFound, httpResponse.Status)
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

func get(url string, response interface{}) error {
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

	httpResponse, err := client.Get(url)
	if err != nil {
		return err
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
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

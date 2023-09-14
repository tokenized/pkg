package bsvalias

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	DefaultDialTimeout    = time.Second * 10
	DefaultRequestTimeout = time.Second * 20
)

// HTTPClient represents a client for a paymail/bsvalias service that uses HTTP for requests.
type HTTPClient struct {
	Handle   string
	Site     Site
	Alias    string
	Hostname string

	DialTimeout, RequestTimeout time.Duration
}

// HTTPFactory is a factory for creating HTTP clients.
type HTTPFactory struct {
	DialTimeout, RequestTimeout time.Duration
}

// NewHTTPFactory creates a new HTTP factory.
func NewHTTPFactory() *HTTPFactory {
	return &HTTPFactory{
		DialTimeout:    DefaultDialTimeout,
		RequestTimeout: DefaultRequestTimeout,
	}
}

func (f *HTTPFactory) SetTimeouts(dialTimeout, requestTimeout time.Duration) {
	f.DialTimeout = dialTimeout
	f.RequestTimeout = requestTimeout
}

// NewClient creates a new client.
func (f *HTTPFactory) NewClient(ctx context.Context, handle string) (Client, error) {
	c, err := NewHTTPClient(ctx, handle)
	if err != nil {
		return nil, err
	}

	c.SetTimeouts(f.DialTimeout, f.RequestTimeout)
	return c, nil
}

// NewHTTPClient creates a new HTTPClient.
func NewHTTPClient(ctx context.Context, handle string) (*HTTPClient, error) {
	result := HTTPClient{
		DialTimeout:    DefaultDialTimeout,
		RequestTimeout: DefaultRequestTimeout,
		Handle:         handle,
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

func (c *HTTPClient) SetTimeouts(dialTimeout, requestTimeout time.Duration) {
	c.DialTimeout = dialTimeout
	c.RequestTimeout = requestTimeout
}

func (c *HTTPClient) IsCapable(name string) (bool, error) {
	if _, err := c.Site.Capabilities.GetURL(name); err != nil {
		if errors.Cause(err) == ErrNotCapable {
			return false, nil
		}

		return false, errors.Wrap(err, "capability url")
	}

	return true, nil
}

func (c *HTTPClient) RequiresNameSenderValidation() bool {
	return c.Site.Capabilities.RequiresNameSenderValidation()
}

// GetPublicKey gets the identity public key for the handle.
func (c *HTTPClient) GetPublicKey(ctx context.Context) (*bitcoin.PublicKey, error) {
	url, err := c.Site.Capabilities.GetURL(URLNamePKI)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response PublicKeyResponse
	if err := get(ctx, c.DialTimeout, c.RequestTimeout, url, &response); err != nil {
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
// signature to the request.
func (c *HTTPClient) GetPaymentDestination(ctx context.Context, senderName, senderHandle,
	purpose string, amount uint64, senderKey *bitcoin.Key) (bitcoin.Script, error) {

	url, err := c.Site.Capabilities.GetURL(URLNamePaymentDestination)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
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

		sig, err := senderKey.Sign(sigHash)
		if err != nil {
			return nil, errors.Wrap(err, "sign")
		}

		request.Signature = sig.ToCompact()
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response PaymentDestinationResponse
	if err := post(ctx, c.DialTimeout, c.RequestTimeout, url, request, &response); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	if len(response.Output) == 0 {
		return nil, errors.New("Empty locking script")
	}

	return response.Output, nil
}

// GetPaymentRequest gets a payment request from the identity.
//
//	senderHandle is required.
//	instrumentID can be empty or "BSV" to request bitcoin.
//
// If senderKey is not nil then it must be associated with senderHandle and will be used to add a
// signature to the request.
func (c *HTTPClient) GetPaymentRequest(ctx context.Context, senderName, senderHandle, purpose,
	instrumentID string, amount uint64, senderKey *bitcoin.Key) (*PaymentRequest, error) {

	url, err := c.Site.Capabilities.GetURL(URLNamePaymentRequest)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
	}

	request := PaymentRequestRequest{
		SenderName:   senderName,
		SenderHandle: senderHandle,
		DateTime:     time.Now().UTC().Format("2006-01-02T15:04:05.999Z"),
		InstrumentID: instrumentID,
		Amount:       amount,
		Purpose:      purpose,
	}

	if senderKey != nil {
		sigHash, err := SignatureHashForMessage(request.SenderHandle + request.InstrumentID +
			strconv.FormatUint(request.Amount, 10) + request.DateTime + request.Purpose)
		if err != nil {
			return nil, errors.Wrap(err, "signature hash")
		}

		sig, err := senderKey.Sign(sigHash)
		if err != nil {
			return nil, errors.Wrap(err, "sign")
		}

		request.Signature = sig.ToCompact()
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response PaymentRequestResponse
	if err := post(ctx, c.DialTimeout, c.RequestTimeout, url, request, &response); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	b, err := hex.DecodeString(response.PaymentRequest)
	if err != nil {
		return nil, errors.Wrap(err, "parse tx hex")
	}

	result := &PaymentRequest{
		Tx: wire.NewMsgTx(1),
	}
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

	if len(result.Tx.TxIn) != len(result.Outputs) {
		return nil, ErrWrongOutputCount
	}

	return result, nil
}

// GetP2PPaymentDestination requests a peer to peer payment destination.
func (c *HTTPClient) GetP2PPaymentDestination(ctx context.Context,
	value uint64) (*P2PPaymentDestinationOutputs, error) {

	url, err := c.Site.Capabilities.GetURL(URLNameP2PPaymentDestination)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
	}

	request := P2PPaymentDestinationRequest{
		Value: value,
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response P2PPaymentDestinationResponse
	if err := post(ctx, c.DialTimeout, c.RequestTimeout, url, request, &response); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	result := &P2PPaymentDestinationOutputs{
		Outputs:   make([]*wire.TxOut, len(response.Outputs)),
		Reference: response.Reference,
	}

	totalValue := uint64(0)
	for i, output := range response.Outputs {
		result.Outputs[i] = &wire.TxOut{
			LockingScript: output.Script,
			Value:         output.Value,
		}
		totalValue += output.Value
	}

	if totalValue != value {
		return nil, fmt.Errorf("Wrong value outputs : got %d, want %d", totalValue, value)
	}

	return result, nil
}

// PostP2PTransaction posts a P2P transaction to the handle being paid. The same as that used by the
// corresponding GetP2PPaymentDestination.
func (c *HTTPClient) PostP2PTransaction(ctx context.Context, senderHandle, note,
	reference string, senderKey *bitcoin.Key, tx *wire.MsgTx) (string, error) {

	url, err := c.Site.Capabilities.GetURL(URLNameP2PTransactions)
	if err != nil {
		return "", errors.Wrap(err, "capability url")
	}

	txid := *tx.TxHash()

	request := P2PTransactionRequest{
		Tx: tx,
		MetaData: P2PTransactionMetaData{
			Sender: senderHandle,
			Note:   note,
		},
		Reference: reference,
	}

	if senderKey != nil {
		if err := request.Sign(*senderKey); err != nil {
			return "", errors.Wrap(err, "sign txid")
		}
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response P2PTransactionResponse
	if err := post(ctx, c.DialTimeout, c.RequestTimeout, url, request, &response); err != nil {
		return "", errors.Wrap(err, "http post")
	}

	if !response.TxID.Equal(&txid) {
		return "", fmt.Errorf("Wrong txid returned : got %s, want %s", response.TxID, txid)
	}

	return response.Note, nil
}

// ListTokenizedInstruments returns the list of instrument aliases for this paymail handle.
func (c *HTTPClient) ListTokenizedInstruments(ctx context.Context) ([]InstrumentAlias, error) {
	url, err := c.Site.Capabilities.GetURL(URLNameListTokenizedInstrumentAlias)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	var response InstrumentAliasListResponse
	if err := get(ctx, c.DialTimeout, c.RequestTimeout, url, &response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	return response.InstrumentAliases, nil
}

// GetPublicProfile returns the public profile for this paymail handle.
func (c *HTTPClient) GetPublicProfile(ctx context.Context) (*PublicProfile, error) {
	url, err := c.Site.Capabilities.GetURL(URLNamePublicProfile)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	response := &PublicProfile{}
	if err := get(ctx, c.DialTimeout, c.RequestTimeout, url, response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	return response, nil

}

func (c *HTTPClient) PostNegotiationTx(ctx context.Context,
	tx *NegotiationTransaction) error {

	url, err := c.Site.Capabilities.GetURL(URLNameTransactionNegotiation)
	if err != nil {
		return errors.Wrap(err, "capability url")
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	status, body, err := postRaw(ctx, c.DialTimeout, c.RequestTimeout, url, tx)
	if err != nil {
		return errors.Wrap(err, "http post")
	}
	if body != nil {
		defer body.Close()
	}

	switch status {
	case http.StatusAccepted:
		return nil

	case http.StatusNotAcceptable:
		message := http.StatusText(status)
		if body != nil {
			b, rerr := ioutil.ReadAll(body)
			if rerr == nil && len(b) > 0 {
				message = string(b)
			}
		}

		return errors.Wrap(ErrNotSupported, message)

	case http.StatusNotFound:
		message := http.StatusText(status)
		if body != nil {
			b, rerr := ioutil.ReadAll(body)
			if rerr == nil && len(b) > 0 {
				message = string(b)
			}
		}

		return errors.Wrap(ErrNotFound, message)

	default:
		message := http.StatusText(status)
		if body != nil {
			b, rerr := ioutil.ReadAll(body)
			if rerr == nil && len(b) > 0 {
				message = string(b)
			}
		}

		return errors.Wrapf(ErrServiceFailure, "(status %d) %s", status, message)
	}
}

func (c *HTTPClient) GetNegotiationCapabilities(ctx context.Context) (*NegotiationCapabilities, error) {
	url, err := c.Site.Capabilities.GetURL(URLNameNegotiationCapabilities)
	if err != nil {
		return nil, errors.Wrap(err, "capability url")
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	response := &NegotiationCapabilities{}
	if err := get(ctx, c.DialTimeout, c.RequestTimeout, url, response); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	return response, nil
}

func (c *HTTPClient) PostMerkleProofs(ctx context.Context, merkleProofs MerkleProofs) error {
	url, err := c.Site.Capabilities.GetURL(URLNameMerkleProof)
	if err != nil {
		return errors.Wrap(err, "capability url")
	}

	url = strings.ReplaceAll(url, "{alias}", c.Alias)
	url = strings.ReplaceAll(url, "{domain.tld}", c.Hostname)

	if err := post(ctx, c.DialTimeout, c.RequestTimeout, url, merkleProofs, nil); err != nil {
		return errors.Wrap(err, "http post")
	}

	return nil
}

func postRaw(ctx context.Context, dialTimeout, requestTimeout time.Duration, url string,
	request interface{}) (int, io.ReadCloser, error) {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: dialTimeout,
		}).Dial,
		TLSHandshakeTimeout: dialTimeout,
	}

	var client = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return 0, nil, errors.Wrap(err, "marshal request")
	}

	httpResponse, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}

	return httpResponse.StatusCode, httpResponse.Body, nil
}

// post sends a request to the HTTP server using the POST method.
func post(ctx context.Context, dialTimeout, requestTimeout time.Duration, url string,
	request, response interface{}) error {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: dialTimeout,
		}).Dial,
		TLSHandshakeTimeout: dialTimeout,
	}

	var client = &http.Client{
		Timeout:   requestTimeout,
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
		message := httpResponse.Status
		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil && len(b) > 0 {
				message = string(b)
			}
		}

		if httpResponse.StatusCode == 404 {
			return errors.Wrap(ErrNotFound, message)
		}
		return fmt.Errorf("%v %s", httpResponse.StatusCode, message)
	}

	defer httpResponse.Body.Close()

	if response != nil {
		if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
			return errors.Wrap(err, "decode response")
		}
	}

	return nil
}

// get sends a request to the HTTP server using the GET method.
func get(ctx context.Context, dialTimeout, requestTimeout time.Duration, url string,
	response interface{}) error {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: dialTimeout,
		}).Dial,
		TLSHandshakeTimeout: dialTimeout,
	}

	var client = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}

	httpResponse, err := client.Get(url)
	if err != nil {
		return err
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		message := httpResponse.Status
		if httpResponse.Body != nil {
			b, rerr := ioutil.ReadAll(httpResponse.Body)
			if rerr == nil && len(b) > 0 {
				message = string(b)
			}
		}

		return fmt.Errorf("%v %s", httpResponse.StatusCode, message)
	}

	defer httpResponse.Body.Close()

	if response != nil {
		if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
			return errors.Wrap(err, "decode response")
		}
	}

	return nil
}

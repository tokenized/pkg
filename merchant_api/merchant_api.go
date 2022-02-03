package merchant_api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/json_envelope"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	ErrFailure        = errors.New("Failure")
	ErrDoubleSpend    = errors.New("Double Spend")
	ErrHTTPNotFound   = errors.New("HTTP Not Found")
	ErrWrongPublicKey = errors.New("Wrong Public Key")
)

const (
	CallBackReasonMerkleProof        = "merkleProof"
	CallBackReasonDoubleSpendAttempt = "doubleSpendAttempt"
	CallBackReasonDoubleSpend        = "doubleSpend"
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

type FeeQuoteResponse struct {
	Version            string            `json:"apiVersion"`
	Timestamp          time.Time         `json:"timestamp"`
	Expiry             time.Time         `json:"expiryTime"`
	MinerID            bitcoin.PublicKey `json:"minerId"`
	CurrentBlockHash   bitcoin.Hash32    `json:"currentHighestBlockHash"`
	CurrentBlockHeight uint32            `json:"currentHighestBlockHeight"`
	Fees               []FeeQuote        `json:"fees"`
}

type FeeQuote struct {
	FeeType   string `json:"feeType"`
	MiningFee Fee    `json:"miningFee"`
	RelayFee  Fee    `json:"relayFee"`
}

type Fee struct {
	Satoshis uint64 `json:"satoshis"`
	Bytes    uint64 `json:"bytes"`
}

func GetFeeQuote(ctx context.Context, baseURL string) (*FeeQuoteResponse, error) {
	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := get(ctx, baseURL+"/mapi/feeQuote", envelope); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	if envelope.MimeType != "application/json" {
		return nil, fmt.Errorf("MIME Type not JSON : %s", envelope.MimeType)
	}

	result := &FeeQuoteResponse{}
	if err := json.Unmarshal([]byte(envelope.Payload), result); err != nil {
		return nil, errors.Wrap(err, "json unmarshal")
	}

	return result, envelope.Verify()
}

type SubmitTxRequest struct {
	Tx                 *wire.MsgTx `json:"rawtx"`
	CallBackURL        *string     `json:"callBackUrl,omitempty"`
	CallBackToken      *string     `json:"callBackToken,omitempty"`
	SendMerkleProof    bool        `json:"merkleProof,omitempty"`
	MerkleProofFormat  *string     `json:"merkleFormat,omitempty"`
	DoubleSpendCheck   bool        `json:"dsCheck,omitempty"`
	CallBackEncryption *string     `json:"callBackEncryption,omitempty"`
}

// When tx broadcast by someone else:
//   "returnResult": "failure",
//   "resultDescription": "Transaction already in the mempool",
type SubmitTxResponse struct {
	Version                string            `json:"apiVersion"`
	Timestamp              time.Time         `json:"timestamp"`
	TxID                   bitcoin.Hash32    `json:"txid"`
	Result                 string            `json:"returnResult"`
	ResultDescription      string            `json:"resultDescription"`
	MinerID                bitcoin.PublicKey `json:"minerId"`
	CurrentBlockHash       bitcoin.Hash32    `json:"currentHighestBlockHash"`
	CurrentBlockHeight     uint32            `json:"currentHighestBlockHeight"`
	SecondaryMempoolExpiry uint32            `json:"txSecondMempoolExpiry"`
	Conflicts              []Conflict        `json:"conflictedWith"`
}

type Conflict struct {
	TxID bitcoin.Hash32 `json:"txid"`
	Size uint64         `json:"size"`
	Tx   *wire.MsgTx    `json:"hex"`
}

func (str SubmitTxResponse) Success() error {
	if len(str.Conflicts) != 0 {
		txids := make([]bitcoin.Hash32, len(str.Conflicts))
		for i, conflict := range str.Conflicts {
			txids[i] = conflict.TxID
		}
		return errors.Wrapf(ErrDoubleSpend, "%+v", txids)
	}

	if str.Result != "success" {
		return errors.Wrap(ErrFailure, str.Result)
	}

	return nil
}

// SubmitTxCallbackResponse is the message posted to the SPV channel specified in the
// SubmitTxRequest.CallBackURL.
// When Reason is "merkleProof" Payload is a merkle proof. If SubmitTxRequest.MerkleProofFormat was
// "TSC", then it follows the Technical Standards Committee's format which is also implemented in
// the package merkle_proof next to this one.
// When Reason is "doubleSpend" or "doubleSpendAttempt" then Payload is CallBackDoubleSpend.
type SubmitTxCallbackResponse struct {
	Version     string            `json:"apiVersion"`
	Reason      string            `json:"callbackReason"`
	TxID        *bitcoin.Hash32   `json:"callbackTxId"`
	MinerID     bitcoin.PublicKey `json:"minerId"`
	Timestamp   time.Time         `json:"timestamp"`
	BlockHash   bitcoin.Hash32    `json:"blockHash"`
	BlockHeight uint32            `json:"blockHeight"`
	Payload     json.RawMessage   `json:"callbackPayload"`
}

type CallBackDoubleSpend struct {
	TxID bitcoin.Hash32 `json:"doubleSpendTxId"`
	Tx   *wire.MsgTx    `json:"payload"`
}

func SubmitTx(ctx context.Context, baseURL string,
	request SubmitTxRequest) (*SubmitTxResponse, error) {

	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := post(ctx, baseURL+"/mapi/tx", request, envelope); err != nil {
		return nil, errors.Wrap(err, "http post")
	}

	if envelope.MimeType != "application/json" {
		return nil, fmt.Errorf("MIME Type not JSON : %s", envelope.MimeType)
	}

	result := &SubmitTxResponse{}
	if err := json.Unmarshal([]byte(envelope.Payload), result); err != nil {
		return nil, errors.Wrap(err, "json unmarshal")
	}

	if envelope.PublicKey != nil && !result.MinerID.Equal(*envelope.PublicKey) {
		return nil, ErrWrongPublicKey
	}

	return result, envelope.Verify()
}

type GetTxStatusResponse struct {
	Version                string            `json:"apiVersion"`
	Timestamp              time.Time         `json:"timestamp"`
	TxID                   *bitcoin.Hash32   `json:"txid"`
	Result                 string            `json:"returnResult"`
	ResultDescription      string            `json:"resultDescription"`
	MinerID                bitcoin.PublicKey `json:"minerId"`
	BlockHash              *bitcoin.Hash32   `json:"blockHash"`
	BlockHeight            *uint32           `json:"blockHeight"`
	Confirmations          *uint32           `json:"confirmations"`
	SecondaryMempoolExpiry uint32            `json:"txSecondMempoolExpiry"`
}

func GetTxStatus(ctx context.Context, baseURL string,
	txid bitcoin.Hash32) (*GetTxStatusResponse, error) {

	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := get(ctx, baseURL+"/mapi/tx/"+txid.String(), envelope); err != nil {
		return nil, errors.Wrap(err, "http get")
	}

	if envelope.MimeType != "application/json" {
		return nil, fmt.Errorf("MIME Type not JSON : %s", envelope.MimeType)
	}

	result := &GetTxStatusResponse{}
	if err := json.Unmarshal([]byte(envelope.Payload), result); err != nil {
		return nil, errors.Wrap(err, "json unmarshal")
	}

	if envelope.PublicKey != nil && !result.MinerID.Equal(*envelope.PublicKey) {
		return nil, ErrWrongPublicKey
	}

	return result, envelope.Verify()
}

// post sends a request to the HTTP server using the POST method.
func post(ctx context.Context, url string, request, response interface{}) error {
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

	defer httpResponse.Body.Close()

	if response != nil {
		if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
			return errors.Wrap(err, "decode response")
		}
	}

	return nil
}

// get sends a request to the HTTP server using the GET method.
func get(ctx context.Context, url string, response interface{}) error {
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

	defer httpResponse.Body.Close()

	if response != nil {
		if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
			return errors.Wrap(err, "decode response")
		}
	}

	return nil
}

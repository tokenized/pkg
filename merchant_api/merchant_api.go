package merchant_api

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	ErrTimeout        = errors.New("Timeout")

	// AlreadyInMempool can be returned when submitting a tx that the miner has already seen. It
	// doesn't mean the tx is invalid, but it does mean you will not get the callbacks. The "status"
	// endpoint should be checked to verify the tx is valid.
	AlreadyInMempool = errors.New("Already In Mempool")

	// MissingInputs can be returned when you are submitting a tx and the inputs are already spent,
	// or don't exist. One scenario is when submitting a tx that was included in a block a while ago
	// it will return missing inputs because the inputs were spent long enough in the past. It
	// doesn't mean the tx is invalid, but it does mean you will not get the callbacks. The "status"
	// endpoint should be checked to verify the tx is valid.
	MissingInputs = errors.New("Missing Inputs")
)

const (
	CallBackReasonMerkleProof        = "merkleProof"
	CallBackReasonDoubleSpendAttempt = "doubleSpendAttempt"
	CallBackReasonDoubleSpend        = "doubleSpend"

	CallBackMerkleProofFormat = "TSC"

	FeeQuoteTypeStandard = "standard"
	FeeQuoteTypeData     = "data" // only bytes in scripts that start with OP_RETURN or OP_FALSE, OP_RETURN
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
	Fees               []*FeeQuote       `json:"fees"`
	CallBacks          []*FeeCallBack    `json:"callbacks"`
	Policies           *FeePolicies      `json:"policies"`
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

type FeeCallBack struct {
	IPAddress string `json:"ipAddress"`
}

type FeePolicies struct {
	SkipScriptFlags               []string `json:"skipscriptflags"`
	MaxTxSize                     int      `json:"maxtxsizepolicy"`
	DataCarrierSize               int      `json:"datacarriersize"`
	MaxScriptSize                 int      `json:"maxscriptsizepolicy"`
	MaxScriptNumberLength         int      `json:"maxscriptnumlengthpolicy"`
	MaxStackMemoryUsage           int      `json:"maxstackmemoryusagepolicy"`
	AncestorCountLimit            int      `json:"limitancestorcount"`
	CPFPGroupMembersCountLimit    int      `json:"limitcpfpgroupmemberscount"`
	AcceptNonStdOutputs           bool     `json:"acceptnonstdoutputs"`
	DataCarrier                   bool     `json:"datacarrier"`
	DustRelayFee                  int      `json:"dustrelayfee"`
	MaxStdTxValidationDuration    int      `json:"maxstdtxvalidationduration"`
	MaxNonStdTxValidationDuration int      `json:"maxnonstdtxvalidationduration"`
	DustLimitFactor               int      `json:"dustlimitfactor"`
}

func GetFeeQuote(ctx context.Context, baseURL string) (*FeeQuoteResponse, error) {
	return GetFeeQuoteWithAuth(ctx, baseURL, "")
}

func GetFeeQuoteWithAuth(ctx context.Context,
	baseURL, authToken string) (*FeeQuoteResponse, error) {

	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := get(ctx, baseURL+"/mapi/feeQuote", authToken, envelope); err != nil {
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

	if str.Result == "success" {
		return nil
	}

	if str.Result == "failure" && str.ResultDescription == "Transaction already in the mempool" {
		return AlreadyInMempool
	}

	if str.Result == "failure" && str.ResultDescription == "Missing inputs" {
		return MissingInputs
	}

	return errors.Wrap(ErrFailure, str.Result)
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
	return SubmitTxWithAuth(ctx, baseURL, request, "")
}

func SubmitTxWithAuth(ctx context.Context, baseURL string,
	request SubmitTxRequest, authToken string) (*SubmitTxResponse, error) {

	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := post(ctx, baseURL+"/mapi/tx", authToken, request, envelope); err != nil {
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

func (str GetTxStatusResponse) Success() error {
	if str.Result != "success" {
		return errors.Wrap(ErrFailure, str.Result)
	}

	return nil
}

// GetTxStatus returns the status of a tx. If it is confirmed it will return valid.
func GetTxStatus(ctx context.Context, baseURL string,
	txid bitcoin.Hash32) (*GetTxStatusResponse, error) {
	return GetTxStatusWithAuth(ctx, baseURL, txid, "")
}

func GetTxStatusWithAuth(ctx context.Context, baseURL string,
	txid bitcoin.Hash32, authToken string) (*GetTxStatusResponse, error) {

	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := get(ctx, baseURL+"/mapi/tx/"+txid.String(), authToken, envelope); err != nil {
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
func post(ctx context.Context, url, authToken string, request, response interface{}) error {
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

	httpRequest.Header.Add("Authorization", authToken)
	if request != nil {
		httpRequest.Header.Add("Content-Type", "application/json")
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		if errors.Cause(err) == context.DeadlineExceeded {
			return errors.Wrap(ErrTimeout, errors.Wrap(err, "http post").Error())
		}
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
func get(ctx context.Context, url, authToken string, response interface{}) error {
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

	httpRequest.Header.Add("Authorization", authToken)

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		if errors.Cause(err) == context.DeadlineExceeded {
			return errors.Wrap(ErrTimeout, errors.Wrap(err, "http get").Error())
		}
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

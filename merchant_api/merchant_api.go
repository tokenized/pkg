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
	AlreadyInMempool  = errors.New("Already In Mempool")
	ErrHTTPNotFound   = errors.New("HTTP Not Found")
	ErrWrongPublicKey = errors.New("Wrong Public Key")
	ErrTimeout        = errors.New("Timeout")
)

const (
	CallBackReasonMerkleProof        = "merkleProof"
	CallBackReasonDoubleSpendAttempt = "doubleSpendAttempt"
	CallBackReasonDoubleSpend        = "doubleSpend"

	CallBackMerkleProofFormat = "TSC"

	// FeeTypeStandard is for any bytes in the tx that don't fall in another fee type.
	FeeTypeStandard = FeeType(0)

	// FeeTypeData only applies to bytes in scripts that start with OP_RETURN or OP_FALSE, OP_RETURN.
	FeeTypeData = FeeType(1)
)

type FeeType uint8

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
	Version            string            `bsor:"1" json:"apiVersion"`
	Timestamp          time.Time         `bsor:"2" json:"timestamp"`
	Expiry             time.Time         `bsor:"3" json:"expiryTime"`
	MinerID            bitcoin.PublicKey `bsor:"4" json:"minerId"`
	CurrentBlockHash   bitcoin.Hash32    `bsor:"5" json:"currentHighestBlockHash"`
	CurrentBlockHeight uint32            `bsor:"6" json:"currentHighestBlockHeight"`
	Fees               FeeQuotes         `bsor:"7" json:"fees"`
	CallBacks          []*FeeCallBack    `bsor:"8" json:"callbacks"`
	Policies           *FeePolicies      `bsor:"9" json:"policies"`
}

type FeeQuote struct {
	FeeType   FeeType `bsor:"1" json:"feeType"`
	MiningFee Fee     `bsor:"2" json:"miningFee"`
	RelayFee  Fee     `bsor:"3" json:"relayFee"`
}

type FeeQuotes []*FeeQuote

type Fee struct {
	Satoshis uint64 `bsor:"1" json:"satoshis"`
	Bytes    uint64 `bsor:"2" json:"bytes"`
}

func (f Fee) Rate() float32 {
	return float32(f.Satoshis) / float32(f.Bytes)
}

func (q FeeQuotes) GetQuote(t FeeType) *FeeQuote {
	for _, quote := range q {
		if quote.FeeType == t {
			return quote
		}
	}

	return nil
}

func (v *FeeType) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("Too short for FeeType : %d", len(data))
	}

	return v.SetString(string(data[1 : len(data)-1]))
}

func (v FeeType) MarshalJSON() ([]byte, error) {
	s := v.String()
	if len(s) == 0 {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf("\"%s\"", s)), nil
}

func (v FeeType) MarshalText() ([]byte, error) {
	s := v.String()
	if len(s) == 0 {
		return nil, fmt.Errorf("Unknown FeeType value \"%d\"", uint8(v))
	}

	return []byte(s), nil
}

func (v *FeeType) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *FeeType) SetString(s string) error {
	switch s {
	case "standard":
		*v = FeeTypeStandard
	case "data":
		*v = FeeTypeData
	default:
		*v = FeeTypeStandard
		return fmt.Errorf("Unknown FeeType value \"%s\"", s)
	}

	return nil
}

func (v FeeType) String() string {
	switch v {
	case FeeTypeStandard:
		return "standard"
	case FeeTypeData:
		return "data"
	default:
		return ""
	}
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

	if str.Result == "success" {
		return nil
	}

	if str.Result == "failure" && str.ResultDescription == "Transaction already in the mempool" {
		return AlreadyInMempool
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

func (str GetTxStatusResponse) Success() error {
	if str.Result != "success" {
		return errors.Wrap(ErrFailure, str.Result)
	}

	return nil
}

// GetTxStatus returns the status of a tx. If it is confirmed it will return valid.
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

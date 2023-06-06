package merchant_api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/json_envelope"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	ErrDoubleSpend    = errors.New("Double Spend")
	ErrHTTPNotFound   = errors.New("HTTP Not Found")
	ErrWrongPublicKey = errors.New("Wrong Public Key")
	ErrTimeout        = errors.New("Timeout")

	// NotFound means the tx is not known by the service. Most likely from a tx status request.
	NotFound = errors.New("Not Found")

	// AlreadyInMempool can be returned when submitting a tx that the miner has already seen. It
	// doesn't mean the tx is invalid, but it does mean you will not get the callbacks.
	AlreadyInMempool = errors.New("Already In Mempool")

	// ExistingTx can be returned when submitting a tx that the miner has already seen. It
	// doesn't mean the tx is invalid, but it does mean you will not get the callbacks.
	ExistingTx = errors.New("Existing Tx")

	// MissingInputs can be returned when you are submitting a tx and the inputs are already spent,
	// or don't exist. One scenario is when submitting a tx that was included in a block a while ago
	// it will return missing inputs because the inputs were spent long enough in the past. It
	// doesn't mean the tx is invalid, but it does mean you will not get the callbacks. The "status"
	// endpoint should be checked to verify the tx is valid.
	MissingInputs = errors.New("Missing Inputs")

	// ConflictingTx means there is a transaction that has conflicting inputs. In other words a
	// double spend attempt.
	ConflictingTx = errors.New("Mempool Conflict")

	// InsufficientFee means the tx fee is too low to be mined.
	InsufficientFee = errors.New("Insufficient Fee")

	// ErrServiceFailure means that the service failed to process the request.
	ErrServiceFailure = errors.New("Service Failure")

	// ErrSafeMode seems to mean that there is a pending block chain fork and the service can't
	// currently verify the transaction as valid.
	ErrSafeMode = errors.New("Safe Mode")

	// ErrUnsupportedFailure means the merchant api returned a "failure" with an unrecognized
	// message.
	ErrUnsupportedFailure = errors.New("Unsupported failure")
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

// IsRejectError returns true if the error represents the node actually rejecting the tx and saying
// it will not be mined.
func IsRejectError(err error) bool {
	switch errors.Cause(err) {
	case MissingInputs, InsufficientFee, ErrDoubleSpend:
		return true
	default:
		return false
	}
}

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
	return GetFeeQuoteWithAuth(ctx, baseURL, "")
}

func GetFeeQuoteWithAuth(ctx context.Context,
	baseURL, authToken string) (*FeeQuoteResponse, error) {
	return GetFeeQuoteFull(ctx, baseURL, authToken, time.Second*10)
}

func GetFeeQuoteFull(ctx context.Context, baseURL, authToken string,
	timeout time.Duration) (*FeeQuoteResponse, error) {

	if len(baseURL) == 0 {
		return nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := get(ctx, timeout, baseURL+"/mapi/feeQuote", authToken, envelope); err != nil {
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

func (r SubmitTxRequest) Copy() SubmitTxRequest {
	result := SubmitTxRequest{
		SendMerkleProof:  r.SendMerkleProof,
		DoubleSpendCheck: r.DoubleSpendCheck,
	}

	if r.Tx != nil {
		c := r.Tx.Copy()
		result.Tx = &c
	}

	if r.CallBackURL != nil {
		s := CopyString(*r.CallBackURL)
		result.CallBackURL = &s
	}

	if r.CallBackToken != nil {
		s := CopyString(*r.CallBackToken)
		result.CallBackToken = &s
	}

	if r.MerkleProofFormat != nil {
		s := CopyString(*r.MerkleProofFormat)
		result.MerkleProofFormat = &s
	}

	if r.CallBackEncryption != nil {
		s := CopyString(*r.CallBackEncryption)
		result.CallBackEncryption = &s
	}

	return result
}

func CopyString(s string) string {
	result := make([]byte, len(s))
	copy(result, s)
	return string(result)
}

// When tx broadcast by someone else:
//   "returnResult": "failure",
//   "resultDescription": "Transaction already in the mempool",
type SubmitTxResponse struct {
	Version                string            `json:"apiVersion"`
	Timestamp              time.Time         `json:"timestamp"`
	TxID                   *bitcoin.Hash32   `json:"txid"`
	Result                 string            `json:"returnResult"`
	ResultDescription      string            `json:"resultDescription"`
	MinerID                bitcoin.PublicKey `json:"minerId"`
	CurrentBlockHash       bitcoin.Hash32    `json:"currentHighestBlockHash"`
	CurrentBlockHeight     uint32            `json:"currentHighestBlockHeight"`
	SecondaryMempoolExpiry uint32            `json:"txSecondMempoolExpiry"`
	Conflicts              []Conflict        `json:"conflictedWith"`
}

type Conflict struct {
	TxID *bitcoin.Hash32 `json:"txid"`
	Size uint64          `json:"size"`
	Tx   *wire.MsgTx     `json:"hex"`
}

func (r SubmitTxResponse) Success() error {
	if len(r.Conflicts) != 0 {
		txids := make([]bitcoin.Hash32, len(r.Conflicts))
		for i, conflict := range r.Conflicts {
			if conflict.TxID != nil {
				txids[i] = *conflict.TxID
			} else if conflict.Tx != nil {
				txids[i] = *conflict.Tx.TxHash()
			}
		}
		return errors.Wrapf(ErrDoubleSpend, "%+v", txids)
	}

	return translateResult(r.Result, r.ResultDescription)
}

// GeneralResponse can be used to check the success of submit tx and tx status response payloads.
type GeneralResponse struct {
	Result            string `json:"returnResult"`
	ResultDescription string `json:"resultDescription"`
}

func (r GeneralResponse) Error() error {
	return translateResult(r.Result, r.ResultDescription)
}

func translateResult(result, description string) error {
	if result == "success" {
		return nil
	}

	if result == "failure" {
		if strings.Contains(description, "txn-already-known") {
			return errors.Wrap(ExistingTx, description)
		}

		if strings.Contains(description, "txn-mempool-conflict") {
			return errors.Wrap(ConflictingTx, description)
		}

		if strings.Contains(description, "Transaction already in the mempool") {
			return errors.Wrap(AlreadyInMempool, description)
		}

		if strings.Contains(description, "Missing inputs") {
			return errors.Wrap(MissingInputs, description)
		}

		if strings.Contains(description, "Not enough fees") {
			return errors.Wrap(InsufficientFee, description)
		}

		if strings.Contains(description, "Error while submitting") {
			return errors.Wrap(ErrServiceFailure, description)
		}

		if strings.Contains(description, "Safe mode") {
			return errors.Wrap(ErrSafeMode, description)
		}

		if strings.Contains(description, "No such mempool or blockchain transaction") {
			return errors.Wrap(NotFound, description)
		}

		if strings.Contains(description, "Transaction already submitted with different parameters") {
			// Transactions seem to still confirm. I am not sure what parameters are different. --ce
			return errors.Wrap(AlreadyInMempool, description)
		}
	}

	return errors.Wrap(ErrUnsupportedFailure, result)
}

func translateHTTPError(err error) error {
	httpError, ok := errors.Cause(err).(HTTPError)
	if !ok {
		return err
	}

	if httpError.Status != 400 { // HTTP 400 Bad Request
		return err
	}

	if strings.Contains(httpError.Message, "Insufficient fees") {
		return errors.Wrap(InsufficientFee, err.Error())
	}

	return err
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

func SubmitTxWithAuth(ctx context.Context, baseURL string, request SubmitTxRequest,
	authToken string) (*SubmitTxResponse, error) {

	_, response, err := SubmitTxFull(ctx, baseURL, time.Second*10, request, authToken)
	return response, err
}

func SubmitTxFull(ctx context.Context, baseURL string, timeout time.Duration,
	request SubmitTxRequest,
	authToken string) (*json_envelope.JSONEnvelope, *SubmitTxResponse, error) {

	if len(baseURL) == 0 {
		return nil, nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := post(ctx, timeout, baseURL+"/mapi/tx", authToken, request, envelope); err != nil {
		return nil, nil, translateHTTPError(errors.Wrap(err, "http post"))
	}

	if envelope.MimeType != "application/json" {
		return envelope, nil, fmt.Errorf("MIME Type not JSON : %s", envelope.MimeType)
	}

	result := &SubmitTxResponse{}
	if err := json.Unmarshal([]byte(envelope.Payload), result); err != nil {
		return envelope, nil, errors.Wrap(err, "json unmarshal")
	}

	if envelope.PublicKey != nil && !result.MinerID.Equal(*envelope.PublicKey) {
		return envelope, result, ErrWrongPublicKey
	}

	return envelope, result, envelope.Verify()
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
	return translateResult(str.Result, str.ResultDescription)
}

// GetTxStatus returns the status of a tx. If it is confirmed it will return valid.
func GetTxStatus(ctx context.Context, baseURL string,
	txid bitcoin.Hash32) (*GetTxStatusResponse, error) {
	return GetTxStatusWithAuth(ctx, baseURL, txid, "")
}

func GetTxStatusWithAuth(ctx context.Context, baseURL string,
	txid bitcoin.Hash32, authToken string) (*GetTxStatusResponse, error) {

	_, response, err := GetTxStatusFull(ctx, baseURL, time.Second*10, txid, authToken)
	return response, err
}

func GetTxStatusFull(ctx context.Context, baseURL string, timeout time.Duration,
	txid bitcoin.Hash32,
	authToken string) (*json_envelope.JSONEnvelope, *GetTxStatusResponse, error) {

	if len(baseURL) == 0 {
		return nil, nil, fmt.Errorf("Invalid Base URL : %s", baseURL)
	}

	if baseURL[len(baseURL)-1:] == "/" {
		baseURL = baseURL[:len(baseURL)-1]
	}

	envelope := &json_envelope.JSONEnvelope{}
	if err := get(ctx, timeout, baseURL+"/mapi/tx/"+txid.String(), authToken,
		envelope); err != nil {
		return nil, nil, translateHTTPError(errors.Wrap(err, "http get"))
	}

	if envelope.MimeType != "application/json" {
		return envelope, nil, fmt.Errorf("MIME Type not JSON : %s", envelope.MimeType)
	}

	result := &GetTxStatusResponse{}
	if err := json.Unmarshal([]byte(envelope.Payload), result); err != nil {
		return envelope, nil, errors.Wrap(err, "json unmarshal")
	}

	if envelope.PublicKey != nil && !result.MinerID.Equal(*envelope.PublicKey) {
		return envelope, nil, ErrWrongPublicKey
	}

	return envelope, result, envelope.Verify()
}

// post sends a request to the HTTP server using the POST method.
func post(ctx context.Context, timeout time.Duration, url, authToken string,
	request, response interface{}) error {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: timeout,
		}).Dial,
		TLSHandshakeTimeout: timeout,
	}

	var client = &http.Client{
		Timeout:   timeout,
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
func get(ctx context.Context, timeout time.Duration, url, authToken string,
	response interface{}) error {

	var transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: timeout,
		}).Dial,
		TLSHandshakeTimeout: timeout,
	}

	var client = &http.Client{
		Timeout:   timeout,
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

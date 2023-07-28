package bsvalias

import (
	"fmt"
	"strconv"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/expanded_tx"
	"github.com/tokenized/pkg/fees"
	"github.com/tokenized/pkg/merkle_proof"
	"github.com/tokenized/pkg/peer_channels"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

const (
	// StatusOK means the request was valid and accepted.
	StatusOK = Status(0)

	// StatusReject means the request was rejected. The CodeProtocolID and Code should explain the
	// reason.
	StatusReject = Status(1)

	// StatusInvalid means something in the request was invalid. The CodeProtocolID and Code should
	// explain the reason.
	StatusInvalid = Status(2)

	// StatusUnauthorized means the request is not permitted.
	StatusUnauthorized = Status(3)

	// StatusUnsupportedProtocol means the message received used a protocol not supported by
	// this software.
	StatusUnsupportedProtocol = Status(4)

	// StatusUnwanted means the request message received was valid, but the recipient doesn't
	// want to accept it.
	StatusUnwanted = Status(5)

	// StatusNeedPayment means that a payment request was previously exchanged and not yet
	// fulfilled. Until that is fulfilled or renegotiated further requests will be rejected.
	StatusNeedPayment = Status(6)

	// StatusChannelInUse means the peer channel the request was received on is already in use
	// for another purpose.
	StatusChannelInUse = Status(7)

	// StatusSystemIssue means there was a systems issue and it was important to respond, but
	// a successful response was not possible.
	StatusSystemIssue = Status(8)
)

// Capabilities contains the information about the endpoints supported by the bsvalias host.
type Capabilities struct {
	Version      string                 `json:"bsvalias"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// Site represents a bsvalias host.
type Site struct {
	Capabilities Capabilities
	URL          string `json:"url"`
}

// PublicKeyResponse is the raw response from a PublicKey endpoint.
type PublicKeyResponse struct {
	Version   string `json:"bsvalias"`
	Handle    string `json:"handle"`
	PublicKey string `json:"pubkey"`
}

// PaymentDestinationRequest is the data structure sent to request a payment destination.
type PaymentDestinationRequest struct {
	SenderName   string `json:"senderName"`
	SenderHandle string `json:"senderHandle"`
	DateTime     string `json:"dt"`
	Amount       uint64 `json:"amount"`
	Purpose      string `json:"purpose"`
	Signature    string `json:"signature"`
}

// Sign adds a signature to the request. The key should correspond to the sender handle's PKI.
func (r *PaymentDestinationRequest) Sign(key bitcoin.Key) error {
	sigHash, err := SignatureHashForMessage(r.SenderHandle + strconv.FormatUint(r.Amount, 10) +
		r.DateTime + r.Purpose)
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := key.Sign(sigHash)
	if err != nil {
		return errors.Wrap(err, "sign")
	}

	r.Signature = sig.ToCompact()
	return nil
}

func (r PaymentDestinationRequest) CheckSignature(publicKey bitcoin.PublicKey) error {
	sigHash, err := SignatureHashForMessage(r.SenderHandle + strconv.FormatUint(r.Amount, 10) +
		r.DateTime + r.Purpose)
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := bitcoin.SignatureFromCompact(r.Signature)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("parse signature: %s", r.Signature))
	}

	if !sig.Verify(sigHash, publicKey) {
		return ErrInvalidSignature
	}

	return nil
}

type P2PPaymentDestinationRequest struct {
	Value uint64 `json:"satoshis"`
}

type P2PTransactionRequest struct {
	Tx        *wire.MsgTx            `json:"hex"`
	MetaData  P2PTransactionMetaData `json:"metadata"`
	Reference string                 `json:"reference"` // From prior P2PPaymentDestinationResponse
}

type P2PTransactionMetaData struct {
	Sender    string             `json:"sender,omitempty"`
	Key       *bitcoin.PublicKey `json:"pubkey,omitempty"`
	Signature string             `json:"signature,omitempty"`
	Note      string             `json:"note,omitempty"`
}

// Sign adds a signature to the request. The key should correspond to the sender handle's PKI.
func (r *P2PTransactionRequest) Sign(key bitcoin.Key) error {
	// Sign txid
	txid := *r.Tx.TxHash()

	sigHash, err := SignatureHashForMessage(txid.String())
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := key.Sign(sigHash)
	if err != nil {
		return errors.Wrap(err, "sign txid")
	}

	r.MetaData.Signature = sig.ToCompact()

	publicKey := key.PublicKey()
	r.MetaData.Key = &publicKey

	return nil
}

func (r P2PTransactionRequest) CheckSignature(publicKey bitcoin.PublicKey) error {
	txid := *r.Tx.TxHash()

	sigHash, err := SignatureHashForMessage(txid.String())
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := bitcoin.SignatureFromCompact(r.MetaData.Signature)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("parse signature: %s", r.MetaData.Signature))
	}

	if !sig.Verify(sigHash, publicKey) {
		return ErrInvalidSignature
	}

	return nil
}

// PaymentDestinationResponse is the raw response from a PaymentDestination endpoint.
type PaymentDestinationResponse struct {
	Output bitcoin.Script `json:"output"`
}

// P2PPaymentDestinationResponse is the raw response from a PaymentDestination endpoint.
type P2PPaymentDestinationResponse struct {
	Outputs   []P2PPaymentDestinationOutput `json:"outputs"`
	Reference string                        `json:"reference"` // Used to identify transaction when returned
}

type P2PPaymentDestinationOutput struct {
	Script bitcoin.Script `json:"script"`
	Value  uint64         `json:"satoshis"`
}

type P2PPaymentDestinationOutputs struct {
	Outputs   []*wire.TxOut
	Reference string
}

type P2PTransactionResponse struct {
	TxID bitcoin.Hash32 `json:"txid"`
	Note string         `json:"note,omitempty"`
}

// PaymentRequestRequest is the data structure sent to request a payment request.
type PaymentRequestRequest struct {
	SenderName   string `json:"senderName"`
	SenderHandle string `json:"senderHandle"`
	DateTime     string `json:"dt"`
	InstrumentID string `json:"instrumentID"`
	Amount       uint64 `json:"amount"`
	Purpose      string `json:"purpose"`
	Signature    string `json:"signature"`
}

// Sign adds a signature to the request. The key should correspond to the sender handle's PKI.
func (r *PaymentRequestRequest) Sign(key bitcoin.Key) error {
	sigHash, err := SignatureHashForMessage(r.SenderHandle + r.InstrumentID +
		strconv.FormatUint(r.Amount, 10) + r.DateTime + r.Purpose)
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := key.Sign(sigHash)
	if err != nil {
		return errors.Wrap(err, "sign")
	}

	r.Signature = sig.ToCompact()
	return nil
}

func (r PaymentRequestRequest) CheckSignature(publicKey bitcoin.PublicKey) error {
	sigHash, err := SignatureHashForMessage(r.SenderHandle + r.InstrumentID +
		strconv.FormatUint(r.Amount, 10) + r.DateTime + r.Purpose)
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := bitcoin.SignatureFromCompact(r.Signature)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("parse signature: %s", r.Signature))
	}

	if !sig.Verify(sigHash, publicKey) {
		return ErrInvalidSignature
	}

	return nil
}

// PaymentRequestResponse is the raw response from a PaymentRequest endpoint.
type PaymentRequestResponse struct {
	PaymentRequest string   `json:"paymentRequest"`
	Outputs        []string `json:"outputs"`
}

// PaymentRequest is the processed response from a PaymentRequest endpoint.
type PaymentRequest struct {
	Tx      *wire.MsgTx
	Outputs []*wire.TxOut
}

type InstrumentAliasListResponse struct {
	InstrumentAliases []InstrumentAlias `json:"instrument_aliases"`
}

type InstrumentAlias struct {
	InstrumentAlias string `json:"instrument_alias"`
	InstrumentID    string `json:"instrument_id"`
}

type PublicProfile struct {
	// Name is the name of the owner of the paymail (person, business). Max 100 characters
	Name *string `json:"name,omitempty"`

	// AvatarURL is a URL that returns a 180 by 180 image. It can accept an optional parameter `s`
	// to return an image of width and height `s`
	AvatarURL *string `json:"avatar,omitempty"`
}

type NegotiationTransaction struct {
	// ThreadID is a unique "conversation" ID for the negotiation. Responses should include the same
	// ID. UUIDs are recommended.
	ThreadID *string `json:"thread_id,omitempty"`

	// Fees specifies any requirements for fees when modifying the transaction.
	Fees fees.FeeRequirements `json:"fees,omitempty"`

	// ReplyTo is information on how to respond to the message.
	ReplyTo *ReplyTo `json:"reply_to,omitempty"`

	// Note is optional text that is displayed to the user.
	Note *string `json:"note,omitempty"`

	// Expiry is the nanoseconds since the unix epoch until this transaction expires.
	Expiry *uint64 `json:"expiry,omitempty"`

	// Timestamp is the nanoseconds since the unix epoch until when this transaction was created.
	Timestamp *uint64 `json:"timestamp,omitempty"`

	Response *Response `json:"response,omitempty"`

	// Tx is the current state of the negotiation. It will start as a partial transaction, likely
	// missing inputs and/or outputs.
	Tx *expanded_tx.ExpandedTx `json:"expanded_tx,omitempty"`
}

type ReplyTo struct {
	PeerChannel *peer_channels.Channel `json:"peer_channel"`
	Handle      *string                `json:"handle"`
}

var (
	ProtocolIDResponse = bitcoin.Hex("RE") // Protocol ID for channel response messages
)

type Status uint32

type Response struct {
	Status         Status      `json:"status,omitempty"`
	CodeProtocolID bitcoin.Hex `json:"protocol_id,omitempty"`
	Code           uint32      `json:"code,omitempty"` // Protocol specific codes
	Note           string      `json:"note,omitempty"`
}

type MerkleProofs merkle_proof.MerkleProofs

func (v *Status) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("Too short for Status : %d", len(data))
	}

	return v.SetString(string(data[1 : len(data)-1]))
}

func (v Status) MarshalJSON() ([]byte, error) {
	s := v.String()
	if len(s) == 0 {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf("\"%s\"", s)), nil
}

func (v Status) MarshalText() ([]byte, error) {
	s := v.String()
	if len(s) == 0 {
		return nil, fmt.Errorf("Unknown Status value \"%d\"", uint8(v))
	}

	return []byte(s), nil
}

func (v *Status) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *Status) SetString(s string) error {
	switch s {
	case "ok":
		*v = StatusOK
	case "reject":
		*v = StatusReject
	case "invalid":
		*v = StatusInvalid
	case "unauthorized":
		*v = StatusUnauthorized
	case "unsupported_protocol":
		*v = StatusUnsupportedProtocol
	case "unwanted":
		*v = StatusUnwanted
	case "need_payment":
		*v = StatusNeedPayment
	case "in_use":
		*v = StatusChannelInUse
	case "system_issue":
		*v = StatusSystemIssue
	default:
		*v = StatusInvalid
		return fmt.Errorf("Unknown Status value \"%s\"", s)
	}

	return nil
}

func (v Status) String() string {
	switch v {
	case StatusOK:
		return "ok"
	case StatusReject:
		return "reject"
	case StatusInvalid:
		return "invalid"
	case StatusUnauthorized:
		return "unauthorized"
	case StatusUnsupportedProtocol:
		return "unsupported_protocol"
	case StatusUnwanted:
		return "unwanted"
	case StatusNeedPayment:
		return "need_payment"
	case StatusChannelInUse:
		return "in_use"
	case StatusSystemIssue:
		return "system_issue"
	default:
		return ""
	}
}

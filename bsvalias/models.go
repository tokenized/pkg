package bsvalias

import (
	"fmt"
	"strconv"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
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

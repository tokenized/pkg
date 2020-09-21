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

	sig, err := key.Sign(sigHash.Bytes())
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

	if !sig.Verify(sigHash.Bytes(), publicKey) {
		return ErrInvalidSignature
	}

	return nil
}

// PaymentDestinationResponse is the raw response from a PaymentDestination endpoint.
type PaymentDestinationResponse struct {
	Output string `json:"output"`
}

// PaymentRequestRequest is the data structure sent to request a payment request.
type PaymentRequestRequest struct {
	SenderName   string `json:"senderName"`
	SenderHandle string `json:"senderHandle"`
	DateTime     string `json:"dt"`
	AssetID      string `json:"assetID"`
	Amount       uint64 `json:"amount"`
	Purpose      string `json:"purpose"`
	Signature    string `json:"signature"`
}

// Sign adds a signature to the request. The key should correspond to the sender handle's PKI.
func (r *PaymentRequestRequest) Sign(key bitcoin.Key) error {
	sigHash, err := SignatureHashForMessage(r.SenderHandle + r.AssetID +
		strconv.FormatUint(r.Amount, 10) + r.DateTime + r.Purpose)
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := key.Sign(sigHash.Bytes())
	if err != nil {
		return errors.Wrap(err, "sign")
	}

	r.Signature = sig.ToCompact()
	return nil
}

func (r PaymentRequestRequest) CheckSignature(publicKey bitcoin.PublicKey) error {
	sigHash, err := SignatureHashForMessage(r.SenderHandle + r.AssetID +
		strconv.FormatUint(r.Amount, 10) + r.DateTime + r.Purpose)
	if err != nil {
		return errors.Wrap(err, "signature hash")
	}

	sig, err := bitcoin.SignatureFromCompact(r.Signature)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("parse signature: %s", r.Signature))
	}

	if !sig.Verify(sigHash.Bytes(), publicKey) {
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

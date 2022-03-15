package json_envelope

import (
	"crypto/sha256"
	"errors"

	"github.com/tokenized/pkg/bitcoin"
)

var (
	ErrInvalidJSONSignature = errors.New("Invalid JSON Signature")

	ErrJSONNotSigned = errors.New("JSON Not Signed")
)

type JSONEnvelope struct {
	Payload   string             `json:"payload"`
	Signature *bitcoin.Signature `json:"signature"`
	PublicKey *bitcoin.PublicKey `json:"publicKey"`
	Encoding  string             `json:"encoding"`
	MimeType  string             `json:"mimetype"`
}

// Verify verifies the signature is valid.
func (je *JSONEnvelope) Verify() error {
	if je.Signature == nil || je.PublicKey == nil {
		return ErrJSONNotSigned
	}

	hash := bitcoin.Hash32(sha256.Sum256([]byte(je.Payload)))

	if !je.Signature.Verify(hash, *je.PublicKey) {
		return ErrInvalidJSONSignature
	}

	return nil
}

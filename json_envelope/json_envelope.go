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
	Payload   string             `bsor:"1" json:"payload"`
	Signature *bitcoin.Signature `bsor:"2" json:"signature"`
	PublicKey *bitcoin.PublicKey `bsor:"3" json:"publicKey"`
	Encoding  string             `bsor:"4" json:"encoding"`
	MimeType  string             `bsor:"5" json:"mimetype"`
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

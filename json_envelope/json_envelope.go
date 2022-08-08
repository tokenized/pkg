package json_envelope

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
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

func WrapJSON(key bitcoin.Key, payloadStruct interface{}) (*JSONEnvelope, error) {
	js, err := json.Marshal(payloadStruct)
	if err != nil {
		return nil, errors.Wrap(err, "marshal payload")
	}

	hash := bitcoin.Hash32(sha256.Sum256(js))
	publicKey := key.PublicKey()

	sig, err := key.Sign(hash)
	if err != nil {
		return nil, errors.Wrap(err, "sign")
	}

	return &JSONEnvelope{
		Payload:   string(js),
		Signature: &sig,
		PublicKey: &publicKey,
		Encoding:  "UTF-8",
		MimeType:  "application/json",
	}, nil
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

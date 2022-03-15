package bitcoin

import (
	"encoding/hex"
	"errors"
)

var (
	ErrMissingQuotes = errors.New("Must be contained in quotes")
)

// Hex is used in structures as a byte slice that will marshal as hex instead of base64 like is
// default for json.
type Hex []byte

func (b Hex) MarshalJSON() ([]byte, error) {
	return ConvertBytesToJSONHex(b)
}

func (b *Hex) UnmarshalJSON(data []byte) error {
	d, err := ConvertJSONHexToBytes(data)
	if err != nil {
		return err
	}

	*b = d
	return nil
}

func (b Hex) MarshalText() ([]byte, error) {
	result := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(result, b)
	return result, nil
}

func (b *Hex) UnmarshalText(text []byte) error {
	d := make([]byte, hex.DecodedLen(len(text)))
	_, err := hex.Decode(d, text)
	if err != nil {
		return err
	}

	*b = d
	return nil
}

func (b Hex) MarshalBinary() ([]byte, error) {
	return b, nil
}

func (b *Hex) UnmarshalBinary(data []byte) error {
	*b = data
	return nil
}

func ConvertBytesToJSONHex(b []byte) ([]byte, error) {
	hexLen := hex.EncodedLen(len(b))

	result := make([]byte, hexLen+2)
	result[0] = '"'
	hex.Encode(result[1:], b)
	result[hexLen+1] = '"'

	return result, nil
}

func ConvertJSONHexToBytes(js []byte) ([]byte, error) {
	l := len(js)
	if l < 2 {
		return nil, ErrMissingQuotes
	}
	if js[0] != '"' || js[l-1] != '"' {
		return nil, ErrMissingQuotes
	}

	byteLen := hex.DecodedLen(l - 2)
	b := make([]byte, byteLen)
	_, err := hex.Decode(b, js[1:l-1])
	if err != nil {
		return nil, err
	}

	return b, nil
}

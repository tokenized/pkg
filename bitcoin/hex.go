package bitcoin

import (
	"encoding/hex"

	"github.com/pkg/errors"
)

const (
	hexChars  = "0123456789abcdef"
	hexValues = "\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\xff\xff\xff\xff\xff\xff" +
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff"
)

var (
	ErrMissingQuotes = errors.New("Must be contained in quotes")
)

// Hex is used in structures as a byte slice that will marshal as hex instead of base64 like is
// default for json.
type Hex []byte

func (b Hex) String() string {
	result := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(result, b)
	return string(result)
}

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

func ConvertJSONHexToReverseBytes(js []byte) ([]byte, error) {
	l := len(js)
	if l < 2 {
		return nil, ErrMissingQuotes
	}
	if js[0] != '"' || js[l-1] != '"' {
		return nil, ErrMissingQuotes
	}

	byteLen := hex.DecodedLen(l - 2)
	if byteLen%2 == 1 {
		return nil, errors.Wrapf(hex.ErrLength, "%d", byteLen)
	}

	b := make([]byte, byteLen)
	h := js[1 : l-1]
	j := 0
	for i := byteLen - 1; i >= 0; i-- {
		hf := h[j]
		f := hexValues[hf]
		if f == 0xff {
			return nil, hex.InvalidByteError(hf)
		}
		j++

		hs := h[j]
		j++
		h := hexValues[hs]
		if h == 0xff {
			return nil, hex.InvalidByteError(hs)
		}

		b[i] = (f << 4) + h
	}

	return b, nil
}

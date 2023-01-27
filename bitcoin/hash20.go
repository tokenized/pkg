package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"github.com/pkg/errors"
)

var (
	ErrWrongSize = errors.New("Wrong size")
)

const Hash20Size = 20

// Hash20 is a 20 byte integer in little endian format.
type Hash20 [Hash20Size]byte

func NewHash20(b []byte) (*Hash20, error) {
	if len(b) != Hash20Size {
		return nil, errors.Wrapf(ErrWrongSize, "got %d, want %d", len(b), Hash20Size)
	}
	result := Hash20{}
	copy(result[:], b)
	return &result, nil
}

// NewHash20FromStr creates a little endian hash from a big endian string.
func NewHash20FromStr(s string) (*Hash20, error) {
	result := &Hash20{}
	if err := result.SetString(s); err != nil {
		return nil, err
	}
	return result, nil
}

// NewHash20FromData creates a Hash20 by hashing the data with a Ripemd160(Sha256(b))
func NewHash20FromData(b []byte) (*Hash20, error) {
	return NewHash20(Ripemd160(Sha256(b)))
}

// Bytes returns the data for the hash.
func (h Hash20) Bytes() []byte {
	return h[:]
}

// Bytes returns the bytes in reverse order (big endian).
func (h Hash20) ReverseBytes() []byte {
	b := make([]byte, Hash20Size)
	reverse20(b, h[:])
	return b
}

func (h Hash20) Value() *big.Int {
	value := &big.Int{}
	value.SetBytes(h.ReverseBytes())
	return value
}

// SetBytes sets the value of the hash.
func (h *Hash20) SetBytes(b []byte) error {
	if len(b) != Hash20Size {
		return errors.Wrapf(ErrWrongSize, "got %d, want %d", len(b), Hash20Size)
	}
	copy(h[:], b)
	return nil
}

func (h *Hash20) SetString(s string) error {
	if len(s) != 2*Hash20Size {
		return errors.Wrapf(ErrWrongSize, "hex: got %d, want %d", len(s), Hash20Size*2)
	}

	j := 0
	for i := Hash20Size - 1; i >= 0; i-- {
		hf := s[j]
		f := hexValues[hf]
		if f == 0xff {
			return hex.InvalidByteError(hf)
		}
		j++

		hs := s[j]
		j++
		s := hexValues[hs]
		if s == 0xff {
			return hex.InvalidByteError(hs)
		}

		h[i] = (f << 4) + s
	}

	return nil
}

// String returns the hex for the hash.
func (h Hash20) String() string {
	var hex [Hash20Size * 2]byte
	i := (Hash20Size * 2) - 1
	for _, b := range h[:] {
		hex[i] = hexChars[b&0x0f]
		i--

		hex[i] = hexChars[b>>4]
		i--
	}
	return string(hex[:])
}

// Equal returns true if the parameter has the same value.
func (h *Hash20) Equal(o *Hash20) bool {
	if h == nil {
		return o == nil
	}
	if o == nil {
		return false
	}
	return bytes.Equal(h[:], o[:])
}

func (h Hash20) Copy() Hash20 {
	var c Hash20
	copy(c[:], h[:])
	return c
}

func (h Hash20) IsZero() bool {
	var zero Hash20 // automatically initializes to zero
	return h.Equal(&zero)
}

// Serialize writes the hash into a writer.
func (h Hash20) Serialize(w io.Writer) error {
	_, err := w.Write(h[:])
	return err
}

func (h *Hash20) Deserialize(r io.Reader) error {
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return err
	}
	return nil
}

// Deserialize reads a hash from a reader.
func DeserializeHash20(r io.Reader) (*Hash20, error) {
	result := Hash20{}
	_, err := io.ReadFull(r, result[:])
	if err != nil {
		return nil, err
	}

	return &result, err
}

// MarshalJSON converts to json.
func (h Hash20) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", h)), nil
}

// UnmarshalJSON converts from json.
func (h *Hash20) UnmarshalJSON(data []byte) error {
	b, err := ConvertJSONHexToReverseBytes(data)
	if err != nil {
		return errors.Wrap(err, "hex")
	}

	if len(b) == 0 {
		h = nil
		return nil
	}

	return h.SetBytes(b)
}

// MarshalText returns the text encoding of the hash.
// Implements encoding.TextMarshaler interface.
func (h Hash20) MarshalText() ([]byte, error) {
	result := h.String()
	return []byte(result), nil
}

// UnmarshalText parses a text encoded hash and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (h *Hash20) UnmarshalText(text []byte) error {
	return h.SetString(string(text))
}

func (h Hash20) MarshalBinaryFixedSize() int {
	return 20
}

// MarshalBinary returns the binary encoding of the hash.
// Implements encoding.BinaryMarshaler interface.
func (h Hash20) MarshalBinary() ([]byte, error) {
	return h.Bytes(), nil
}

// UnmarshalBinary parses a binary encoded hash and sets the value of this object.
// Implements encoding.BinaryUnmarshaler interface.
func (h *Hash20) UnmarshalBinary(data []byte) error {
	return h.SetBytes(data)
}

// Scan converts from a database column.
func (h *Hash20) Scan(data interface{}) error {
	b, ok := data.([]byte)
	if !ok {
		return errors.New("Hash20 db column not bytes")
	}

	return h.SetBytes(b)
}

func reverse20(h, rh []byte) {
	i := Hash20Size - 1
	for _, b := range rh[:] {
		h[i] = b
		i--
	}
}

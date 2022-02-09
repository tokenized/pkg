package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"github.com/pkg/errors"
)

const Hash32Size = 32

// Hash32 is a 32 byte integer in little endian format.
type Hash32 [Hash32Size]byte

func NewHash32(b []byte) (*Hash32, error) {
	if len(b) != Hash32Size {
		return nil, errors.Wrapf(ErrWrongSize, "got %d, want %d", len(b), Hash32Size)
	}
	result := Hash32{}
	copy(result[:], b)
	return &result, nil
}

// NewHash32FromStr creates a little endian hash from a big endian string.
func NewHash32FromStr(s string) (*Hash32, error) {
	if len(s) != 2*Hash32Size {
		return nil, errors.Wrapf(ErrWrongSize, "hex: got %d, want %d", len(s), Hash32Size*2)
	}

	b := make([]byte, Hash32Size)
	_, err := hex.Decode(b, []byte(s[:]))
	if err != nil {
		return nil, err
	}

	result := Hash32{}
	reverse32(result[:], b)
	return &result, nil
}

// Sha256 sets the value of this hash to the SHA256 of itself.
func (h *Hash32) Sha256() {
	copy(h[:], Sha256(h[:]))
}

// AddHashes adds the value of the hashes together using big integer modular math.
func AddHashes(l, r Hash32) Hash32 {
	b := addPrivateKeys(l[:], r[:])
	result, _ := NewHash32(b) // Ignore error because addPrivateKeys always returns 32 bytes
	return *result
}

// Bytes returns the data for the hash.
func (h Hash32) Bytes() []byte {
	return h[:]
}

// Bytes returns the bytes in reverse order (big endian).
func (h Hash32) ReverseBytes() []byte {
	b := make([]byte, Hash32Size)
	reverse32(b, h[:])
	return b
}

func (h Hash32) Value() *big.Int {
	value := &big.Int{}
	value.SetBytes(h.ReverseBytes())
	return value
}

// SetBytes sets the value of the hash.
func (h *Hash32) SetBytes(b []byte) error {
	if len(b) != Hash32Size {
		return errors.Wrapf(ErrWrongSize, "got %d, want %d", len(b), Hash32Size)
	}
	copy(h[:], b)
	return nil
}

// String returns the hex for the hash.
func (h Hash32) String() string {
	var r [Hash32Size]byte
	reverse32(r[:], h[:])
	return fmt.Sprintf("%x", r[:])
}

// Equal returns true if the parameter has the same value.
func (h *Hash32) Equal(o *Hash32) bool {
	if h == nil {
		return o == nil
	}
	if o == nil {
		return false
	}
	return bytes.Equal(h[:], o[:])
}

func (h Hash32) IsZero() bool {
	var zero Hash32 // automatically initializes to zero
	return h.Equal(&zero)
}

// Serialize writes the hash into a writer.
func (h Hash32) Serialize(w io.Writer) error {
	_, err := w.Write(h[:])
	return err
}

func (h *Hash32) Deserialize(r io.Reader) error {
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return err
	}
	return nil
}

// Deserialize reads a hash from a reader.
func DeserializeHash32(r io.Reader) (*Hash32, error) {
	result := Hash32{}
	_, err := io.ReadFull(r, result[:])
	if err != nil {
		return nil, err
	}

	return &result, err
}

// MarshalJSON converts to json.
func (h Hash32) MarshalJSON() ([]byte, error) {
	var r [Hash32Size]byte
	reverse32(r[:], h[:])
	return []byte(fmt.Sprintf("\"%x\"", r[:])), nil
}

// UnmarshalJSON converts from json.
func (h *Hash32) UnmarshalJSON(data []byte) error {
	b, err := ConvertJSONHexToBytes(data)
	if err != nil {
		return errors.Wrap(err, "hex")
	}

	if len(b) == 0 {
		h = nil
		return nil
	}

	if len(b) != Hash32Size {
		return fmt.Errorf("Wrong size hex for Hash32 : %d", len(b)*2)
	}

	reverse32(h[:], b)
	return nil
}

// MarshalText returns the text encoding of the hash.
// Implements encoding.TextMarshaler interface.
func (h Hash32) MarshalText() ([]byte, error) {
	b := h.Bytes()
	result := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(result, b)
	return result, nil
}

// UnmarshalText parses a text encoded hash and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (h *Hash32) UnmarshalText(text []byte) error {
	b := make([]byte, hex.DecodedLen(len(text)))
	_, err := hex.Decode(b, text)
	if err != nil {
		return err
	}

	return h.SetBytes(b)
}

// MarshalBinary returns the binary encoding of the hash.
// Implements encoding.BinaryMarshaler interface.
func (h Hash32) MarshalBinary() ([]byte, error) {
	return h.Bytes(), nil
}

// UnmarshalBinary parses a binary encoded hash and sets the value of this object.
// Implements encoding.BinaryUnmarshaler interface.
func (h *Hash32) UnmarshalBinary(data []byte) error {
	return h.SetBytes(data)
}

// Scan converts from a database column.
func (h *Hash32) Scan(data interface{}) error {
	b, ok := data.([]byte)
	if !ok {
		return errors.New("Hash32 db column not bytes")
	}

	return h.SetBytes(b)
}

func reverse32(h, rh []byte) {
	i := Hash32Size - 1
	for _, b := range rh[:] {
		h[i] = b
		i--
	}
}

package bitcoin

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
)

const (
	// These values are place holders in the template for where the public key values should be
	// swapped in when instantiating the template.
	OP_PUBKEY     = 0xb8 // OP_NOP9 - Must be replaced by a public key
	OP_PUBKEYHASH = 0xb9 // OP_NOP10 - Must be replaced by a public key hash
)

var (
	ErrNotEnoughPublicKeys = errors.New("Not Enough Public Keys")

	PKHTemplate = Template{OP_DUP, OP_HASH160, OP_PUBKEYHASH, OP_EQUALVERIFY, OP_CHECKSIG}

	MultiPKHWrap = Script{OP_IF, OP_DUP, OP_HASH160, OP_PUBKEYHASH, OP_EQUALVERIFY,
		OP_CHECKSIGVERIFY, OP_FROMALTSTACK, OP_1ADD, OP_TOALTSTACK, OP_ENDIF}
)

// Template represents a locking script that is incomplete. It represents the function of the
// locking script without the public keys or other specific values needed to make it complete.
type Template Script

func NewMultiPKHTemplate(required, total int) (Template, error) {
	result := &bytes.Buffer{}

	// Push zero to alt stack to initialize the counter.
	if err := result.WriteByte(OP_0); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	if err := result.WriteByte(OP_TOALTSTACK); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	// Wrap each key in an if statement and a P2PKH locking script
	for i := 0; i < total; i++ {
		if _, err := result.Write(MultiPKHWrap); err != nil {
			return nil, errors.Wrap(err, "write")
		}
	}

	if err := result.WriteByte(OP_FROMALTSTACK); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	if _, err := result.Write(PushNumberScript(int64(required))); err != nil {
		return nil, errors.Wrap(err, "write")
	}

	if err := result.WriteByte(OP_GREATERTHANOREQUAL); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	return Template(result.Bytes()), nil
}

// LockingScript populates the template with public key values and creates a locking script.
func (t Template) LockingScript(publicKeys []PublicKey) (Script, error) {
	result := &bytes.Buffer{}
	buf := bytes.NewReader(t)
	pubKeyIndex := 0

	for {
		opCode, data, err := ParsePushDataScript(buf)
		if err == io.EOF {
			break
		}

		if err == nil {
			if err := WritePushDataScript(result, data); err != nil {
				return nil, errors.Wrap(err, "write push data")
			}
			continue
		}

		if err != ErrNotPushOp {
			return nil, errors.Wrap(err, "parse")
		}

		switch opCode {
		case OP_PUBKEY:
			if pubKeyIndex >= len(publicKeys) {
				return nil, ErrNotEnoughPublicKeys
			}

			if err := WritePushDataScript(result, publicKeys[pubKeyIndex].Bytes()); err != nil {
				return nil, errors.Wrap(err, "write public key")
			}

			pubKeyIndex++
			continue

		case OP_PUBKEYHASH:
			if pubKeyIndex >= len(publicKeys) {
				return nil, ErrNotEnoughPublicKeys
			}

			if err := WritePushDataScript(result,
				Hash160(publicKeys[pubKeyIndex].Bytes())); err != nil {
				return nil, errors.Wrap(err, "write public key")
			}

			pubKeyIndex++
			continue
		}

		// Op Code
		if err := result.WriteByte(opCode); err != nil {
			return nil, errors.Wrap(err, "write op code")
		}
	}

	return NewScript(result.Bytes()), nil
}

func (t Template) String() string {
	return ScriptToString(t)
}

func (t Template) Bytes() []byte {
	return t
}

// MarshalText returns the text encoding of the raw address.
// Implements encoding.TextMarshaler interface.
func (t Template) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText parses a text encoded raw address and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (t *Template) UnmarshalText(text []byte) error {
	b, err := StringToScript(string(text))
	if err != nil {
		return errors.Wrap(err, "script to string")
	}

	return t.UnmarshalBinary(b)
}

// MarshalBinary returns the binary encoding of the raw address.
// Implements encoding.BinaryMarshaler interface.
func (t Template) MarshalBinary() ([]byte, error) {
	return t.Bytes(), nil
}

// UnmarshalBinary parses a binary encoded raw address and sets the value of this object.
// Implements encoding.BinaryUnmarshaler interface.
func (t *Template) UnmarshalBinary(data []byte) error {
	// Copy byte slice in case it is reused after this call.
	*t = make([]byte, len(data))
	copy(*t, data)
	return nil
}

// Scan converts from a database column.
func (t *Template) Scan(data interface{}) error {
	if data == nil {
		*t = nil
		return nil
	}

	b, ok := data.([]byte)
	if !ok {
		return errors.New("Template db column not bytes")
	}

	if len(b) == 0 {
		*t = nil
		return nil
	}

	// Copy byte slice because it will be wiped out by the database after this call.
	*t = make([]byte, len(b))
	copy(*t, b)

	return nil
}

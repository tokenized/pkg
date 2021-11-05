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
	PKTemplate  = Template{OP_PUBKEY, OP_CHECKSIG}

	MultiPKHWrap = Script{OP_IF, OP_DUP, OP_HASH160, OP_PUBKEYHASH, OP_EQUALVERIFY,
		OP_CHECKSIGVERIFY, OP_FROMALTSTACK, OP_1ADD, OP_TOALTSTACK, OP_ENDIF}
)

// Template represents a locking script that is incomplete. It represents the function of the
// locking script without the public keys or other specific values needed to make it complete.
type Template Script

func NewMultiPKHTemplate(required, total uint32) (Template, error) {
	result := &bytes.Buffer{}

	// Push zero to alt stack to initialize the counter.
	if err := result.WriteByte(OP_0); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	if err := result.WriteByte(OP_TOALTSTACK); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	// Wrap each key in an if statement and a P2PKH locking script
	for i := uint32(0); i < total; i++ {
		if _, err := result.Write(MultiPKHWrap); err != nil {
			return nil, errors.Wrap(err, "write")
		}
	}

	// From https://github.com/bitcoin-sv/bitcoin-sv/blob/master/src/script/interpreter.cpp#L1144
	//
	// const auto& arg_2 = stack.stacktop(-2);
	// const auto& arg_1 = stack.stacktop(-1);

	// CScriptNum bn1(arg_2.GetElement(), fRequireMinimal,
	//                maxScriptNumLength,
	//                utxo_after_genesis);
	// CScriptNum bn2(arg_1.GetElement(), fRequireMinimal,
	//                maxScriptNumLength,
	//                utxo_after_genesis);
	//
	// ...
	//
	// case OP_LESSTHANOREQUAL:
	//     bn = (bn1 <= bn2);
	//     break;
	// case OP_GREATERTHANOREQUAL:
	//     bn = (bn1 >= bn2);
	//     break;

	// To make it confusing they switch values so arg_2 goes into bn1 and arg_1 goes into bn2.
	// bn2 is the top stack item
	// bn1 is the next stack item (under top)
	//
	// Therefore "{Required Signature Count} OP_FROMALTSTACK OP_LESSTHANOREQUAL" equates to:
	//   {Required Signature Count} <= OP_FROMALTSTACK
	//
	// OP_FROMALTSTACK gets the the accumulated count of valid signatures from the alt stack so we
	// want it to be greater than or equal to the required signature count. In other words we want
	// the required signature count to be less than or equal to OP_FROMALTSTACK.

	// "OP_FROMALTSTACK {Required Signature Count} OP_GREATERTHANOREQUAL" would also work, but
	// doesn't seem to be most common and it is preferable to match with other implementations.

	if _, err := result.Write(PushNumberScript(int64(required))); err != nil {
		return nil, errors.Wrap(err, "write")
	}

	if err := result.WriteByte(OP_FROMALTSTACK); err != nil {
		return nil, errors.Wrap(err, "write byte")
	}

	if err := result.WriteByte(OP_LESSTHANOREQUAL); err != nil {
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
		item, err := ParseScript(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrap(err, "parse script")
		}

		if item.Type == ScriptItemTypePushData {
			if err := WritePushDataScript(result, item.Data); err != nil {
				return nil, errors.Wrap(err, "write push data")
			}
			continue
		}

		switch item.OpCode {
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
		if err := result.WriteByte(item.OpCode); err != nil {
			return nil, errors.Wrap(err, "write op code")
		}
	}

	return NewScript(result.Bytes()), nil
}

func (t Template) PubKeyCount() uint32 {
	var result uint32
	for _, b := range t {
		if b == OP_PUBKEY || b == OP_PUBKEYHASH {
			result++
		}
	}
	return result
}

// RequiredSignatures is the number of signatures required to unlock the template.
// Note: Only supports PKH, PK, and MultiPKH.
func (t Template) RequiredSignatures() (uint32, error) {
	if bytes.Equal(t, PKHTemplate) || bytes.Equal(t, PKTemplate) {
		return 1, nil
	}

	// Assume this is a multi-pkh accumulator script.
	buf := bytes.NewReader(t)
	var previousItems []*ScriptItem
	for {
		item, err := ParseScript(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, errors.Wrap(err, "parse")
		}

		if item.OpCode == OP_LESSTHANOREQUAL {
			break
		}

		// Save last two items
		previousItems = append(previousItems, item)
		if len(previousItems) > 2 {
			previousItems = previousItems[1:]
		}
	}

	if len(previousItems) != 2 {
		return 0, errors.Wrap(ErrUnknownScriptTemplate, "not enough items")
	}

	if previousItems[1].Type != ScriptItemTypeOpCode || previousItems[1].OpCode != OP_FROMALTSTACK {
		return 0, errors.Wrap(ErrUnknownScriptTemplate, "not OP_FROMALTSTACK")
	}

	requiredSigners, err := ScriptNumberValue(previousItems[0])
	if err != nil {
		return 0, errors.Wrap(err, "script number")
	}

	if requiredSigners < 1 || requiredSigners > 0xffffffff {
		return 0, errors.Wrapf(ErrUnknownScriptTemplate, "require signer value %d", requiredSigners)
	}

	return uint32(requiredSigners), nil
}

func (t Template) String() string {
	return ScriptToString(Script(t))
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

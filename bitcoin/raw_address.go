package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

const (
	ScriptTypeEmpty    = 0xff // Empty address
	ScriptTypePKH      = 0x20 // Public Key Hash
	ScriptTypeSH       = 0x21 // Script Hash
	ScriptTypeMultiPKH = 0x22 // Multi-PKH
	ScriptTypeRPH      = 0x23 // RPH
	ScriptTypePK       = 0x24 // Public Key

	ScriptTypeNonStandard = 0x25 // Unknown, but possibly spendable locking script

	ScriptHashLength = 20 // Length of standard public key, script, and R hashes RIPEMD(SHA256())
)

// RawAddress represents a bitcoin address in raw format, with no check sum or encoding.
// It represents a "script template" for common locking and unlocking scripts.
// It enables parsing and creating of common locking and unlocking scripts as well as identifying
//   participants involved in the scripts via public key hashes and other hashes.
type RawAddress struct {
	scriptType byte
	data       []byte
}

// DecodeRawAddress decodes a binary raw address. It returns an error if there was an issue.
func DecodeRawAddress(b []byte) (RawAddress, error) {
	var result RawAddress
	err := result.Decode(b)
	return result, err
}

// Decode decodes a binary raw address. It returns an error if there was an issue.
func (ra *RawAddress) Decode(b []byte) error {
	if len(b) == 0 {
		return errors.Wrap(ErrBadType, "empty")
	}

	switch b[0] {
	case ScriptTypeEmpty:
		ra.scriptType = ScriptTypeEmpty
		ra.data = nil
		return nil

	// Public Key Hash
	case AddressTypeMainPKH:
		fallthrough
	case AddressTypeTestPKH:
		fallthrough
	case ScriptTypePKH:
		return ra.SetPKH(b[1:])

	// Public Key
	case AddressTypeMainPK:
		fallthrough
	case AddressTypeTestPK:
		fallthrough
	case ScriptTypePK:
		return ra.SetCompressedPublicKey(b[1:])

	// Script Hash
	case AddressTypeMainSH:
		fallthrough
	case AddressTypeTestSH:
		fallthrough
	case ScriptTypeSH:
		return ra.SetSH(b[1:])

	// Multiple Public Key Hash
	case AddressTypeMainMultiPKH:
		fallthrough
	case AddressTypeTestMultiPKH:
		fallthrough
	case ScriptTypeMultiPKH:
		ra.scriptType = ScriptTypeMultiPKH
		ra.data = b[1:]

		// Validate data
		b = b[1:] // remove type
		// Parse required count
		buf := bytes.NewBuffer(b)
		var required uint64
		var err error
		if required, err = ReadBase128VarInt(buf); err != nil {
			return err
		}
		// Parse hash count
		var count uint64
		if count, err = ReadBase128VarInt(buf); err != nil {
			return err
		}
		pkhs := make([][]byte, 0, count)
		for i := uint64(0); i < count; i++ {
			pkh := make([]byte, ScriptHashLength)
			if _, err := buf.Read(pkh); err != nil {
				return err
			}
			pkhs = append(pkhs, pkh)
		}
		return ra.SetMultiPKH(int(required), pkhs)

	// R Puzzle Hash
	case AddressTypeMainRPH:
		fallthrough
	case AddressTypeTestRPH:
		fallthrough
	case ScriptTypeRPH:
		return ra.SetRPH(b[1:])

	// Non Standard
	case AddressTypeMainNonStandard:
		fallthrough
	case AddressTypeTestNonStandard:
		fallthrough
	case ScriptTypeNonStandard:
		return ra.SetNonStandard(b[1:])
	}

	return ErrBadType
}

// Deserialize reads a binary raw address. It returns an error if there was an issue.
func (ra *RawAddress) Deserialize(r io.Reader) error {
	var t [1]byte
	if _, err := io.ReadFull(r, t[:]); err != nil {
		return err
	}

	switch t[0] {
	case ScriptTypeEmpty:
		ra.scriptType = ScriptTypeEmpty
		ra.data = nil
		return nil

	// Public Key Hash
	case AddressTypeMainPKH:
		fallthrough
	case AddressTypeTestPKH:
		fallthrough
	case ScriptTypePKH:
		pkh := make([]byte, ScriptHashLength)
		if _, err := io.ReadFull(r, pkh); err != nil {
			return err
		}
		return ra.SetPKH(pkh)

	// Public Key
	case AddressTypeMainPK:
		fallthrough
	case AddressTypeTestPK:
		fallthrough
	case ScriptTypePK:
		pk := make([]byte, PublicKeyCompressedLength)
		if _, err := io.ReadFull(r, pk); err != nil {
			return err
		}
		return ra.SetCompressedPublicKey(pk)

	// Script Hash
	case AddressTypeMainSH:
		fallthrough
	case AddressTypeTestSH:
		fallthrough
	case ScriptTypeSH:
		sh := make([]byte, ScriptHashLength)
		if _, err := io.ReadFull(r, sh); err != nil {
			return err
		}
		return ra.SetSH(sh)

	// Multiple Public Key Hash
	case AddressTypeMainMultiPKH:
		fallthrough
	case AddressTypeTestMultiPKH:
		fallthrough
	case ScriptTypeMultiPKH:
		// Parse required count
		var required uint64
		var err error
		if required, err = ReadBase128VarInt(r); err != nil {
			return err
		}
		// Parse hash count
		var count uint64
		if count, err = ReadBase128VarInt(r); err != nil {
			return err
		}
		pkhs := make([][]byte, 0, count)
		for i := uint64(0); i < count; i++ {
			pkh := make([]byte, ScriptHashLength)
			if _, err := io.ReadFull(r, pkh); err != nil {
				return err
			}
			pkhs = append(pkhs, pkh)
		}
		return ra.SetMultiPKH(int(required), pkhs)

	// R Puzzle Hash
	case AddressTypeMainRPH:
		fallthrough
	case AddressTypeTestRPH:
		fallthrough
	case ScriptTypeRPH:
		rph := make([]byte, ScriptHashLength)
		if _, err := io.ReadFull(r, rph); err != nil {
			return err
		}
		return ra.SetRPH(rph)
	}

	return errors.Wrapf(ErrBadType, "Type : %d", t)
}

// NewRawAddressFromAddress creates a RawAddress from an Address.
func NewRawAddressFromAddress(a Address) RawAddress {
	result := RawAddress{data: a.data}

	switch a.addressType {
	case AddressTypeMainPKH:
		fallthrough
	case AddressTypeTestPKH:
		result.scriptType = ScriptTypePKH
	case AddressTypeMainPK:
		fallthrough
	case AddressTypeTestPK:
		result.scriptType = ScriptTypePK
	case AddressTypeMainSH:
		fallthrough
	case AddressTypeTestSH:
		result.scriptType = ScriptTypeSH
	case AddressTypeMainMultiPKH:
		fallthrough
	case AddressTypeTestMultiPKH:
		result.scriptType = ScriptTypeMultiPKH
	case AddressTypeMainRPH:
		fallthrough
	case AddressTypeTestRPH:
		result.scriptType = ScriptTypeRPH
	}

	return result
}

/****************************************** PKH ***************************************************/

// NewRawAddressPKH creates an address from a public key hash.
func NewRawAddressPKH(pkh []byte) (RawAddress, error) {
	var result RawAddress
	err := result.SetPKH(pkh)
	return result, err
}

// SetPKH sets the type as ScriptTypePKH and sets the data to the specified Public Key Hash.
func (ra *RawAddress) SetPKH(pkh []byte) error {
	if len(pkh) != ScriptHashLength {
		return ErrBadScriptHashLength
	}

	ra.scriptType = ScriptTypePKH
	ra.data = pkh
	return nil
}

func (ra *RawAddress) GetPublicKeyHash() (Hash20, error) {
	if ra.scriptType != ScriptTypePKH {
		return Hash20{}, ErrWrongType
	}

	hash, err := NewHash20(ra.data)
	return *hash, err
}

/****************************************** PK ***************************************************/

// NewRawAddressPublicKey creates an address from a public key.
func NewRawAddressPublicKey(pk PublicKey) (RawAddress, error) {
	var result RawAddress
	err := result.SetPublicKey(pk)
	return result, err
}

// SetPublicKey sets the type as ScriptTypePKH and sets the data to the specified public key.
func (ra *RawAddress) SetPublicKey(pk PublicKey) error {
	ra.scriptType = ScriptTypePK
	ra.data = pk.Bytes()
	return nil
}

// NewRawAddressCompressedPublicKey creates an address from a compressed public key.
func NewRawAddressCompressedPublicKey(pk []byte) (RawAddress, error) {
	var result RawAddress
	err := result.SetCompressedPublicKey(pk)
	return result, err
}

// SetCompressedPublicKey sets the type as ScriptTypePKH and sets the data to the specified
//   compressed public key.
func (ra *RawAddress) SetCompressedPublicKey(pk []byte) error {
	if len(pk) != PublicKeyCompressedLength {
		return ErrBadScriptHashLength
	}

	ra.scriptType = ScriptTypePK
	ra.data = pk
	return nil
}

func (ra *RawAddress) GetPublicKey() (PublicKey, error) {
	if ra.scriptType != ScriptTypePK {
		return PublicKey{}, ErrWrongType
	}

	return PublicKeyFromBytes(ra.data)
}

/******************************************* SH ***************************************************/

// NewRawAddressSH creates an address from a script hash.
func NewRawAddressSH(sh []byte) (RawAddress, error) {
	var result RawAddress
	err := result.SetSH(sh)
	return result, err
}

// SetSH sets the type as ScriptTypeSH and sets the data to the specified Script Hash.
func (ra *RawAddress) SetSH(sh []byte) error {
	if len(sh) != ScriptHashLength {
		return ErrBadScriptHashLength
	}

	ra.scriptType = ScriptTypeSH
	ra.data = sh
	return nil
}

/**************************************** MultiPKH ************************************************/

// NewRawAddressMultiPKH creates an address from multiple public key hashes.
func NewRawAddressMultiPKH(required int, pkhs [][]byte) (RawAddress, error) {
	var result RawAddress
	err := result.SetMultiPKH(required, pkhs)
	return result, err
}

// SetMultiPKH sets the type as ScriptTypeMultiPKH and puts the required count and Public Key Hashes into data.
func (ra *RawAddress) SetMultiPKH(required int, pkhs [][]byte) error {
	ra.scriptType = ScriptTypeMultiPKH
	buf := bytes.NewBuffer(make([]byte, 0, 4+(len(pkhs)*ScriptHashLength)))

	if err := WriteBase128VarInt(buf, uint64(required)); err != nil {
		return err
	}
	if err := WriteBase128VarInt(buf, uint64(len(pkhs))); err != nil {
		return err
	}
	for _, pkh := range pkhs {
		n, err := buf.Write(pkh)
		if err != nil {
			return err
		}
		if n != ScriptHashLength {
			return ErrBadScriptHashLength
		}
	}
	ra.data = buf.Bytes()
	return nil
}

// GetMultiPKH returns all of the hashes from a ScriptTypeMultiPKH address.
func (ra *RawAddress) GetMultiPKH() ([][]byte, error) {
	if ra.scriptType != ScriptTypeMultiPKH {
		return nil, ErrBadType
	}

	buf := bytes.NewBuffer(ra.data)
	var err error

	// Parse required count
	if _, err = ReadBase128VarInt(buf); err != nil {
		return nil, err
	}
	// Parse hash count
	var count uint64
	if count, err = ReadBase128VarInt(buf); err != nil {
		return nil, err
	}
	pkhs := make([][]byte, 0, count)
	for i := uint64(0); i < count; i++ {
		pkh := make([]byte, ScriptHashLength)
		if _, err := buf.Read(pkh); err != nil {
			return nil, err
		}
		pkhs = append(pkhs, pkh)
	}

	return pkhs, nil
}

/******************************************** RPH *************************************************/

// NewRawAddressRPH creates an address from a R puzzle hash.
func NewRawAddressRPH(rph []byte) (RawAddress, error) {
	var result RawAddress
	err := result.SetRPH(rph)
	return result, err
}

// SetRPH sets the type as ScriptTypeRPH and sets the data to the specified R Puzzle Hash.
func (ra *RawAddress) SetRPH(rph []byte) error {
	if len(rph) != ScriptHashLength {
		return ErrBadScriptHashLength
	}
	ra.scriptType = ScriptTypeRPH
	ra.data = rph
	return nil
}

/**************************************** Non-Standard ********************************************/

// NewRawAddressNonStandard creates an address from a non-standard but possibly spendable script.
func NewRawAddressNonStandard(script []byte) (RawAddress, error) {
	var result RawAddress
	err := result.SetNonStandard(script)
	return result, err
}

// SetNonStandard sets the type as ScriptTypeNonStandard and sets the data to the specified script.
func (ra *RawAddress) SetNonStandard(script []byte) error {
	ra.scriptType = ScriptTypeNonStandard
	ra.data = script
	return nil
}

/***************************************** Common *************************************************/

// Type returns the script type of the address.
func (ra RawAddress) Type() byte {
	return ra.scriptType
}

// IsSpendable returns true if the address produces a locking script that can be unlocked.
func (ra RawAddress) IsSpendable() bool {
	// TODO Full locking and unlocking support only available for P2PKH.
	return !ra.IsEmpty() && (ra.scriptType == ScriptTypePKH)
}

// IsNonStandard returns true if the address represents a script that is possibly spendable, but
// not one of the standard (known) locking scripts.
func (ra RawAddress) IsNonStandard() bool {
	return !ra.IsEmpty() && (ra.scriptType == ScriptTypeNonStandard)
}

// Bytes returns the byte encoded format of the address.
func (ra RawAddress) Bytes() []byte {
	if len(ra.data) == 0 {
		return nil
	}
	return append([]byte{ra.scriptType}, ra.data...)
}

func (ra RawAddress) Equal(other RawAddress) bool {
	return ra.scriptType == other.scriptType && bytes.Equal(ra.data, other.data)
}

func (ra RawAddress) Copy() RawAddress {
	result := RawAddress{
		scriptType: ra.scriptType,
		data:       make([]byte, len(ra.data)),
	}
	copy(result.data, ra.data)
	return result
}

// IsEmpty returns true if the address does not have a value set.
func (ra RawAddress) IsEmpty() bool {
	return len(ra.data) == 0
}

func (ra RawAddress) Serialize(w io.Writer) error {
	if ra.IsEmpty() {
		if _, err := w.Write([]byte{ScriptTypeEmpty}); err != nil {
			return err
		}
	}

	if _, err := w.Write([]byte{ra.scriptType}); err != nil {
		return err
	}
	if _, err := w.Write(ra.data); err != nil {
		return err
	}
	return nil
}

// Hash returns the hash corresponding to the address.
func (ra *RawAddress) Hash() (*Hash20, error) {
	switch ra.scriptType {
	case ScriptTypePKH:
		return NewHash20(ra.data)
	case ScriptTypeSH:
		return NewHash20(ra.data)
	case ScriptTypePK:
		return NewHash20(Hash160(ra.data))
	case ScriptTypeMultiPKH:
		return NewHash20(Hash160(ra.data))
	case ScriptTypeRPH:
		return NewHash20(ra.data)
	case ScriptTypeNonStandard:
		return NewHash20(Hash160(ra.data))
	}
	return nil, ErrUnknownScriptTemplate
}

// Hashes returns the hashes corresponding to the address. Including the all PKHs in a MultiPKH.
func (ra *RawAddress) Hashes() ([]Hash20, error) {

	switch ra.scriptType {
	case ScriptTypePKH:
		fallthrough
	case ScriptTypeSH:
		fallthrough
	case ScriptTypeRPH:
		hash, err := NewHash20(ra.data)
		if err != nil {
			return nil, err
		}
		return []Hash20{*hash}, nil

	case ScriptTypePK:
		hash, err := NewHash20(Hash160(ra.data))
		if err != nil {
			return nil, err
		}
		return []Hash20{*hash}, nil

	case ScriptTypeMultiPKH:
		pkhs, err := ra.GetMultiPKH()
		if err != nil {
			return nil, err
		}
		result := make([]Hash20, 0, len(pkhs))
		for _, pkh := range pkhs {
			hash, err := NewHash20(pkh)
			if err != nil {
				return nil, err
			}
			result = append(result, *hash)
		}
		return result, nil

	case ScriptTypeNonStandard:
		return PKHsFromLockingScript(ra.data)
	}

	return nil, ErrUnknownScriptTemplate
}

// MarshalJSON converts to json.
func (ra RawAddress) MarshalJSON() ([]byte, error) {
	if len(ra.data) == 0 {
		return []byte("\"\""), nil
	}
	return []byte("\"" + hex.EncodeToString(ra.Bytes()) + "\""), nil
}

// UnmarshalJSON converts from json.
func (ra *RawAddress) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("Too short for RawAddress hex data : %d", len(data))
	}

	if len(data) == 2 {
		// Empty raw address
		ra.scriptType = 0
		ra.data = nil
		return nil
	}

	// Decode hex and remove double quotes.
	raw, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}

	// Decode into raw address
	return ra.Decode(raw)
}

// MarshalText returns the text encoding of the raw address.
// Implements encoding.TextMarshaler interface.
func (ra RawAddress) MarshalText() ([]byte, error) {
	b := ra.Bytes()
	result := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(result, b)
	return result, nil
}

// UnmarshalText parses a text encoded raw address and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (ra *RawAddress) UnmarshalText(text []byte) error {
	b := make([]byte, hex.DecodedLen(len(text)))
	_, err := hex.Decode(b, text)
	if err != nil {
		return err
	}

	return ra.Decode(b)
}

// MarshalBinary returns the binary encoding of the raw address.
// Implements encoding.BinaryMarshaler interface.
func (ra RawAddress) MarshalBinary() ([]byte, error) {
	return ra.Bytes(), nil
}

// UnmarshalBinary parses a binary encoded raw address and sets the value of this object.
// Implements encoding.BinaryUnmarshaler interface.
func (ra *RawAddress) UnmarshalBinary(data []byte) error {
	return ra.Decode(data)
}

// Scan converts from a database column.
func (ra *RawAddress) Scan(data interface{}) error {
	if data == nil {
		// Empty raw address
		ra.scriptType = 0
		ra.data = nil
		return nil
	}

	b, ok := data.([]byte)
	if !ok {
		return errors.New("RawAddress db column not bytes")
	}

	if len(b) == 0 {
		// Empty raw address
		ra.scriptType = 0
		ra.data = nil
		return nil
	}

	// Copy byte slice because it will be wiped out by the database after this call.
	c := make([]byte, len(b))
	copy(c, b)

	// Decode into raw address
	return ra.Decode(c)
}

package bitcoin

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math/big"

	"github.com/pkg/errors"
)

// Signature is an elliptic curve signature using the secp256k1 elliptic curve.
type Signature struct {
	R big.Int
	S big.Int
}

// Verify returns true if the signature is valid for this public key and hash.
func (s Signature) Verify(hash Hash32, pubkey PublicKey) bool {
	ecPubKey := &ecdsa.PublicKey{
		curveS256,
		&pubkey.X,
		&pubkey.Y,
	}
	return ecdsa.Verify(ecPubKey, hash[:], &s.R, &s.S)
}

// SignatureFromStr converts signature text to a signature.
func SignatureFromStr(s string) (Signature, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return Signature{}, err
	}

	return SignatureFromBytes(b)
}

// SignatureFromBytes decodes a binary bitcoin signature. It returns the signature and an error if
//   there was an issue.
func SignatureFromBytes(b []byte) (Signature, error) {
	// 0x30 <length of whole message> <0x02> <length of R> <R> 0x2
	// <length of S> <S>.

	if len(b) < 8 {
		return Signature{}, errors.New("Signature too short")
	}
	// 0x30
	index := 0
	if b[index] != 0x30 {
		return Signature{}, errors.New("Signature missing header byte")
	}
	index++
	// length of remaining message
	length := b[index]
	index++

	// length should be less than the entire message and greater than
	// the minimal message size.
	if int(length+2) > len(b) || int(length+2) < 8 {
		return Signature{}, errors.New("Signature has bad length")
	}
	// trim the slice we're working on so we only look at what matters.
	b = b[:length+2]

	// 0x02
	if b[index] != 0x02 {
		return Signature{}, errors.New("Signature missing 1st int marker")
	}
	index++

	// Length of R.
	rLen := int(b[index])
	// must be positive, must be able to fit in another 0x2, <len> <s>
	// hence the -3. We assume that the length must be at least one byte.
	index++
	if rLen <= 0 || rLen > len(b)-index-3 {
		return Signature{}, errors.New("Signature has bad R length")
	}

	// Then R itself.
	rBytes := b[index : index+rLen]
	switch err := canonicalPadding(rBytes); err {
	case errNegativeValue:
		return Signature{}, errors.New("Signature R is negative")
	case errExcessivelyPaddedValue:
		return Signature{}, errors.New("Signature R is excessively padded")
	}
	var r big.Int
	r.SetBytes(rBytes)
	index += rLen
	// 0x02. length already checked in previous if.
	if b[index] != 0x02 {
		return Signature{}, errors.New("malformed signature: no 2nd int marker")
	}
	index++

	// Length of signature S.
	sLen := int(b[index])
	index++
	// S should be the rest of the string.
	if sLen <= 0 || sLen > len(b)-index {
		return Signature{}, errors.New("Signature has bad S length")
	}

	// Then S itself.
	sBytes := b[index : index+sLen]
	switch err := canonicalPadding(sBytes); err {
	case errNegativeValue:
		return Signature{}, errors.New("Signature S is negative")
	case errExcessivelyPaddedValue:
		return Signature{}, errors.New("Signature S is excessively padded")
	}
	var s big.Int
	s.SetBytes(sBytes)
	index += sLen

	// sanity check length parsing
	if index != len(b) {
		return Signature{}, fmt.Errorf("Signature has bad final length %v != %v", index, len(b))
	}

	result := Signature{R: r, S: s}
	return result, result.Validate()
}

// SignatureFromCompact converts base64 "compact" signature text to a signature.
// This is apparently a signature format used in old software to sign text messages.
func SignatureFromCompact(s string) (Signature, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Signature{}, errors.Wrap(err, "base64 decode")
	}

	if len(b) != 65 {
		return Signature{}, fmt.Errorf("Wrong length : %d should be 65", len(b))
	}

	recovery := int(b[0]) - 27 - 4
	if recovery < 0 {
		recovery += 4
	}

	if recovery < 0 || recovery > 3 {
		return Signature{}, fmt.Errorf("Invalid recover value : %d should be (0 >= r <= 3)", recovery)
	}

	var result Signature
	result.R.SetBytes(b[1:33])
	result.S.SetBytes(b[33:])

	return result, result.Validate()
}

func (s Signature) Serialize(w io.Writer) error {
	// low 'S' malleability breaker
	sigS := s.S
	if sigS.Cmp(curveHalfOrder) == 1 {
		sigS.Sub(curveS256.N, &sigS)
	}
	// Ensure the encoded bytes for the r and s values are canonical and
	// thus suitable for DER encoding.
	rb := canonicalizeInt(s.R)
	sb := canonicalizeInt(sigS)

	// total length of returned signature is 1 byte for each magic and
	// length (4 total), plus lengths of r and s
	length := 4 + len(rb) + len(sb)

	// Header
	b := make([]byte, 4)
	b[0] = 0x30
	b[1] = byte(length)

	// Write R
	b[2] = 0x02
	b[3] = byte(len(rb))
	if _, err := w.Write(b); err != nil {
		return errors.Wrap(err, "header")
	}
	if _, err := w.Write(rb); err != nil {
		return errors.Wrap(err, "r")
	}

	// Write S
	b = make([]byte, 2)
	b[0] = 0x02
	b[1] = byte(len(sb))
	if _, err := w.Write(b); err != nil {
		return errors.Wrap(err, "header")
	}
	if _, err := w.Write(sb); err != nil {
		return errors.Wrap(err, "s")
	}

	return nil
}

func (s *Signature) Deserialize(r io.Reader) error {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "header byte")
	}

	// 0x30
	if b[0] != 0x30 {
		return errors.New("Signature missing header byte")
	}

	// length of remaining message
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "length")
	}

	b = make([]byte, b[0])
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "data")
	}

	// 0x02
	index := 0
	if b[index] != 0x02 {
		return errors.New("Signature missing 1st int marker")
	}
	index++

	// Length of R.
	rLen := int(b[index])
	// must be positive, must be able to fit in another 0x2, <len> <s>
	// hence the -3. We assume that the length must be at least one byte.
	index++
	if rLen <= 0 || rLen > len(b)-index-3 {
		return errors.New("Signature has bad R length")
	}

	// Then R itself.
	rBytes := b[index : index+rLen]
	switch err := canonicalPadding(rBytes); err {
	case errNegativeValue:
		return errors.New("Signature R is negative")
	case errExcessivelyPaddedValue:
		return errors.New("Signature R is excessively padded")
	}

	s.R.SetBytes(rBytes)
	index += rLen
	// 0x02. length already checked in previous if.
	if b[index] != 0x02 {
		return errors.New("malformed signature: no 2nd int marker")
	}
	index++

	// Length of signature S.
	sLen := int(b[index])
	index++
	// S should be the rest of the string.
	if sLen <= 0 || sLen > len(b)-index {
		return errors.New("Signature has bad S length")
	}

	// Then S itself.
	sBytes := b[index : index+sLen]
	switch err := canonicalPadding(sBytes); err {
	case errNegativeValue:
		return errors.New("Signature S is negative")
	case errExcessivelyPaddedValue:
		return errors.New("Signature S is excessively padded")
	}

	s.S.SetBytes(sBytes)
	index += sLen

	// sanity check length parsing
	if index != len(b) {
		return fmt.Errorf("Signature has bad final length %v != %v", index, len(b))
	}

	return s.Validate()
}

// Validate checks the mathematical validity of the signature's R and S values.
func (s Signature) Validate() error {
	// FWIW the ecdsa spec states that R and S must be | 1, N - 1 |
	// but crypto/ecdsa only checks for Sign != 0. Mirror that.
	if s.R.Sign() != 1 {
		return errors.New("signature R isn't 1 or more")
	}
	if s.S.Sign() != 1 {
		return errors.New("signature S isn't 1 or more")
	}
	if s.R.Cmp(curveS256Params.N) >= 0 {
		return errors.New("signature R is >= curve.N")
	}
	if s.S.Cmp(curveS256Params.N) >= 0 {
		return errors.New("signature S is >= curve.N")
	}

	return nil
}

// ToCompact converts a signature to a base64 "compact" signature encoding.
func (s Signature) ToCompact() string {
	b := make([]byte, 0, 65)
	b = append(b, byte(27+4+1)) // recovery=1, not sure how to choose recovery (0-3) --ce

	rb := s.R.Bytes()
	if len(rb) < 32 {
		extra := make([]byte, 32-len(rb))
		rb = append(extra, rb...)
	}
	b = append(b, rb...)

	sb := s.S.Bytes()
	if len(sb) < 32 {
		extra := make([]byte, 32-len(sb))
		sb = append(extra, sb...)
	}
	b = append(b, sb...)

	return base64.StdEncoding.EncodeToString(b)
}

// String returns the signature data with a checksum, encoded with Base58.
func (s Signature) String() string {
	return hex.EncodeToString(s.Bytes())
}

// SetString decodes a signature from hex text.
func (s *Signature) SetString(str string) error {
	ns, err := SignatureFromStr(str)
	if err != nil {
		return err
	}

	*s = ns
	return nil
}

// SetBytes decodes the signature from bytes.
func (s *Signature) SetBytes(b []byte) error {
	ns, err := SignatureFromBytes(b)
	if err != nil {
		return err
	}

	*s = ns
	return nil
}

// Bytes returns serialized compressed key data.
func (s Signature) Bytes() []byte {
	// low 'S' malleability breaker
	sigS := s.S
	if sigS.Cmp(curveHalfOrder) == 1 {
		sigS.Sub(curveS256.N, &sigS)
	}
	// Ensure the encoded bytes for the r and s values are canonical and
	// thus suitable for DER encoding.
	rb := canonicalizeInt(s.R)
	sb := canonicalizeInt(sigS)

	// total length of returned signature is 1 byte for each magic and
	// length (6 total), plus lengths of r and s
	length := 6 + len(rb) + len(sb)
	b := make([]byte, length)

	b[0] = 0x30
	b[1] = byte(length - 2)
	b[2] = 0x02
	b[3] = byte(len(rb))
	offset := copy(b[4:], rb) + 4
	b[offset] = 0x02
	b[offset+1] = byte(len(sb))
	copy(b[offset+2:], sb)
	return b
}

// MarshalJSON converts to json.
func (s Signature) MarshalJSON() ([]byte, error) {
	return []byte("\"" + s.String() + "\""), nil
}

// UnmarshalJSON converts from json.
func (s *Signature) UnmarshalJSON(data []byte) error {
	return s.SetString(string(data[1 : len(data)-1]))
}

// MarshalText returns the text encoding of the signature.
// Implements encoding.TextMarshaler interface.
func (s Signature) MarshalText() ([]byte, error) {
	b := s.Bytes()
	result := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(result, b)
	return result, nil
}

// UnmarshalText parses a text encoded signature and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (s *Signature) UnmarshalText(text []byte) error {
	b := make([]byte, hex.DecodedLen(len(text)))
	_, err := hex.Decode(b, text)
	if err != nil {
		return err
	}

	return s.SetBytes(b)
}

// MarshalBinary returns the binary encoding of the signature.
// Implements encoding.BinaryMarshaler interface.
func (s Signature) MarshalBinary() ([]byte, error) {
	return s.Bytes(), nil
}

// UnmarshalBinary parses a binary encoded signature and sets the value of this object.
// Implements encoding.BinaryUnmarshaler interface.
func (s *Signature) UnmarshalBinary(data []byte) error {
	return s.SetBytes(data)
}

// Scan converts from a database column.
func (s *Signature) Scan(data interface{}) error {
	b, ok := data.([]byte)
	if !ok {
		return errors.New("Public Key db column not bytes")
	}

	c := make([]byte, len(b))
	copy(c, b)
	return s.SetBytes(c)
}

func (s Signature) Equal(o Signature) bool {
	return s.S.Cmp(&o.S) == 0 && s.R.Cmp(&o.R) == 0
}

/********************************************* RFC6979 ********************************************/
var (
	// Used in RFC6979 implementation when testing the nonce for correctness
	one = big.NewInt(1)

	// oneInitializer is used to fill a byte slice with byte 0x01.  It is provided
	// here to avoid the need to create it multiple times.
	oneInitializer = []byte{0x01}

	// Errors returned by canonicalPadding.
	errNegativeValue          = errors.New("value may be interpreted as negative")
	errExcessivelyPaddedValue = errors.New("value is excessively padded")
)

// signRFC6979 generates a deterministic ECDSA signature according to RFC 6979 and BIP 62.
func signRFC6979(pk big.Int, hash []byte) (Signature, error) {
	N := curveS256.N
	k := nonceRFC6979(pk, hash)
	inv := new(big.Int).ModInverse(k, N)
	r, _ := curveS256.ScalarBaseMult(k.Bytes())
	r.Mod(r, N)

	if r.Sign() == 0 {
		return Signature{}, errors.New("calculated R is zero")
	}

	e := hashToInt(hash, curveS256)
	s := new(big.Int).Mul(&pk, r)
	s.Add(s, e)
	s.Mul(s, inv)
	s.Mod(s, N)

	if s.Cmp(curveHalfOrder) == 1 {
		s.Sub(N, s)
	}
	if s.Sign() == 0 {
		return Signature{}, errors.New("calculated S is zero")
	}
	return Signature{R: *r, S: *s}, nil
}

// nonceRFC6979 generates an ECDSA nonce (`k`) deterministically according to RFC 6979.
// It takes a 32-byte hash as an input and returns 32-byte nonce to be used in ECDSA algorithm.
func nonceRFC6979(pk big.Int, hash []byte) *big.Int {
	q := curveS256Params.N
	alg := sha256.New

	qlen := q.BitLen()
	holen := alg().Size()
	rolen := (qlen + 7) >> 3
	bx := append(int2octets(pk, rolen), bits2octets(hash, curveS256, rolen)...)

	// Step B
	v := bytes.Repeat(oneInitializer, holen)

	// Step C (Go zeroes the all allocated memory)
	k := make([]byte, holen)

	// Step D
	k = mac(alg, k, append(append(v, 0x00), bx...))

	// Step E
	v = mac(alg, k, v)

	// Step F
	k = mac(alg, k, append(append(v, 0x01), bx...))

	// Step G
	v = mac(alg, k, v)

	// Step H
	for {
		// Step H1
		var t []byte

		// Step H2
		for len(t)*8 < qlen {
			v = mac(alg, k, v)
			t = append(t, v...)
		}

		// Step H3
		secret := hashToInt(t, curveS256)
		if secret.Cmp(one) >= 0 && secret.Cmp(q) < 0 {
			return secret
		}
		k = mac(alg, k, append(v, 0x00))
		v = mac(alg, k, v)
	}
}

// mac returns an HMAC of the given key and message.
func mac(alg func() hash.Hash, k, m []byte) []byte {
	h := hmac.New(alg, k)
	h.Write(m)
	return h.Sum(nil)
}

// https://tools.ietf.org/html/rfc6979#section-2.3.3
func int2octets(v big.Int, rolen int) []byte {
	out := v.Bytes()

	// left pad with zeros if it's too short
	if len(out) < rolen {
		out2 := make([]byte, rolen)
		copy(out2[rolen-len(out):], out)
		return out2
	}

	// drop most significant bytes if it's too long
	if len(out) > rolen {
		out2 := make([]byte, rolen)
		copy(out2, out[len(out)-rolen:])
		return out2
	}

	return out
}

// https://tools.ietf.org/html/rfc6979#section-2.3.4
func bits2octets(in []byte, curve elliptic.Curve, rolen int) []byte {
	z1 := hashToInt(in, curve)
	z2 := new(big.Int).Sub(z1, curve.Params().N)
	if z2.Sign() < 0 {
		return int2octets(*z1, rolen)
	}
	return int2octets(*z2, rolen)
}

// canonicalizeInt returns the bytes for the passed big integer adjusted as
// necessary to ensure that a big-endian encoded integer can't possibly be
// misinterpreted as a negative number.  This can happen when the most
// significant bit is set, so it is padded by a leading zero byte in this case.
// Also, the returned bytes will have at least a single byte when the passed
// value is 0.  This is required for DER encoding.
func canonicalizeInt(val big.Int) []byte {
	b := val.Bytes()
	if len(b) == 0 {
		b = []byte{0x00}
	}
	if b[0]&0x80 != 0 {
		paddedBytes := make([]byte, len(b)+1)
		copy(paddedBytes[1:], b)
		b = paddedBytes
	}
	return b
}

// canonicalPadding checks whether a big-endian encoded integer could
// possibly be misinterpreted as a negative number (even though OpenSSL
// treats all numbers as unsigned), or if there is any unnecessary
// leading zero padding.
func canonicalPadding(b []byte) error {
	switch {
	case b[0]&0x80 == 0x80:
		return errNegativeValue
	case len(b) > 1 && b[0] == 0x00 && b[1]&0x80 != 0x80:
		return errExcessivelyPaddedValue
	default:
		return nil
	}
}

// hashToInt converts a hash value to an integer. There is some disagreement
// about how this is done. [NSA] suggests that this is done in the obvious
// manner, but [SECG] truncates the hash to the bit-length of the curve order
// first. We follow [SECG] because that's what OpenSSL does. Additionally,
// OpenSSL right shifts excess bits from the number if the hash is too large
// and we mirror that too.
// This is borrowed from crypto/ecdsa.
func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}

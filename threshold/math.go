package threshold

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
)

var (
	bigZero      = big.NewInt(0)
	bigOne       = big.NewInt(1)
	bigTwo       = big.NewInt(2)
	bigModulo, _ = big.NewInt(0).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
)

func RandomOrdinal() (big.Int, error) {
	return generateRandRange(*bigOne, *bigModulo)
}

// generateRandRange generates a cryptographically safe random big integer between min and max.
func generateRandRange(min, max big.Int) (big.Int, error) {
	var r big.Int
	r.Sub(&max, &min)

	result, err := rand.Int(rand.Reader, &r)
	if err != nil {
		return big.Int{}, errors.Wrap(err, "generate random")
	}

	return *result.Add(result, &min), nil
}

// BigPair is a pair of big numbers. It can be an elliptic curve point, or just two big numbers.
type BigPair struct {
	X, Y big.Int
}

func (bp *BigPair) Set(o BigPair) {
	bp.X.Set(&o.X)
	bp.Y.Set(&o.Y)
}

func (bp BigPair) Equal(o BigPair) bool {
	return bp.X.Cmp(&o.X) == 0 && bp.Y.Cmp(&o.Y) == 0
}

func (bp BigPair) String() string {
	return fmt.Sprintf("%s, %s", bp.X.String(), bp.Y.String())
}

// BigOrdPair is an ordinal big number and a pair of big numbers used as a container for passing
// values.
type BigOrdPair struct {
	Ord   big.Int
	Point BigPair
}

func (bop BigOrdPair) Equal(o BigOrdPair) bool {
	return bop.Ord.Cmp(&o.Ord) == 0 && bop.Point.Equal(o.Point)
}

// MultiplyByGenerator performs elliptic curve multiplication by the generator (base) point G.
func MultiplyByGenerator(v big.Int) BigPair {
	x, y := curveS256.ScalarBaseMult(v.Bytes())
	return BigPair{
		X: *x,
		Y: *y,
	}
}

// ModMultiply performs multiplication using modular arithmetic.
// Assumes the inputs are already within modulus.
func ModMultiply(a, b, mod big.Int) big.Int {
	var result big.Int

	if mod.Cmp(bigZero) == 0 {
		return *result.Mul(&a, &b)
	}

	var aCopy, bCopy, test big.Int
	aCopy.Set(&a) // Manual copy required to protect original value from caller function.
	bCopy.Set(&b) // Manual copy required to protect original value from caller function.
	for bCopy.Cmp(bigZero) > 0 {
		// If "b" is odd then add "a" to the result
		test.Mod(&bCopy, bigTwo)
		if test.Cmp(bigOne) == 0 {
			result.Add(&result, &aCopy)
			result.Mod(&result, &mod)
		}

		// Multiply "a" with 2
		aCopy.Mul(&aCopy, bigTwo)
		aCopy.Mod(&aCopy, &mod)

		// Divide "b" by 2
		bCopy.Div(&bCopy, bigTwo)
	}

	return result
}

// ModInverse returns the modular inverse using modular arithmetic.
// Assumes the inputs are already within modulus.
func ModInverse(a, mod big.Int) big.Int {
	var result big.Int
	result.ModInverse(&a, bigModulo)
	if mod.Cmp(bigZero) != 0 {
		result.Mod(&result, &mod)
	}
	return result
}

// ModDivide performs division using modular arithmetic.
// Assumes the inputs are already within modulus.
func ModDivide(a, b, mod big.Int) big.Int {
	var result big.Int
	result.ModInverse(&b, bigModulo)
	result.Mul(&a, &result)
	if mod.Cmp(bigZero) != 0 {
		result.Mod(&result, &mod)
	}
	return result
}

// ModAdd performs addition using modular arithmetic.
// Assumes the inputs are already within modulus.
func ModAdd(a, b, mod big.Int) big.Int {
	var result big.Int
	result.Add(&a, &b)
	if mod.Cmp(bigZero) != 0 {
		result.Mod(&result, &mod)
	}
	return result
}

// ModSubtract performs subtraction using modular arithmetic.
// Assumes the inputs are already within modulus.
func ModSubtract(a, b, mod big.Int) big.Int {
	if mod.Cmp(bigZero) == 0 || a.Cmp(&b) > 0 {
		var result big.Int
		return *result.Sub(&a, &b)
	}

	// Result would be negative so subtract the absolute value of the difference from the modulus
	// mod - (b - a)
	var result big.Int
	result.Sub(&b, &a)        // b - a
	result.Sub(&mod, &result) // mod - (b - a)
	return result
}

func WriteBigInt(i big.Int, buf *bytes.Buffer) error {
	b := i.Bytes()
	size := uint8(len(b))
	if err := binary.Write(buf, DefaultEndian, size); err != nil {
		return err
	}
	_, err := buf.Write(b)
	return err
}

func ReadBigInt(i *big.Int, buf *bytes.Reader) error {
	var size uint8
	if err := binary.Read(buf, DefaultEndian, &size); err != nil {
		return err
	}
	b := make([]byte, size)
	if _, err := buf.Read(b); err != nil {
		return err
	}
	i.SetBytes(b)
	return nil
}

func WriteBigPair(i BigPair, buf *bytes.Buffer) error {
	if err := WriteBigInt(i.X, buf); err != nil {
		return err
	}
	if err := WriteBigInt(i.Y, buf); err != nil {
		return err
	}
	return nil
}

func ReadBigPair(i *BigPair, buf *bytes.Reader) error {
	if err := ReadBigInt(&i.X, buf); err != nil {
		return err
	}
	if err := ReadBigInt(&i.Y, buf); err != nil {
		return err
	}
	return nil
}

func WriteString(s string, buf *bytes.Buffer) error {
	size := uint32(len(s))
	if err := binary.Write(buf, DefaultEndian, size); err != nil {
		return err
	}
	_, err := buf.Write([]byte(s))
	return err
}

func ReadString(s *string, buf *bytes.Reader) error {
	var size uint32
	if err := binary.Read(buf, DefaultEndian, &size); err != nil {
		return err
	}
	b := make([]byte, size)
	if _, err := buf.Read(b); err != nil {
		return err
	}
	*s = string(b)
	return nil
}

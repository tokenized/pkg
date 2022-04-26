package threshold

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/pkg/errors"
)

type EphemeralKey struct {
	ID              uint64 // to identify this key
	Degree          int
	LittleK         big.Int // little k
	LittleKIsSet    bool
	Alpha           big.Int // blinding value
	AlphaIsSet      bool
	VWShares        [][]big.Int // V W shares from all members (for signatures)
	IsComplete      bool        // The key has been calculated
	Key             big.Int
	IsUsed          bool // A signature share has been given out for this key
	SigHash         big.Int
	SignatureShares []BigPair // Shares used to construct a signature
}

func NewEphemeralKey(id uint64, degree int) *EphemeralKey {
	return &EphemeralKey{
		ID:     id,
		Degree: degree,
	}
}

// SignatureThreshold returns the number of signature shares required to construct a valid
//   signature.
func (e EphemeralKey) SignatureThreshold() int {
	return (2 * e.Degree) + 1
}

// GetVWShare calculates and returns the vw share for this member from the current littlek and alpha
//   values.
func (e *EphemeralKey) GetVWShare(ordinal big.Int) []big.Int {
	if !e.LittleKIsSet || !e.AlphaIsSet {
		return nil
	}

	// v = LittleK * Alpha
	v := ModMultiply(e.LittleK, e.Alpha, *bigModulo)

	// w = Alpha * Generator Point
	w := MultiplyByGenerator(e.Alpha)

	result := []big.Int{ordinal, v, w.X, w.Y}
	e.VWShares = append(e.VWShares, result)
	return result
}

// AddVWShare adds another member's vw share.
func (e *EphemeralKey) AddVWShare(vw []big.Int) {
	e.VWShares = append(e.VWShares, vw)
}

// VWSharesComplete returns true if the threshold of vw shares from other members have been received.
func (e *EphemeralKey) VWSharesComplete() bool {
	count := 0
	for _, vwShared := range e.VWShares {
		if len(vwShared) == 4 { // Each vw is 4 values (ordinal, v, wx, wy)
			count++
		}
	}

	return count >= e.SignatureThreshold()
}

// CalculateKey calculates an ephemeral key from the vw shares.
// Before calling this function at least 3 rounds of polynomial data shares and calculations are
//   required. One for each of private key, littlek, and alpha. Then an exchange of vw shares.
func (e *EphemeralKey) CalculateKey(ordinals []big.Int) error {
	if !e.VWSharesComplete() {
		return errors.New("Missing vw shares")
	}

	xfx_v := make([]BigPair, 0, len(e.VWShares))
	xfx_w := make([]BigOrdPair, 0, len(e.VWShares))

	for _, vw := range e.VWShares {
		xfx_v = append(xfx_v, BigPair{X: vw[0], Y: vw[1]})
		xfx_w = append(xfx_w, BigOrdPair{Ord: vw[0], Point: BigPair{X: vw[2], Y: vw[3]}})
	}

	// Lagrange Interplate at 0 to get V
	vZero, err := LagrangeInterpolate(xfx_v, *bigZero, *bigModulo)
	if err != nil {
		return errors.Wrap(err, "interpolate v")
	}
	vZeroInv := ModInverse(vZero, *bigModulo)

	// Lagrange EC Interplate at 0 to get W
	wZero, err := LagrangeECInterpolate(xfx_w, *bigZero, *bigModulo)
	if err != nil {
		return errors.Wrap(err, "interpolate point w")
	}

	x, y := curveS256.ScalarMult(&wZero.X, &wZero.Y, vZeroInv.Bytes())

	if !curveS256.IsOnCurve(x, y) {
		return ErrNotOnCurve
	}

	e.IsComplete = true
	e.Key.Set(x)
	return nil
}

func (e *EphemeralKey) Serialize(buf *bytes.Buffer) error {
	// ID              uint64 // to identify this key
	if err := binary.Write(buf, DefaultEndian, e.ID); err != nil {
		return err
	}

	// Degree          int
	d := uint32(e.Degree)
	if err := binary.Write(buf, DefaultEndian, d); err != nil {
		return err
	}

	// LittleK         big.Int     // little k
	if err := WriteBigInt(e.LittleK, buf); err != nil {
		return err
	}

	// LittleKIsSet      bool
	if err := binary.Write(buf, DefaultEndian, e.LittleKIsSet); err != nil {
		return err
	}

	// Alpha           big.Int     // blinding value
	if err := WriteBigInt(e.Alpha, buf); err != nil {
		return err
	}

	// AlphaIsSet      bool
	if err := binary.Write(buf, DefaultEndian, e.AlphaIsSet); err != nil {
		return err
	}

	// VWShares        [][]big.Int // V W shares from all members (for signatures)
	count := uint32(len(e.VWShares))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, vwShare := range e.VWShares {
		count = uint32(len(vwShare))
		if err := binary.Write(buf, DefaultEndian, count); err != nil {
			return err
		}
		for _, vw := range vwShare {
			if err := WriteBigInt(vw, buf); err != nil {
				return err
			}
		}
	}

	// IsComplete      bool        // The key has been calculated
	if err := binary.Write(buf, DefaultEndian, e.IsComplete); err != nil {
		return err
	}

	// Key             big.Int
	if err := WriteBigInt(e.Key, buf); err != nil {
		return err
	}

	// IsUsed          bool // A signature share has been given out for this key
	if err := binary.Write(buf, DefaultEndian, e.IsUsed); err != nil {
		return err
	}

	// SigHash         big.Int
	if err := WriteBigInt(e.SigHash, buf); err != nil {
		return err
	}

	// SignatureShares []BigPair // Shares used to construct a signature
	count = uint32(len(e.SignatureShares))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, eval := range e.SignatureShares {
		if err := WriteBigPair(eval, buf); err != nil {
			return err
		}
	}

	return nil
}

func (e *EphemeralKey) Deserialize(buf *bytes.Reader) error {
	// ID              uint64 // to identify this key
	if err := binary.Read(buf, DefaultEndian, &e.ID); err != nil {
		return err
	}

	// Degree          int
	var d uint32
	if err := binary.Read(buf, DefaultEndian, &d); err != nil {
		return err
	}
	e.Degree = int(d)

	// LittleK         big.Int     // little k
	if err := ReadBigInt(&e.LittleK, buf); err != nil {
		return err
	}

	// LittleKIsSet          bool
	if err := binary.Read(buf, DefaultEndian, &e.LittleKIsSet); err != nil {
		return err
	}

	// Alpha           big.Int     // blinding value
	if err := ReadBigInt(&e.Alpha, buf); err != nil {
		return err
	}

	// AlphaIsSet          bool
	if err := binary.Read(buf, DefaultEndian, &e.AlphaIsSet); err != nil {
		return err
	}

	// VWShares        [][]big.Int // V W shares from all members (for signatures)
	var count uint32
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	var jcount uint32
	e.VWShares = make([][]big.Int, 0, count)
	for i := uint32(0); i < count; i++ {
		if err := binary.Read(buf, DefaultEndian, &jcount); err != nil {
			return err
		}

		vwShares := make([]big.Int, 0, jcount)
		for j := uint32(0); j < jcount; j++ {
			var vw big.Int
			if err := ReadBigInt(&vw, buf); err != nil {
				return err
			}
			vwShares = append(vwShares, vw)
		}
		e.VWShares = append(e.VWShares, vwShares)
	}

	// IsComplete      bool        // The key has been calculated
	if err := binary.Read(buf, DefaultEndian, &e.IsComplete); err != nil {
		return err
	}

	// Key             big.Int
	if err := ReadBigInt(&e.Key, buf); err != nil {
		return err
	}

	// IsUsed          bool // A signature share has been given out for this key
	if err := binary.Read(buf, DefaultEndian, &e.IsUsed); err != nil {
		return err
	}

	// SigHash         big.Int
	if err := ReadBigInt(&e.SigHash, buf); err != nil {
		return err
	}

	// SignatureShares []BigPair // Shares used to construct a signature
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	e.SignatureShares = make([]BigPair, 0, count)
	for i := uint32(0); i < count; i++ {
		var coeff BigPair
		if err := ReadBigPair(&coeff, buf); err != nil {
			return err
		}
		e.SignatureShares = append(e.SignatureShares, coeff)
	}

	return nil
}

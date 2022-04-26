package threshold

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/tokenized/pkg/bitcoin"
)

const (
	// SecretShare Types
	PrivateKeyShare = 0
	LittleK         = 1
	Alpha           = 2
)

// SecretShare is the data used during Joint Verifiable Random Secret Sharing (JVRSS).
type SecretShare struct {
	ID   uint64 // id of ephemeral key, zero for private key share
	Type int

	// Calculated data
	Evals            []big.Int // Polynomial evaluated for ordinals
	HiddenEvals      []BigPair // Polynomial evaluations multiplied by generator point
	HiddenPolynomial []BigPair // Coefficients multiplied by generator point

	// Shared data from and to all members
	Shared            []bool
	SharedEvals       [][]BigPair // Hidden evaluations of all ordinals on other party's polynomial
	SharedPolynomials [][]BigPair // Hidden coefficients of other party's polynomial

	// Shared data privately from one member
	ActualEvalShared []bool
	ActualEvals      []big.Int // Non-hidden evaluation of this party's ordinal on other party's polynomial
}

func SecretShareTypeName(t int) string {
	switch t {
	case PrivateKeyShare:
		return "PrivateKeyShare"
	case LittleK:
		return "LittleK"
	case Alpha:
		return "Alpha"
	}
	return "Unknown"
}

// NewTransientPolynomial generates a new private polynomial and does precalculation on it. This
//   should be done before each step in an ephemeral key generation (littlek and alpha).
func NewSecretShare(id uint64, secretType, degree, ordinalIndex int, ordinals []big.Int) (*SecretShare, error) {
	result := SecretShare{
		ID:   id,
		Type: secretType,
	}

	poly, err := NewPolynomial(degree, *bigOne, *bigModulo)
	if err != nil {
		return nil, errors.Wrap(err, "new polynomial")
	}

	err = result.preCalculation(poly, ordinalIndex, ordinals)
	if err != nil {
		return nil, errors.Wrap(err, "precalculation")
	}

	return &result, nil
}

// ManualSetup is for specifying a polynomial to use instead of having a random polynomial.
func (s *SecretShare) ManualSetup(poly Polynomial, ordinalIndex int, ordinals []big.Int) error {
	return s.preCalculation(poly, ordinalIndex, ordinals)
}

// PolynomialPreCalculation calculates information about the private polynomial that is needed for
//   the following steps of Joint Verifiable Random Secret Sharing (JVRSS).
// Evaluate the polynomial for all ordinals and hide (encrypt) them.
// Hide (encrypt) the coefficients of the private polynomial.
func (s *SecretShare) preCalculation(poly Polynomial, ordinalIndex int, ordinals []big.Int) error {

	// evaluate polynomials for the group ordinals
	s.Evals = make([]big.Int, 0, len(ordinals))
	s.HiddenEvals = make([]BigPair, 0, len(ordinals))
	for _, ord := range ordinals {
		p := poly.Evaluate(ord)
		s.Evals = append(s.Evals, p)
		s.HiddenEvals = append(s.HiddenEvals, MultiplyByGenerator(p))
	}

	// hide polynomial using generator (base) point
	s.HiddenPolynomial = poly.Hide()

	s.Shared = make([]bool, len(ordinals))
	s.Shared[ordinalIndex] = true // we know our own values

	s.SharedEvals = make([][]BigPair, len(ordinals))
	s.SharedEvals[ordinalIndex] = s.HiddenEvals

	s.SharedPolynomials = make([][]BigPair, len(ordinals))
	s.SharedPolynomials[ordinalIndex] = s.HiddenPolynomial

	s.ActualEvalShared = make([]bool, len(ordinals))
	s.ActualEvalShared[ordinalIndex] = true // we know our own value

	s.ActualEvals = make([]big.Int, len(ordinals))
	s.ActualEvals[ordinalIndex] = s.Evals[ordinalIndex]

	return nil
}

// GetShare returns the data to share to the group.
// This should be shared via broadcast to all members in the group, so everyone sees that everyone
//   received the same data.
func (s SecretShare) GetShare() ([]BigPair, []BigPair) {
	return s.HiddenPolynomial, s.HiddenEvals
}

// AddShare adds another member's shared data to the current calculation.
func (s *SecretShare) AddShare(fromIndex int, poly, evals []BigPair) {
	s.Shared[fromIndex] = true
	s.SharedPolynomials[fromIndex] = poly
	s.SharedEvals[fromIndex] = evals
}

// GetEvalShare returns the unhidden evaluation of the toOrdinal on the private polynomial.
// This should be shared privately to only the owner of toOrdinal.
func (s SecretShare) GetEvalShare(toIndex int) big.Int {
	return s.Evals[toIndex]
}

// AddEvalShare adds the unhidden evaluation of this ordinal on another member's private polynomial.
func (s *SecretShare) AddEvalShare(fromIndex int, eval big.Int) {
	s.ActualEvalShared[fromIndex] = true
	s.ActualEvals[fromIndex] = eval
}

// SharesComplete returns true if all shares from other members have been populated.
func (s SecretShare) SharesComplete() bool {
	for _, shared := range s.Shared {
		if !shared {
			return false
		}
	}
	for _, shared := range s.ActualEvalShared {
		if !shared {
			return false
		}
	}

	return true
}

// CreateSecret creates a secret from the shared data by adding the evaluations of our ordinal on
//   all of the other member's private polynomials.
// The unhidden evaluation of this member's ordinal should only be known by this member and the
//   member that generated it.
func (s SecretShare) CreateSecret(ordinalIndex int, ordinals []big.Int) (big.Int, error) {
	var result big.Int

	if !s.SharesComplete() {
		return result, errors.New("Missing shares")
	}

	if err := s.verifyCorrectness(ordinals); err != nil {
		return result, errors.Wrap(err, "correctness")
	}
	if err := s.verifyHonesty(ordinalIndex, ordinals); err != nil {
		return result, errors.Wrap(err, "honesty")
	}

	for _, eval := range s.ActualEvals {
		result = ModAdd(result, eval, *bigModulo)
	}

	return result, nil
}

// CreatePublicKey creates the public key of the shared secret by adding all of the 0th hidden
//   coefficients.
func (s SecretShare) CreatePublicKey() *bitcoin.PublicKey {
	var result BigPair
	result.Set(s.SharedPolynomials[0][0])

	for _, coeffList := range s.SharedPolynomials[1:] {
		x, y := curveS256.Add(&result.X, &result.Y, &coeffList[0].X, &coeffList[0].Y)
		result.X.Set(x)
		result.Y.Set(y)
	}

	return &bitcoin.PublicKey{
		X: result.X,
		Y: result.Y,
	}
}

// verifyCorrectness returns an error if any of the shared data is not correct.
func (s SecretShare) verifyCorrectness(ordinals []big.Int) error {
	for i, ordinal := range ordinals {
		curvepoints := make([]BigOrdPair, 0, len(s.SharedEvals[i]))
		for j, pubPoly := range s.SharedEvals[i] {
			curvepoints = append(curvepoints, BigOrdPair{Ord: ordinals[j], Point: pubPoly})
		}

		// Interpolate at zero
		zeroVal, err := LagrangeECInterpolate(curvepoints, *bigZero, *bigModulo)
		if err != nil {
			return errors.Wrap(err, "interpolate point")
		}

		// First coefficient is the "constant" (c*x^0), not multiplied by x, just added to y
		if !zeroVal.Equal(s.SharedPolynomials[i][0]) {
			return errors.Wrap(ErrNotCorrect, fmt.Sprintf("ordinal %d %s :\n  %s\n !=\n  %s", i,
				ordinal.String(), zeroVal.String(), s.SharedPolynomials[i][0].String()))
		}
	}

	return nil
}

// verifyHonesty returns an error if any of the shared data is not honest (does not line up).
func (s SecretShare) verifyHonesty(ordinalIndex int, ordinals []big.Int) error {
	// Verify that unencrypted (unhidden) evaluations encrypt to the hidden shared value.
	for i, _ := range ordinals {
		// Perform eval hide calculation.
		hiddenEval := MultiplyByGenerator(s.ActualEvals[i])

		// Verify the shared hidden eval matches.
		if !s.SharedEvals[i][ordinalIndex].Equal(hiddenEval) {
			return errors.Wrap(ErrDishonest, fmt.Sprintf("invalid eval %d : \n  eval %s\n  h.X  %s\n  h.Y  %s", i,
				s.ActualEvals[i].String(), s.SharedEvals[i][ordinalIndex].X.String(), s.SharedEvals[i][ordinalIndex].Y.String()))
		}
	}

	// Verify that the shared hidden evals line up with the shared hidden polynomials.
	for from, _ := range ordinals {
		for to, toOrd := range ordinals {
			if from == to {
				continue
			}

			if !verifySharedEval(s.SharedEvals[from][to], s.SharedPolynomials, from, to, toOrd) {
				return errors.Wrap(ErrDishonest, fmt.Sprintf("invalid hidden eval : %d, %d", from, to))
			}
		}
	}

	return nil
}

// verifySharedEval verifies that the shared hidden eval corresponds to the shared hidden polynomial.
func verifySharedEval(sharedEval BigPair, sharedPolynomials [][]BigPair,
	from, to int, toOrd big.Int) bool {

	multiplier := toOrd
	encryptedCoeffs := sharedPolynomials[from]
	var resCoeffEC BigPair
	resCoeffEC.Set(encryptedCoeffs[0])
	for _, coeff := range encryptedCoeffs[1:] {
		cx, cy := curveS256.ScalarMult(&coeff.X, &coeff.Y, multiplier.Bytes())
		sx, sy := curveS256.Add(&resCoeffEC.X, &resCoeffEC.Y, cx, cy)
		resCoeffEC.X.Set(sx)
		resCoeffEC.Y.Set(sy)
		multiplier = ModMultiply(multiplier, toOrd, *bigModulo)
	}

	return sharedEval.Equal(resCoeffEC)
}

func (s *SecretShare) Serialize(buf *bytes.Buffer) error {
	// ID   uint64 // id of ephemeral key, zero for private key share
	if err := binary.Write(buf, DefaultEndian, s.ID); err != nil {
		return err
	}

	// Type int
	t := uint32(s.Type)
	if err := binary.Write(buf, DefaultEndian, t); err != nil {
		return err
	}

	// Evals            []big.Int // Polynomial evaluated for ordinals
	count := uint32(len(s.Evals))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, eval := range s.Evals {
		if err := WriteBigInt(eval, buf); err != nil {
			return err
		}
	}

	// HiddenEvals      []BigPair // Polynomial evaluations multiplied by generator point
	count = uint32(len(s.HiddenEvals))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, eval := range s.HiddenEvals {
		if err := WriteBigPair(eval, buf); err != nil {
			return err
		}
	}

	// HiddenPolynomial []BigPair // Coefficients multiplied by generator point
	count = uint32(len(s.HiddenPolynomial))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, coeff := range s.HiddenPolynomial {
		if err := WriteBigPair(coeff, buf); err != nil {
			return err
		}
	}

	// Shared            []bool
	count = uint32(len(s.Shared))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, shared := range s.Shared {
		if err := binary.Write(buf, DefaultEndian, shared); err != nil {
			return err
		}
	}

	// SharedEvals       [][]BigPair // Hidden evaluations of all ordinals on other party's polynomial
	count = uint32(len(s.SharedEvals))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, sharedEval := range s.SharedEvals {
		count = uint32(len(sharedEval))
		if err := binary.Write(buf, DefaultEndian, count); err != nil {
			return err
		}
		for _, eval := range sharedEval {
			if err := WriteBigPair(eval, buf); err != nil {
				return err
			}
		}
	}

	// SharedPolynomials [][]BigPair // Hidden coefficients of other party's polynomial
	count = uint32(len(s.SharedPolynomials))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, sharedPoly := range s.SharedPolynomials {
		count = uint32(len(sharedPoly))
		if err := binary.Write(buf, DefaultEndian, count); err != nil {
			return err
		}
		for _, coeff := range sharedPoly {
			if err := WriteBigPair(coeff, buf); err != nil {
				return err
			}
		}
	}

	// ActualEvalShared []bool
	count = uint32(len(s.ActualEvalShared))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, shared := range s.ActualEvalShared {
		if err := binary.Write(buf, DefaultEndian, shared); err != nil {
			return err
		}
	}

	// ActualEvals      []big.Int // Non-hidden evaluation of this party's ordinal on other party's polynomial
	count = uint32(len(s.ActualEvals))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, eval := range s.ActualEvals {
		if err := WriteBigInt(eval, buf); err != nil {
			return err
		}
	}

	return nil
}

func (s *SecretShare) Deserialize(buf *bytes.Reader) error {
	// ID   uint64 // id of ephemeral key, zero for private key share
	if err := binary.Read(buf, DefaultEndian, &s.ID); err != nil {
		return err
	}

	// Type int
	var t uint32
	if err := binary.Read(buf, DefaultEndian, &t); err != nil {
		return err
	}
	s.Type = int(t)

	// Evals            []big.Int // Polynomial evaluated for ordinals
	var count uint32
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.Evals = make([]big.Int, 0, count)
	for i := uint32(0); i < count; i++ {
		var eval big.Int
		if err := ReadBigInt(&eval, buf); err != nil {
			return err
		}
		s.Evals = append(s.Evals, eval)
	}

	// HiddenEvals      []BigPair // Polynomial evaluations multiplied by generator point
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.HiddenEvals = make([]BigPair, 0, count)
	for i := uint32(0); i < count; i++ {
		var eval BigPair
		if err := ReadBigPair(&eval, buf); err != nil {
			return err
		}
		s.HiddenEvals = append(s.HiddenEvals, eval)
	}

	// HiddenPolynomial []BigPair // Coefficients multiplied by generator point
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.HiddenPolynomial = make([]BigPair, 0, count)
	for i := uint32(0); i < count; i++ {
		var coeff BigPair
		if err := ReadBigPair(&coeff, buf); err != nil {
			return err
		}
		s.HiddenPolynomial = append(s.HiddenPolynomial, coeff)
	}

	// Shared            []bool
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.Shared = make([]bool, 0, count)
	for i := uint32(0); i < count; i++ {
		var shared bool
		if err := binary.Read(buf, DefaultEndian, &shared); err != nil {
			return err
		}
		s.Shared = append(s.Shared, shared)
	}

	// SharedEvals       [][]BigPair // Hidden evaluations of all ordinals on other party's polynomial
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	var jcount uint32
	s.SharedEvals = make([][]BigPair, 0, count)
	for i := uint32(0); i < count; i++ {
		if err := binary.Read(buf, DefaultEndian, &jcount); err != nil {
			return err
		}

		sharedEvals := make([]BigPair, 0, jcount)
		for j := uint32(0); j < jcount; j++ {
			var eval BigPair
			if err := ReadBigPair(&eval, buf); err != nil {
				return err
			}
			sharedEvals = append(sharedEvals, eval)
		}
		s.SharedEvals = append(s.SharedEvals, sharedEvals)
	}

	// SharedPolynomials [][]BigPair // Hidden coefficients of other party's polynomial
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.SharedPolynomials = make([][]BigPair, 0, count)
	for i := uint32(0); i < count; i++ {
		if err := binary.Read(buf, DefaultEndian, &jcount); err != nil {
			return err
		}

		sharedPoly := make([]BigPair, 0, jcount)
		for j := uint32(0); j < jcount; j++ {
			var coeff BigPair
			if err := ReadBigPair(&coeff, buf); err != nil {
				return err
			}
			sharedPoly = append(sharedPoly, coeff)
		}
		s.SharedPolynomials = append(s.SharedPolynomials, sharedPoly)
	}

	// ActualEvalShared []bool
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.ActualEvalShared = make([]bool, 0, count)
	for i := uint32(0); i < count; i++ {
		var shared bool
		if err := binary.Read(buf, DefaultEndian, &shared); err != nil {
			return err
		}
		s.ActualEvalShared = append(s.ActualEvalShared, shared)
	}

	// ActualEvals      []big.Int // Non-hidden evaluation of this party's ordinal on other party's polynomial
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return err
	}

	s.ActualEvals = make([]big.Int, 0, count)
	for i := uint32(0); i < count; i++ {
		var eval big.Int
		if err := ReadBigInt(&eval, buf); err != nil {
			return err
		}
		s.ActualEvals = append(s.ActualEvals, eval)
	}

	return nil
}

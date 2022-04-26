package threshold

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
)

// Polynomial represents a
type Polynomial struct {
	Coefficients []big.Int
}

func (p Polynomial) Degree() int {
	return len(p.Coefficients) - 1
}

// NewPolynomial generates a new random polynomial of the specified degree. The first coefficient
//   will not be less than 1 and the last coefficient will not be zero. All other coefficients will
//   be within the inclusive range of the min and max specified.
func NewPolynomial(degree int, min, max big.Int) (Polynomial, error) {
	if degree < 1 {
		return Polynomial{}, errors.New("Degree must be one or more")
	}

	p := Polynomial{
		Coefficients: make([]big.Int, 0, degree+1),
	}

	for i := 0; i <= degree; i++ {
		randomBig, err := generateRandRange(min, max)
		if err != nil {
			return Polynomial{}, errors.Wrap(err, "generate range")
		}

		if i == 0 { // first
			for randomBig.Cmp(bigOne) < 0 {
				randomBig, err = generateRandRange(min, max)
				if err != nil {
					return Polynomial{}, errors.Wrap(err, "generate range")
				}
			}
		} else if i == degree { // last
			for randomBig.Cmp(bigZero) == 0 {
				randomBig, err = generateRandRange(min, max)
				if err != nil {
					return Polynomial{}, errors.Wrap(err, "generate range")
				}
			}
		}

		p.Coefficients = append(p.Coefficients, randomBig)
	}

	return p, nil
}

// Evaluate returns the y value when the polynomial is evaluated for x.
func (p Polynomial) Evaluate(x big.Int) big.Int {

	// Horners method for polynomial evaluation
	var result big.Int
	for i := len(p.Coefficients) - 1; i >= 0; i-- {
		result = ModMultiply(result, x, *bigModulo)
		result = ModAdd(result, p.Coefficients[i], *bigModulo)
	}

	return result
}

// IsValid returns true if the polynomial is valid.
func (p Polynomial) IsValid() bool {
	// Empty
	if len(p.Coefficients) == 0 {
		return false
	}

	// Last is zero
	if p.Coefficients[len(p.Coefficients)-1].Cmp(bigZero) == 0 {
		return false
	}

	// First is less than one
	if p.Coefficients[0].Cmp(bigOne) < 0 {
		return false
	}

	return true
}

// Hide returns the coefficients multiplied by the generator (base) point.
func (p Polynomial) Hide() []BigPair {
	result := make([]BigPair, 0, len(p.Coefficients))
	for _, coef := range p.Coefficients {
		x, y := curveS256.ScalarBaseMult(coef.Bytes())
		result = append(result, BigPair{X: *x, Y: *y})
	}
	return result
}

// LagrangeInterpolate uses scalar lagrange interpolation to evaluate x based on the points
//   specified.
func LagrangeInterpolate(points []BigPair, x, mod big.Int) (big.Int, error) {
	// Sanity check the points.
	if len(points) < 2 {
		return big.Int{}, errors.New("Not enough points")
	}

	for i, point := range points {
		for _, point2 := range points[i+1:] {
			if point.Equal(point2) {
				return big.Int{}, errors.New("Duplicate points")
			}
		}
	}

	var result big.Int

	// Catch any panics from big int math
	defer func() (big.Int, error) {
		if r := recover(); r != nil {
			return big.Int{}, fmt.Errorf("Interpolate panic : %v", r)
		}
		return result, nil
	}()

	//    k
	//   ---
	//   \
	//   /   y(j)lj(x)
	//   ---
	//   j=0

	for j, jPoint := range points {
		// Calculate lj(x)
		eval := lj(j, jPoint, x, points, mod)

		// Multiply by y(j)
		mul := ModMultiply(eval, jPoint.Y, mod)

		// Add to result
		result = ModAdd(result, mul, mod)
	}

	return result, nil
}

func lj(j int, jPoint BigPair, x big.Int, points []BigPair, mod big.Int) big.Int {
	//   ------
	//    |  |      x    - x(m)
	//    |  |     -------------
	//   0<=m<=k    x(j) - x(m)
	//    m!=j

	result := *bigOne
	for m, mPoint := range points {
		if m == j {
			continue
		}

		numerator := ModSubtract(x, mPoint.X, mod)          // x - x(m)
		denominator := ModSubtract(jPoint.X, mPoint.X, mod) // x(j) - x(m)
		val := ModDivide(numerator, denominator, mod)
		result = ModMultiply(result, val, mod)
	}

	return result
}

// LagrangeECInterpolate uses elliptic point (EC) lagrange interpolation to evaluate x based on the
//   points specified.
func LagrangeECInterpolate(points []BigOrdPair, x, mod big.Int) (BigPair, error) {
	// Sanity check the points.
	if len(points) < 2 {
		return BigPair{}, errors.New("Not enough points")
	}

	for i, point := range points {
		for _, point2 := range points[i+1:] {
			if point.Equal(point2) {
				return BigPair{}, errors.New("Duplicate points")
			}
		}
	}

	var result BigPair

	// Catch any panics from big int math
	defer func() (BigPair, error) {
		if r := recover(); r != nil {
			return BigPair{}, fmt.Errorf("EC Interpolate panic : %v", r)
		}
		return result, nil
	}()

	//    k
	//   ---
	//   \
	//   /   y(j)lj(x)
	//   ---
	//   j=0

	for j, jPoint := range points {
		// Calculate lj(x)
		eval := eclj(j, jPoint, x, points, mod)

		// Multiply by y(j)
		x, y := curveS256.ScalarMult(&jPoint.Point.X, &jPoint.Point.Y, eval.Bytes())

		// Add to result
		xs, ys := curveS256.Add(&result.X, &result.Y, x, y)
		result.X.Set(xs)
		result.Y.Set(ys)
	}

	return result, nil
}

func eclj(j int, jPoint BigOrdPair, x big.Int, points []BigOrdPair, mod big.Int) big.Int {
	//   ------
	//    |  |      x    - x(m)
	//    |  |     -------------
	//   0<=m<=k    x(j) - x(m)
	//    m!=j

	result := *bigOne
	for m, mPoint := range points {
		if m == j {
			continue
		}

		numerator := ModSubtract(x, mPoint.Ord, mod)            // x - x(m)
		denominator := ModSubtract(jPoint.Ord, mPoint.Ord, mod) // x(j) - x(m)
		val := ModDivide(numerator, denominator, mod)
		result = ModMultiply(result, val, mod)
	}

	return result
}

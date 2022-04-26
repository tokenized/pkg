package threshold

import (
	"encoding/binary"

	"github.com/btcsuite/btcd/btcec"
	"github.com/pkg/errors"
)

var (
	ErrSecretNotFound  = errors.New("Secret share not found")
	ErrNotCorrect      = errors.New("Incorrect shares")
	ErrDishonest       = errors.New("Dishonest coefficient")
	ErrInvalidShares   = errors.New("Invalid shares")
	ErrOrdinalNotFound = errors.New("Ordinal not found")
	ErrNotOnCurve      = errors.New("Point not on curve")
	ErrThresholdNotMet = errors.New("Threshold not met")
	ErrSigHashMismatch = errors.New("Signature hash doesn't match ephemeral key")

	DefaultEndian = binary.LittleEndian

	curveS256 = btcec.S256()
)

package threshold

import (
	"math/big"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

func (e *EphemeralKey) SignatureShareSent(fromOrdinal big.Int) bool {
	for _, share := range e.SignatureShares {
		if share.X.Cmp(&fromOrdinal) == 0 {
			return true
		}
	}

	return false
}

// GetSignatureShare returns the signature share required to create a valid signature.
func (e *EphemeralKey) GetSignatureShare(fromOrdinal big.Int, privateKeyShare big.Int,
	sigHash big.Int) (big.Int, error) {

	if e.IsUsed {
		if e.SigHash.Cmp(&sigHash) != 0 {
			return big.Int{}, ErrSigHashMismatch
		}
	} else {
		e.SigHash.Mod(&sigHash, bigModulo)
		e.IsUsed = true
	}

	// s = LittleK * (sigHash + (m.PrivateKeyShare * Key))
	s := ModMultiply(privateKeyShare, e.Key, *bigModulo)
	s = ModAdd(e.SigHash, s, *bigModulo)
	s = ModMultiply(e.LittleK, s, *bigModulo)

	result := BigPair{fromOrdinal, s}
	e.SignatureShares = append(e.SignatureShares, result)
	return s, nil
}

// AddSignatureShare adds the signature share.
func (e *EphemeralKey) AddSignatureShare(sigHash big.Int, fromOrdinal big.Int, share big.Int) error {
	if e.IsUsed {
		if e.SigHash.Cmp(&sigHash) != 0 {
			return ErrSigHashMismatch
		}
	} else {
		e.SigHash.Mod(&sigHash, bigModulo)
		e.IsUsed = true
	}

	for _, share := range e.SignatureShares {
		if share.X.Cmp(&fromOrdinal) == 0 {
			return nil // already have this share
		}
	}

	pair := BigPair{fromOrdinal, share}
	e.SignatureShares = append(e.SignatureShares, pair)
	return nil
}

func (e *EphemeralKey) SignatureSharesComplete() bool {
	return len(e.SignatureShares) >= e.SignatureThreshold()
}

// CreateSignature uses the signature shares to create a full signature. Each signature share value
// is paired with its ordinal. (ordinal, sig share).
//
// At least 2t + 1 signature shares are required to create a valid signature, where t is the degree
// of the polynomials used. More than 2t + 1 will also create a valid signature, but if any shares
// are invalid, then the signature will be invalid even if 2t + 1 are valid.
func (e *EphemeralKey) CreateSignature(sigHash big.Int,
	publicKey bitcoin.PublicKey) (bitcoin.Signature, error) {

	e.IsUsed = true
	if e.SigHash.Cmp(&sigHash) != 0 {
		return bitcoin.Signature{}, ErrSigHashMismatch
	}

	// Calculate signing threshold
	if len(e.SignatureShares) < e.SignatureThreshold() {
		return bitcoin.Signature{}, ErrThresholdNotMet
	}

	sZero, err := LagrangeInterpolate(e.SignatureShares, *bigZero, *bigModulo)
	if err != nil {
		return bitcoin.Signature{}, errors.Wrap(err, "interpolate")
	}

	// Canonize S
	modDivByTwo := ModDivide(*bigModulo, *bigTwo, *bigModulo)
	if sZero.Cmp(&modDivByTwo) > 0 {
		sZero = ModSubtract(*bigModulo, sZero, *bigModulo)
	}

	result := bitcoin.Signature{
		R: e.Key,
		S: sZero,
	}

	hash, err := bitcoin.NewHash32(sigHash.Bytes())
	if err != nil {
		return bitcoin.Signature{}, errors.Wrap(err, "hash32")
	}

	// Verify signature because an invalid share can cause an invalid signature.
	if !result.Verify(*hash, publicKey) {
		return result, ErrInvalidShares
	}

	return result, nil
}

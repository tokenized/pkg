package threshold

import (
	"fmt"
	"math/big"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

// KeyThreshold returns the number of private key shares required to construct the full private key.
func (m Member) KeyThreshold() int {
	return m.Degree + 1
}

// GetPrivateKeyShare returns this member's share of the private key.
func (m *Member) GetPrivateKeyShare() big.Int {
	m.PrivateKeyShared[m.OrdinalIndex] = true
	m.PrivateKeyShares[m.OrdinalIndex] = m.PrivateKeyShare
	return m.PrivateKeyShare
}

// AddPrivateKeyShare adds another member's private key share to the current calculation.
func (m *Member) AddPrivateKeyShare(ordinal big.Int, secret big.Int) error {
	// Find ordinal in list
	for i, ord := range m.Ordinals {
		if ord.Cmp(&ordinal) == 0 {
			m.PrivateKeyShared[i] = true
			m.PrivateKeyShares[i] = secret
			return nil
		}
	}

	return ErrOrdinalNotFound
}

// PrivateKeySharesComplete returns true if the threshold of secret shares from other members have
// been received.
func (m *Member) PrivateKeySharesComplete() bool {
	count := 0
	for _, secretShared := range m.PrivateKeyShared {
		if secretShared {
			count++
		}
	}

	return count >= m.KeyThreshold()
}

func (m Member) GeneratePrivateKey(net bitcoin.Network) (bitcoin.Key, error) {
	shares := make([]BigPair, 0, len(m.Ordinals))
	for i, share := range m.PrivateKeyShares {
		if m.PrivateKeyShared[i] {
			shares = append(shares, BigPair{X: m.Ordinals[i], Y: share})
		}
	}

	if len(shares) < m.KeyThreshold() {
		return bitcoin.Key{}, errors.Wrap(ErrThresholdNotMet, fmt.Sprintf("%d of %d", len(shares),
			m.KeyThreshold()))
	}

	value, err := LagrangeInterpolate(shares, *bigZero, *bigModulo)
	if err != nil {
		return bitcoin.Key{}, errors.Wrap(err, "interpolate shares")
	}

	result := bitcoin.KeyFromValue(value, net)

	// Check public key to verify shares
	pubkey := result.PublicKey()
	if !pubkey.Equal(*m.PublicKey) {
		return result, ErrInvalidShares
	}

	return result, nil
}

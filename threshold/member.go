package threshold

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

const (
	PrivateKeyShareID = 0 // Secret Share ID for private key share
)

// Member represents one of the members in a group that is creating/using a shared secret via
//   Joint Verifiable Random Secret Sharing (JVRSS).
type Member struct {
	Degree       int       // degree of polynomials to use
	OrdinalIndex int       // index into Ordinals for ordinal of this member
	Ordinals     []big.Int // ordinal of all members

	PrivateKeyShare      big.Int            // share of secret
	PublicKey            *bitcoin.PublicKey // public key for secret
	PrivateKeyShareIsSet bool

	// Used to construct full private key
	PrivateKeyShared []bool    // Private key share was received
	PrivateKeyShares []big.Int // Private key shares from all members (for key recovery)

	PendingSecretShares []*SecretShare

	// EphemeralKeys are used to generate signatures. They can only be used once.
	EphemeralKeys   []*EphemeralKey
	NextEphemeralID uint64 // ID of the next ephemeral key
}

func NewEmptyMember(degree int) Member {
	return Member{
		Degree:          degree,
		NextEphemeralID: 1,
	}
}

// NewMember creates a new threshold group member, generates the first private polynomial, and does
//   precalculation on it.
// ordinal is the one identifying this member and must be included in ordinals.
func NewMember(ordinal big.Int, ordinals []big.Int, degree int) (Member, error) {
	result := NewEmptyMember(degree)
	err := result.SetOrdinals(ordinal, ordinals)
	return result, err
}

func (m *Member) HasPending() bool {
	if len(m.Ordinals) == 0 {
		return true
	}

	if len(m.PendingSecretShares) > 0 {
		return true
	}

	for _, key := range m.EphemeralKeys {
		if !key.IsComplete {
			return true
		}
		if key.IsUsed && !key.SignatureSharesComplete() {
			return true
		}
	}

	return false
}

func (m *Member) SetOrdinals(ordinal big.Int, ordinals []big.Int) error {
	m.Ordinals = ordinals

	// Ordinals must be sorted so all members have the same order for evals, so each eval is
	//   associated with the correct ordinal when shared, without requiring ordinals to be sent
	//   every time with evals.
	sort.Sort(SortOrdinals(m.Ordinals))

	m.PrivateKeyShared = make([]bool, len(ordinals))
	m.PrivateKeyShares = make([]big.Int, len(ordinals))

	// t + 1 members are required in the group to be able to recover the shared secret.
	// 2t + 1 for signature generation.
	if len(ordinals) < m.Degree+1 {
		return errors.Wrap(ErrThresholdNotMet,
			fmt.Sprintf("%d of %d members required", len(ordinals), m.Degree+1))
	}

	found := false
	for i, ord := range ordinals {
		if ord.Cmp(&ordinal) == 0 {
			m.OrdinalIndex = i
			found = true
			break
		}
	}

	if !found {
		return ErrOrdinalNotFound
	}

	pks, err := NewSecretShare(PrivateKeyShareID, PrivateKeyShare, m.Degree, m.OrdinalIndex,
		ordinals)
	if err != nil {
		return errors.Wrap(err, "new private key share")
	}

	m.PendingSecretShares = append(m.PendingSecretShares, pks)

	return nil
}

// Ordinal returns the ordinal for this member of the group.
func (m Member) Ordinal() big.Int {
	return m.Ordinals[m.OrdinalIndex]
}

// IndexForOrdinal returns the index, in the group ordinal list, of the specified ordinal.
func (m Member) IndexForOrdinal(o big.Int) int {
	for i, ord := range m.Ordinals {
		if o.Cmp(&ord) == 0 {
			return i
		}
	}
	return -1
}

// SignaturesSupported returns true if there are enough members in the group to generate signatures.
// 2t + 1 are members are required where t is the private polynomial's degree.
func (m Member) SignaturesSupported() bool {
	return len(m.Ordinals) >= (m.Degree*2)+1
}

// SignatureThreshold returns the number of signature shares required to construct a valid
//   signature.
func (m Member) SignatureThreshold() int {
	return (2 * m.Degree) + 1
}

// StartEphemeralKey starts the process of generating a new ephemeral key. It creates pending secret
//   shares for littlek and alpha.
func (m *Member) StartEphemeralKey() ([]*SecretShare, error) {
	key := NewEphemeralKey(m.NextEphemeralID, m.Degree)
	m.EphemeralKeys = append(m.EphemeralKeys, key)

	littleK, err := NewSecretShare(m.NextEphemeralID, LittleK, m.Degree, m.OrdinalIndex, m.Ordinals)
	if err != nil {
		return nil, errors.Wrap(err, "new little k")
	}

	m.PendingSecretShares = append(m.PendingSecretShares, littleK)

	alpha, err := NewSecretShare(m.NextEphemeralID, Alpha, m.Degree, m.OrdinalIndex, m.Ordinals)
	if err != nil {
		return nil, errors.Wrap(err, "new alpha")
	}

	m.PendingSecretShares = append(m.PendingSecretShares, alpha)
	m.NextEphemeralID++
	return []*SecretShare{littleK, alpha}, nil
}

// GetSecretShare returns the secret share with the specified id and type.
func (m *Member) GetSecretShare(id uint64, secretType int) *SecretShare {
	for _, secret := range m.PendingSecretShares {
		if id == secret.ID && secretType == secret.Type {
			return secret
		}
	}

	return nil
}

// SecretShare calculates the shared secret and removes it.
// It then puts the shared secret into the private key share or ephemeral key.
func (m *Member) FinishSecretShare(secretShare *SecretShare) error {
	if !secretShare.SharesComplete() {
		return nil
	}

	// Remove
	for i, ss := range m.PendingSecretShares {
		if secretShare == ss {
			m.PendingSecretShares = append(m.PendingSecretShares[:i], m.PendingSecretShares[i+1:]...)
			break
		}
	}

	sharedSecret, err := secretShare.CreateSecret(m.OrdinalIndex, m.Ordinals)
	if err != nil {
		return errors.Wrap(err, "create shared secret")
	}

	if secretShare.Type == PrivateKeyShare {
		m.PrivateKeyShare = sharedSecret
		m.PublicKey = secretShare.CreatePublicKey()
		m.PrivateKeyShareIsSet = true
	} else {
		ephemeralKey := m.FindEphemeralKey(secretShare.ID)
		if ephemeralKey == nil {
			return fmt.Errorf("Missing ephemeral key for shared secret %d %s", secretShare.ID,
				SecretShareTypeName(secretShare.Type))
		}
		if secretShare.Type == LittleK {
			ephemeralKey.LittleK = sharedSecret
			ephemeralKey.LittleKIsSet = true
		} else if secretShare.Type == Alpha {
			ephemeralKey.Alpha = sharedSecret
			ephemeralKey.AlphaIsSet = true
		} else {
			return fmt.Errorf("Unexpected shared secret type for ephemeral key %d %s",
				secretShare.ID, SecretShareTypeName(secretShare.Type))
		}
	}

	return nil
}

func (m *Member) FindEphemeralKey(id uint64) *EphemeralKey {
	for _, key := range m.EphemeralKeys {
		if key.ID == id {
			return key
		}
	}

	return nil
}

func (m *Member) FindUnusedEphemeralKey() *EphemeralKey {
	for _, key := range m.EphemeralKeys {
		if !key.IsUsed {
			return key
		}
	}

	return nil
}

// RemoveEphemeralKey removes the ephemeral key with the specified id and type.
func (m *Member) RemoveEphemeralKey(id uint64) bool {
	for i, key := range m.EphemeralKeys {
		if id == key.ID {
			m.EphemeralKeys = append(m.EphemeralKeys[:i], m.EphemeralKeys[i+1:]...)
			return true
		}
	}

	return false
}

// Reset resets the ephemeral keys
func (m *Member) Reset() error {
	m.PendingSecretShares = nil
	m.EphemeralKeys = nil
	m.NextEphemeralID = 1
	return nil
}

func (m *Member) Serialize(buf *bytes.Buffer) error {
	// Degree
	value := uint32(m.Degree)
	if err := binary.Write(buf, DefaultEndian, value); err != nil {
		return err
	}

	// OrdinalIndex
	value = uint32(m.OrdinalIndex)
	if err := binary.Write(buf, DefaultEndian, value); err != nil {
		return err
	}

	// Ordinals
	count := uint32(len(m.Ordinals))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, ordinal := range m.Ordinals {
		if err := WriteBigInt(ordinal, buf); err != nil {
			return err
		}
	}

	// PrivateKeyShareIsSet bool
	if err := binary.Write(buf, DefaultEndian, m.PrivateKeyShareIsSet); err != nil {
		return err
	}

	if m.PrivateKeyShareIsSet {
		// PrivateKeyShare big.Int           // share of secret
		if err := WriteBigInt(m.PrivateKeyShare, buf); err != nil {
			return err
		}

		// PublicKey       bitcoin.PublicKey // public key for secret
		if err := m.PublicKey.Serialize(buf); err != nil {
			return err
		}
	}

	// PrivateKeyShared []bool    // Private key share was received
	count = uint32(len(m.PrivateKeyShared))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, pksd := range m.PrivateKeyShared {
		if err := binary.Write(buf, DefaultEndian, pksd); err != nil {
			return err
		}
	}

	// PrivateKeyShares []big.Int // Private key shares from all members (for key recovery)
	count = uint32(len(m.PrivateKeyShares))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, pks := range m.PrivateKeyShares {
		if err := WriteBigInt(pks, buf); err != nil {
			return err
		}
	}

	// PendingSecretShares []*SecretShare
	count = uint32(len(m.PendingSecretShares))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, ss := range m.PendingSecretShares {
		if err := ss.Serialize(buf); err != nil {
			return err
		}
	}

	// EphemeralKeys   []*EphemeralKey
	count = uint32(len(m.EphemeralKeys))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for _, ek := range m.EphemeralKeys {
		if err := ek.Serialize(buf); err != nil {
			return err
		}
	}

	// NextEphemeralID uint64 // ID of the next ephemeral key
	if err := binary.Write(buf, DefaultEndian, m.NextEphemeralID); err != nil {
		return err
	}

	return nil
}

func (m *Member) Deserialize(buf *bytes.Reader) error {
	// Degree
	var value uint32
	if err := binary.Read(buf, DefaultEndian, &value); err != nil {
		return errors.Wrap(err, "value")
	}
	m.Degree = int(value)

	// OrdinalIndex
	if err := binary.Read(buf, DefaultEndian, &value); err != nil {
		return errors.Wrap(err, "ordinal index")
	}
	m.OrdinalIndex = int(value)

	// Ordinals
	var count uint32
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return errors.Wrap(err, "ordinal count")
	}

	m.Ordinals = make([]big.Int, 0, count)
	for i := uint32(0); i < count; i++ {
		var ordinal big.Int
		if err := ReadBigInt(&ordinal, buf); err != nil {
			return errors.Wrapf(err, "ordinal %d", i)
		}
		m.Ordinals = append(m.Ordinals, ordinal)
	}

	// PrivateKeyShareIsSet bool
	if err := binary.Read(buf, DefaultEndian, &m.PrivateKeyShareIsSet); err != nil {
		return errors.Wrap(err, "private key share is set")
	}

	if m.PrivateKeyShareIsSet {
		// PrivateKeyShare big.Int           // share of secret
		if err := ReadBigInt(&m.PrivateKeyShare, buf); err != nil {
			return errors.Wrap(err, "private key share")
		}

		// PublicKey       bitcoin.PublicKey // public key for secret
		if err := m.PublicKey.Deserialize(buf); err != nil {
			return errors.Wrap(err, "public key")
		}
	}

	// PrivateKeyShared []bool    // Private key share was received
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return errors.Wrap(err, "private key shared count")
	}

	m.PrivateKeyShared = make([]bool, 0, count)
	for i := uint32(0); i < count; i++ {
		var shared bool
		if err := binary.Read(buf, DefaultEndian, &shared); err != nil {
			return errors.Wrapf(err, "private key shared %d", i)
		}
		m.PrivateKeyShared = append(m.PrivateKeyShared, shared)
	}

	// PrivateKeyShares []big.Int // Private key shares from all members (for key recovery)
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return errors.Wrap(err, "private key shares")
	}

	m.PrivateKeyShares = make([]big.Int, 0, count)
	for i := uint32(0); i < count; i++ {
		var pks big.Int
		if err := ReadBigInt(&pks, buf); err != nil {
			return errors.Wrapf(err, "private key shares %d", i)
		}
		m.PrivateKeyShares = append(m.PrivateKeyShares, pks)
	}

	// PendingSecretShares []*SecretShare
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return errors.Wrap(err, "pending secret shares count")
	}

	m.PendingSecretShares = make([]*SecretShare, 0, count)
	for i := uint32(0); i < count; i++ {
		var ss SecretShare
		if err := ss.Deserialize(buf); err != nil {
			return errors.Wrapf(err, "pending secret shares %d", i)
		}
		m.PendingSecretShares = append(m.PendingSecretShares, &ss)
	}

	// EphemeralKeys   []*EphemeralKey
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return errors.Wrap(err, "ephemeral keys count")
	}

	m.EphemeralKeys = make([]*EphemeralKey, 0, count)
	for i := uint32(0); i < count; i++ {
		var ek EphemeralKey
		if err := ek.Deserialize(buf); err != nil {
			return errors.Wrapf(err, "ephemeral key %d", i)
		}
		m.EphemeralKeys = append(m.EphemeralKeys, &ek)
	}

	// NextEphemeralID uint64 // ID of the next ephemeral key
	if err := binary.Read(buf, DefaultEndian, &m.NextEphemeralID); err != nil {
		return errors.Wrap(err, "next ephemeral key id")
	}

	return nil
}

type SortOrdinals []big.Int

func (s SortOrdinals) Len() int {
	return len(s)
}

func (s SortOrdinals) Swap(i, j int) {
	var swp big.Int
	swp.Set(&s[i])
	s[i].Set(&s[j])
	s[j].Set(&swp)
}

func (s SortOrdinals) Less(i, j int) bool {
	return s[i].Cmp(&s[j]) < 0
}

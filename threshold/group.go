package threshold

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/pkg/errors"
	"github.com/tokenized/pkg/bitcoin"
)

type Group struct {
	ID                string
	Ordinals          map[string]big.Int // To accumulate until all are received
	Creator           string
	Secret            []byte // For encrypting messages to the group
	IsReady           bool
	Member            Member
	SignatureRequests map[bitcoin.Hash32]*SignatureRequest
}

type SignatureRequest struct {
	HashType       uint32
	Data           []byte
	EphemeralIndex uint64
	Signature      bitcoin.Signature
}

func (g *Group) HasPending() bool {
	if g.Member.HasPending() {
		return true
	}

	for _, sr := range g.SignatureRequests {
		if sr.EphemeralIndex == 0 {
			return true
		}
	}

	return false
}

func (g *Group) IsAccepted(user string) bool {
	ord, exists := g.Ordinals[user]
	if !exists {
		return false // Shouldn't happen
	}
	return ord.Cmp(bigZero) != 0
}

func (g *Group) Reset() error {
	g.SignatureRequests = make(map[bitcoin.Hash32]*SignatureRequest)
	return g.Member.Reset()
}

func (sr *SignatureRequest) Serialize(buf *bytes.Buffer) error {
	// HashType int
	if err := binary.Write(buf, DefaultEndian, sr.HashType); err != nil {
		return err
	}

	// Data []byte
	size := uint32(len(sr.Data))
	if err := binary.Write(buf, DefaultEndian, size); err != nil {
		return err
	}

	if _, err := buf.Write(sr.Data); err != nil {
		return err
	}

	// EphemeralIndex uint64
	if err := binary.Write(buf, DefaultEndian, sr.EphemeralIndex); err != nil {
		return err
	}

	// Signature      bitcoin.Signature
	if err := sr.Signature.Serialize(buf); err != nil {
		return err
	}

	return nil
}

func (sr *SignatureRequest) Deserialize(buf *bytes.Reader) error {
	// HashType int
	if err := binary.Read(buf, DefaultEndian, &sr.HashType); err != nil {
		return err
	}

	// Data []byte
	var size uint32
	if err := binary.Read(buf, DefaultEndian, &size); err != nil {
		return err
	}

	sr.Data = make([]byte, size)
	if _, err := buf.Read(sr.Data); err != nil {
		return err
	}

	// EphemeralIndex uint64
	if err := binary.Read(buf, DefaultEndian, &sr.EphemeralIndex); err != nil {
		return err
	}

	// Signature      bitcoin.Signature
	if err := sr.Signature.Deserialize(buf); err != nil {
		return err
	}

	return nil
}

func (g *Group) Serialize(buf *bytes.Buffer) error {
	// ID       string
	if err := WriteString(g.ID, buf); err != nil {
		return err
	}

	// Ordinals map[string]big.Int // To accumulate until all are received
	count := uint32(len(g.Ordinals))
	if err := binary.Write(buf, DefaultEndian, count); err != nil {
		return err
	}

	for member, ordinal := range g.Ordinals {
		if err := WriteString(member, buf); err != nil {
			return err
		}

		if err := WriteBigInt(ordinal, buf); err != nil {
			return err
		}
	}

	// Creator  string
	if err := WriteString(g.Creator, buf); err != nil {
		return err
	}

	// Secret   []byte // For encrypting messages to the group
	size := uint8(len(g.Secret))
	if err := binary.Write(buf, DefaultEndian, size); err != nil {
		return err
	}

	if _, err := buf.Write(g.Secret); err != nil {
		return err
	}

	// IsReady  bool
	if err := binary.Write(buf, DefaultEndian, g.IsReady); err != nil {
		return err
	}

	// Member   Member
	if err := g.Member.Serialize(buf); err != nil {
		return err
	}

	// SignatureRequests map[bitcoin.Hash32]SignatureRequest
	size32 := uint32(len(g.SignatureRequests))
	if err := binary.Write(buf, DefaultEndian, size32); err != nil {
		return err
	}

	for hash, sr := range g.SignatureRequests {
		if err := hash.Serialize(buf); err != nil {
			return err
		}

		if err := sr.Serialize(buf); err != nil {
			return err
		}
	}

	return nil
}

func (g *Group) Deserialize(buf *bytes.Reader) error {
	// ID       string
	if err := ReadString(&g.ID, buf); err != nil {
		return errors.Wrap(err, "id")
	}

	// Ordinals map[string]big.Int // To accumulate until all are received
	var count uint32
	if err := binary.Read(buf, DefaultEndian, &count); err != nil {
		return errors.Wrap(err, "ordinal count")
	}

	g.Ordinals = make(map[string]big.Int)
	for i := uint32(0); i < count; i++ {
		var member string
		if err := ReadString(&member, buf); err != nil {
			return errors.Wrapf(err, "ordinal %d member", i)
		}

		var ordinal big.Int
		if err := ReadBigInt(&ordinal, buf); err != nil {
			return errors.Wrapf(err, "ordinal %d", i)
		}

		g.Ordinals[member] = ordinal
	}

	// Creator  string
	if err := ReadString(&g.Creator, buf); err != nil {
		return errors.Wrap(err, "creator")
	}

	// Secret   []byte // For encrypting messages to the group
	var size uint8
	if err := binary.Read(buf, DefaultEndian, &size); err != nil {
		return errors.Wrap(err, "secret size")
	}

	g.Secret = make([]byte, size)
	if _, err := buf.Read(g.Secret); err != nil {
		return errors.Wrap(err, "secret")
	}

	// IsReady  bool
	if err := binary.Read(buf, DefaultEndian, &g.IsReady); err != nil {
		return errors.Wrap(err, "is ready")
	}

	// Member   Member
	if err := g.Member.Deserialize(buf); err != nil {
		return errors.Wrap(err, "member")
	}

	// SignatureRequests map[bitcoin.Hash32]SignatureRequest
	var size32 uint32
	if err := binary.Read(buf, DefaultEndian, &size32); err != nil {
		return errors.Wrap(err, "signature requests size")
	}

	g.SignatureRequests = make(map[bitcoin.Hash32]*SignatureRequest)
	for i := uint32(0); i < size32; i++ {
		var hash bitcoin.Hash32
		if err := hash.Deserialize(buf); err != nil {
			return errors.Wrapf(err, "signature request %d hash", i)
		}

		var sr SignatureRequest
		if err := sr.Deserialize(buf); err != nil {
			return errors.Wrapf(err, "signature request %d", i)
		}

		g.SignatureRequests[hash] = &sr
	}

	return nil
}

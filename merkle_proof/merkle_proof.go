package merkle_proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	ErrWrongMerkleRoot = errors.New("Wrong merkle root")
	ErrNotVerifiable   = errors.New("Not verifiable")
	ErrBadIndex        = errors.New("Bad Merkle Proof Index")
	ErrMissingTxID     = errors.New("Missing Transaction ID")
	ErrMissingTarget   = errors.New("Missing Target (block hash, header, merkle root)")

	Endian = binary.LittleEndian
)

type MerkleProof struct {
	Index             int // Index of tx in block
	Tx                *wire.MsgTx
	TxID              *bitcoin.Hash32
	Path              []bitcoin.Hash32
	BlockHeader       *wire.BlockHeader
	BlockHash         *bitcoin.Hash32
	MerkleRoot        *bitcoin.Hash32
	DuplicatedIndexes []int

	// Used for calculations
	root  bitcoin.Hash32
	depth int
}

// NewMerkleProof creates a new merkle proof with a specified transaction id.
func NewMerkleProof(txid bitcoin.Hash32) *MerkleProof {
	return &MerkleProof{
		root:  txid,
		TxID:  &txid,
		Index: -1,
		depth: 1,
	}
}

func (p MerkleProof) GetTxID() *bitcoin.Hash32 {
	if p.TxID != nil {
		return p.TxID
	}

	if p.Tx != nil {
		return p.Tx.TxHash()
	}

	return nil
}

func (p MerkleProof) GetBlockHash() *bitcoin.Hash32 {
	if p.BlockHash != nil {
		return p.BlockHash
	}
	if p.BlockHeader != nil {
		return p.BlockHeader.BlockHash()
	}
	return nil
}

// AddHash adds a new hash to complete a pair with the existing root at the next level in the merkle
// tree. newRoot is the new parent hash after the new hash has been added. It must be equal to the
// current root hashed with the new hash in the proper order.
func (p *MerkleProof) AddHash(hash, newRoot bitcoin.Hash32) {
	p.Path = append(p.Path, hash)
	p.depth++
	p.root = newRoot
}

// AddDuplicate adds the current root as a duplicate pair at the next level in the merkle tree.
// newRoot is the new parent hash after the new duplicate hash has been added.
func (p *MerkleProof) AddDuplicate(newRoot bitcoin.Hash32) {
	p.DuplicatedIndexes = append(p.DuplicatedIndexes, p.depth)
	p.depth++
	p.root = newRoot
}

func (p MerkleProof) Print() {
	fmt.Printf("Index : %d\n", p.Index)
	if p.TxID != nil {
		fmt.Printf("TxID : %s\n", p.TxID.String())
	}
	for _, hash := range p.Path {
		fmt.Printf("  %s\n", hash.String())
	}
}

// CalculateRoot calculates and returns the current merkle tree root hash based on the hashes
// specified in the merkle proof. This is used to verify that the transaction hash at the bottom
// provably belongs to a specified merkle tree root.
func (p MerkleProof) CalculateRoot() (bitcoin.Hash32, error) {
	index := p.Index
	layer := 1
	if p.TxID == nil {
		return bitcoin.Hash32{}, ErrMissingTxID
	}

	hash := *p.TxID
	path := p.Path
	duplicateIndexes := p.DuplicatedIndexes

	for {
		isLeft := index%2 == 0

		// Check duplicate index
		var otherHash bitcoin.Hash32
		if len(duplicateIndexes) > 0 && layer == duplicateIndexes[0] {
			otherHash = hash
			duplicateIndexes = duplicateIndexes[1:]
		} else {
			if len(path) == 0 {
				break
			}
			otherHash = path[0]
			path = path[1:]
		}

		if !isLeft && otherHash.Equal(&hash) {
			// Right hash can't be duplicate
			return bitcoin.Hash32{}, ErrBadIndex
		}

		s := sha256.New()
		if isLeft {
			s.Write(hash[:])
			s.Write(otherHash[:])
		} else {
			s.Write(otherHash[:])
			s.Write(hash[:])
		}
		hash = sha256.Sum256(s.Sum(nil)) // double SHA256

		index = index / 2
		layer++
	}

	return hash, nil
}

func (mp MerkleProof) Verify() error {
	root, err := mp.CalculateRoot()
	if err != nil {
		return errors.Wrap(err, "calculate root")
	}

	verified := false
	if mp.BlockHeader != nil {
		if !mp.BlockHeader.MerkleRoot.Equal(&root) {
			return errors.Wrap(ErrWrongMerkleRoot, "block header")
		}
		verified = true
	}

	if mp.MerkleRoot != nil {
		if !mp.MerkleRoot.Equal(&root) {
			return errors.Wrap(ErrWrongMerkleRoot, "merkle root")
		}
		verified = true
	}

	if !verified {
		return ErrNotVerifiable
	}

	return nil
}

func (mp MerkleProof) Serialize(w io.Writer) error {
	var flag uint8

	if mp.Tx != nil {
		flag = flag | 0x01
	} else if mp.TxID == nil {
		return ErrMissingTxID
	}

	if mp.BlockHeader != nil {
		flag = flag | 0x02
	} else if mp.MerkleRoot != nil {
		flag = flag | 0x04
	} else if mp.BlockHash == nil {
		return ErrMissingTarget
	}

	if err := binary.Write(w, Endian, flag); err != nil {
		return errors.Wrap(err, "flag")
	}

	if err := wire.WriteVarInt(w, 0, uint64(mp.Index)); err != nil {
		return errors.Wrap(err, "index")
	}

	if mp.Tx != nil {
		var buf bytes.Buffer
		if err := mp.Tx.Serialize(&buf); err != nil {
			return errors.Wrap(err, "serialize tx")
		}
		b := buf.Bytes()

		if err := wire.WriteVarInt(w, 0, uint64(len(b))); err != nil {
			return errors.Wrap(err, "tx length")
		}

		if _, err := w.Write(b); err != nil {
			return errors.Wrap(err, "tx")
		}
	} else {
		if err := mp.TxID.Serialize(w); err != nil {
			return errors.Wrap(err, "txid")
		}
	}

	if mp.BlockHeader != nil {
		if err := mp.BlockHeader.Serialize(w); err != nil {
			return errors.Wrap(err, "block header")
		}
	} else if mp.MerkleRoot != nil {
		if err := mp.MerkleRoot.Serialize(w); err != nil {
			return errors.Wrap(err, "merkle root")
		}
	} else {
		if err := mp.BlockHash.Serialize(w); err != nil {
			return errors.Wrap(err, "block hash")
		}
	}

	if err := wire.WriteVarInt(w, 0, uint64(len(mp.Path)+len(mp.DuplicatedIndexes))); err != nil {
		return errors.Wrap(err, "node count")
	}

	// Calculate nodes
	layer := 1
	hash := *mp.TxID
	index := mp.Index
	path := mp.Path
	duplicateIndexes := mp.DuplicatedIndexes

	for {
		isLeft := index%2 == 0

		// Check duplicate index
		var otherHash bitcoin.Hash32
		if len(duplicateIndexes) > 0 && layer == duplicateIndexes[0] {
			otherHash = hash
			duplicateIndexes = duplicateIndexes[1:]

			if err := binary.Write(w, Endian, uint8(1)); err != nil {
				return errors.Wrap(err, "node type duplicate")
			}
		} else {
			if len(path) == 0 {
				break
			}
			otherHash = path[0]
			path = path[1:]

			if err := binary.Write(w, Endian, uint8(0)); err != nil {
				return errors.Wrap(err, "node type hash")
			}
			if err := otherHash.Serialize(w); err != nil {
				return errors.Wrap(err, "node hash")
			}
		}

		if !isLeft && otherHash.Equal(&hash) {
			// Right hash can't be duplicate
			return ErrBadIndex
		}

		s := sha256.New()
		if isLeft {
			s.Write(hash[:])
			s.Write(otherHash[:])
		} else {
			s.Write(otherHash[:])
			s.Write(hash[:])
		}
		hash = sha256.Sum256(s.Sum(nil)) // double SHA256

		index = index / 2
		layer++
	}

	return nil
}

func (mp *MerkleProof) Deserialize(r io.Reader) error {
	var flag uint8
	if err := binary.Read(r, Endian, &flag); err != nil {
		return errors.Wrap(err, "flag")
	}

	index, err := wire.ReadVarInt(r, 0)
	if err != nil {
		return errors.Wrap(err, "index")
	}
	mp.Index = int(index)

	if flag&0x01 == 0x01 {
		txSize, err := wire.ReadVarInt(r, 0)
		if err != nil {
			return errors.Wrap(err, "tx length")
		}

		b := make([]byte, txSize)
		if _, err := io.ReadFull(r, b); err != nil {
			return errors.Wrap(err, "tx bytes")
		}

		tx := &wire.MsgTx{}
		if err := tx.Deserialize(bytes.NewReader(b)); err != nil {
			return errors.Wrap(err, "tx")
		}

		mp.Tx = tx
		mp.TxID = tx.TxHash()
	} else {
		txid := &bitcoin.Hash32{}
		if err := txid.Deserialize(r); err != nil {
			return errors.Wrap(err, "txid")
		}

		mp.Tx = nil
		mp.TxID = txid
	}

	if flag&0x02 == 0x02 {
		header := &wire.BlockHeader{}
		if err := header.Deserialize(r); err != nil {
			return errors.Wrap(err, "block header")
		}

		mp.BlockHeader = header
	} else if flag&0x04 == 0x04 {
		merkleRoot := &bitcoin.Hash32{}
		if err := merkleRoot.Deserialize(r); err != nil {
			return errors.Wrap(err, "merkle root")
		}

		mp.MerkleRoot = merkleRoot
	} else {
		blockHash := &bitcoin.Hash32{}
		if err := blockHash.Deserialize(r); err != nil {
			return errors.Wrap(err, "block hash")
		}

		mp.BlockHash = blockHash
	}

	nodeCount, err := wire.ReadVarInt(r, 0)
	if err != nil {
		return errors.Wrap(err, "node count")
	}

	for i := uint64(0); i < nodeCount; i++ {
		var t uint8
		if err := binary.Read(r, Endian, &t); err != nil {
			return errors.Wrap(err, "node type")
		}

		if t == 0 {
			hash := &bitcoin.Hash32{}
			if err := hash.Deserialize(r); err != nil {
				return errors.Wrapf(err, "node hash %d", i)
			}

			mp.Path = append(mp.Path, *hash)
		} else if t == 1 {
			mp.DuplicatedIndexes = append(mp.DuplicatedIndexes, int(i+1))
		} else {
			return fmt.Errorf("Unsupported node type at index %d : type %d", i, t)
		}
	}

	return nil
}

type jsonMerkleProof struct {
	Index  int    `json:"index"` // Index of tx in block
	TxOrID string `json:"txOrId"`

	TargetType string `json:"targetType,omitempty"` // "hash"(default), "header", or "merkleRoot"
	Target     string `json:"target"`

	ProofType string `json:"proofType,omitempty"` // "branch"(default) or "tree"
	Composite bool   `json:"composite,omitempty"`

	Nodes []string `json:"nodes"`
}

func (mp MerkleProof) MarshalJSON() ([]byte, error) {
	convert := jsonMerkleProof{
		Index: mp.Index,
	}

	var hash bitcoin.Hash32
	if mp.Tx != nil {
		var buf bytes.Buffer
		if err := mp.Tx.Serialize(&buf); err != nil {
			return nil, errors.Wrap(err, "tx")
		}

		convert.TxOrID = hex.EncodeToString(buf.Bytes())
		hash = *mp.Tx.TxHash()
	} else if mp.TxID != nil {
		convert.TxOrID = mp.TxID.String()
		hash = *mp.TxID
	}

	if mp.BlockHeader != nil {
		var buf bytes.Buffer
		if err := mp.BlockHeader.Serialize(&buf); err != nil {
			return nil, errors.Wrap(err, "block header")
		}

		convert.TargetType = "header"
		convert.Target = hex.EncodeToString(buf.Bytes())
	} else if mp.MerkleRoot != nil {
		convert.TargetType = "merkleRoot"
		convert.Target = mp.MerkleRoot.String()
	} else if mp.BlockHash != nil {
		convert.TargetType = ""
		convert.Target = mp.BlockHash.String()
	}
	// Calculate nodes
	layer := 1
	index := mp.Index
	path := mp.Path
	duplicateIndexes := mp.DuplicatedIndexes

	for {
		isLeft := index%2 == 0

		// Check duplicate index
		var otherHash bitcoin.Hash32
		if len(duplicateIndexes) > 0 && layer == duplicateIndexes[0] {
			otherHash = hash
			duplicateIndexes = duplicateIndexes[1:]

			convert.Nodes = append(convert.Nodes, "*")
		} else {
			if len(path) == 0 {
				break
			}
			otherHash = path[0]
			path = path[1:]

			convert.Nodes = append(convert.Nodes, otherHash.String())
		}

		if !isLeft && otherHash.Equal(&hash) {
			// Right hash can't be duplicate
			return nil, ErrBadIndex
		}

		s := sha256.New()
		if isLeft {
			s.Write(hash[:])
			s.Write(otherHash[:])
		} else {
			s.Write(otherHash[:])
			s.Write(hash[:])
		}
		hash = sha256.Sum256(s.Sum(nil)) // double SHA256

		index = index / 2
		layer++
	}

	return json.Marshal(convert)
}

func (mp MerkleProof) String() string {
	result := &bytes.Buffer{}
	result.Write([]byte(fmt.Sprintf("Tx Index : %d\n", mp.Index)))

	var hash bitcoin.Hash32
	if mp.Tx != nil {
		result.Write([]byte(mp.Tx.String()))
	} else if mp.TxID != nil {
		result.Write([]byte(fmt.Sprintf("TxID : %s\n", mp.TxID)))
	}

	if mp.BlockHeader != nil {
		result.Write([]byte(fmt.Sprintf("Header : %s\n", mp.BlockHeader.BlockHash())))
	} else if mp.MerkleRoot != nil {
		result.Write([]byte(fmt.Sprintf("Merkle Root : %s\n", mp.MerkleRoot)))
	} else if mp.BlockHash != nil {
		result.Write([]byte(fmt.Sprintf("Block Hash : %s\n", mp.BlockHash)))
	}

	// Calculate nodes
	layer := 1
	index := mp.Index
	path := mp.Path
	duplicateIndexes := mp.DuplicatedIndexes
	result.Write([]byte(fmt.Sprintf("%d Nodes\n", len(mp.DuplicatedIndexes)+len(mp.Path))))

	for {
		isLeft := index%2 == 0

		// Check duplicate index
		var otherHash bitcoin.Hash32
		if len(duplicateIndexes) > 0 && layer == duplicateIndexes[0] {
			result.Write([]byte("  *\n"))
		} else {
			if len(path) == 0 {
				break
			}
			otherHash = path[0]
			path = path[1:]

			result.Write([]byte("  " + otherHash.String() + "\n"))
		}

		if !isLeft && otherHash.Equal(&hash) {
			// Right hash can't be duplicate
			result.Write([]byte("  Invalid Duplicate\n"))
		}

		s := sha256.New()
		if isLeft {
			s.Write(hash[:])
			s.Write(otherHash[:])
		} else {
			s.Write(otherHash[:])
			s.Write(hash[:])
		}
		hash = sha256.Sum256(s.Sum(nil)) // double SHA256

		index = index / 2
		layer++
	}

	return string(result.Bytes())
}

func (mp *MerkleProof) UnmarshalJSON(data []byte) error {
	var convert jsonMerkleProof
	if err := json.Unmarshal(data, &convert); err != nil {
		return err
	}

	mp.Index = convert.Index

	if len(convert.TxOrID) == bitcoin.Hash32Size*2 {
		txid, err := bitcoin.NewHash32FromStr(convert.TxOrID)
		if err != nil {
			return errors.Wrap(err, "txid")
		}

		mp.TxID = txid
	} else {
		b, err := hex.DecodeString(convert.TxOrID)
		if err != nil {
			return errors.Wrap(err, "tx hex")
		}

		tx := &wire.MsgTx{}
		if err := tx.Deserialize(bytes.NewReader(b)); err != nil {
			return errors.Wrap(err, "tx")
		}

		mp.Tx = tx
		mp.TxID = tx.TxHash()
	}

	switch convert.TargetType {
	case "hash", "": // block hash
		hash, err := bitcoin.NewHash32FromStr(convert.Target)
		if err != nil {
			return errors.Wrap(err, "target hash")
		}

		mp.BlockHash = hash

	case "header": // block header
		b, err := hex.DecodeString(convert.Target)
		if err != nil {
			return errors.Wrap(err, "target header hex")
		}

		header := &wire.BlockHeader{}
		if err := header.Deserialize(bytes.NewReader(b)); err != nil {
			return errors.Wrap(err, "target header")
		}

		mp.BlockHeader = header

	case "merkleRoot": // merkle root hash
		hash, err := bitcoin.NewHash32FromStr(convert.Target)
		if err != nil {
			return errors.Wrap(err, "target merkle root")
		}

		mp.MerkleRoot = hash
	}

	mp.depth = 1
	mp.DuplicatedIndexes = nil
	mp.Path = nil
	for i, node := range convert.Nodes {
		if node == "*" {
			mp.DuplicatedIndexes = append(mp.DuplicatedIndexes, mp.depth)
			mp.depth++
		} else if len(node) == bitcoin.Hash32Size*2 { // *2 because hex
			hash, err := bitcoin.NewHash32FromStr(node)
			if err != nil {
				return errors.Wrapf(err, "node %d", i)
			}

			mp.Path = append(mp.Path, *hash)
			mp.depth++
		} else {
			return fmt.Errorf("Unsupported node value at index %d : %s", i, node)
		}
	}

	return nil
}

func (mp MerkleProof) MarshalText() ([]byte, error) {
	return json.Marshal(mp)
}

func (mp *MerkleProof) UnmarshalText(data []byte) error {
	return json.Unmarshal(data, mp)
}

func (mp MerkleProof) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if err := mp.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (mp *MerkleProof) UnmarshalBinary(data []byte) error {
	return mp.Deserialize(bytes.NewReader(data))
}

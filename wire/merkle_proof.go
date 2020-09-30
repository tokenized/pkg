package wire

import (
	"crypto/sha256"
	"fmt"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

// MerkleProof is a structure used to provide mathematical proof of existence of a transaction id in
// a merkle tree based on the merkle tree's root hash and a set of hashes to derive up the tree.
type MerkleProof struct {
	Index             int // Index of tx in block
	TxID              *bitcoin.Hash32
	Path              []bitcoin.Hash32
	BlockHeader       *BlockHeader
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
		return bitcoin.Hash32{}, errors.New("Missing Transaction ID")
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
			return bitcoin.Hash32{}, errors.New("Bad Merkle Proof Index")
		}

		s := sha256.New()
		if isLeft {
			s.Write(hash[:])
			s.Write(otherHash[:])
		} else {
			s.Write(otherHash[:])
			s.Write(hash[:])
		}
		hash = sha256.Sum256(s.Sum(nil))

		index = index / 2
		layer++
	}

	return hash, nil
}

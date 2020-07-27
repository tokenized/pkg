package wire

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/tokenized/pkg/bitcoin"
)

// MerkleTree is an efficient structure for calculating a merkle root hash.
type MerkleTree struct {
	layers []*merkleNodeLayer // First layer, index zero, is the lowest level of the tree.
	prune  bool
	count  int

	merkleProofs []*MerkleProof
}

// NewMerkleTree creates a new merkle tree. When prune is true only the necessary hashes to
// calculate the root will be maintained as bottom layer hashes are added to the tree.
func NewMerkleTree(prune bool) *MerkleTree {
	return &MerkleTree{
		prune: prune,
	}
}

// AddMerkleProof adds a merkle proof to be calculated with the tree.
func (t *MerkleTree) AddMerkleProof(txid bitcoin.Hash32) {
	t.merkleProofs = append(t.merkleProofs, NewMerkleProof(txid))
}

func (t MerkleTree) Print() {
	indent := ""
	for i, layer := range t.layers {
		fmt.Printf("%sLayer %d\n", indent, i+1)
		indent += "  "
		hashIndex := layer.count - len(layer.hashes) + 1
		for _, hash := range layer.hashes {
			fmt.Printf("%s%d : %s\n", indent, hashIndex, hash.String())
			hashIndex++
		}
	}
}

// AddHash adds a new hash to the bottom level of the merkle tree.
func (t *MerkleTree) AddHash(hash bitcoin.Hash32) {

	// Check merkle proofs for this hash.
	for _, mp := range t.merkleProofs {
		if mp.Index == -1 && mp.TxID != nil && mp.TxID.Equal(&hash) {
			mp.Index = t.count
			break
		}
	}

	if len(t.layers) == 0 {
		// First hash in tree
		t.layers = []*merkleNodeLayer{newMerkleNodeLayer(hash)}
		t.count = 1
		return
	}

	next := &hash
	t.count++

	// Calculate a new hash up the tree
	s := sha256.New()
	for _, layer := range t.layers {
		// Append to this row
		layer.addHash(*next)

		l := layer.len()
		if l%2 != 0 {
			return // Above layers do not need to be updated.
		}

		// Even number of hashes in layer. Hash last 2 hashes together to add to layer above.
		next = t.processProofsLayer(s, layer.nextLastHash(), *next, false)
		if t.prune {
			layer.clear() // Clear out hashes that aren't needed anymore
		}
	}

	// Append new layer
	t.layers = append(t.layers, newMerkleNodeLayer(*next))
}

// RootHash calculates and returns the root hash of the merkle tree.
func (t *MerkleTree) RootHash() bitcoin.Hash32 {
	if t.count == 0 {
		return bitcoin.Hash32{} // zero hash should never be valid
	}
	if t.count == 1 {
		return t.layers[0].lastHash()
	}

	// Check for odd length layer to calculate up from
	var next *bitcoin.Hash32
	s := sha256.New()
	for d, layer := range t.layers {
		l := layer.len()

		if next != nil {
			// Odd layer was below this. So keep calculating up.
			if l%2 == 0 {
				// Layer will be odd length with new hash, so hash next hash with itself
				next = processLayer(s, *next, *next)
				continue
			}

			// Hash last hash with next hash
			next = processLayer(s, layer.lastHash(), *next)
			continue
		}

		if l%2 != 0 {
			if l == 1 && d == len(t.layers)-1 {
				// Last layer so this is the root hash
				return layer.lastHash()
			}

			// Odd length layer. Calculate from here up.
			l := layer.lastHash()
			next = processLayer(s, l, l)
		}
	}

	if next == nil {
		return bitcoin.Hash32{} // zero hash should never be valid
	}
	return *next
}

// processLayer adds a new layer the any merkle proofs appropriate while continuing to
// calculate the root hash.
func processLayer(hasher hash.Hash, l, r bitcoin.Hash32) *bitcoin.Hash32 {
	hasher.Write(l[:])
	hasher.Write(r[:])
	b := hasher.Sum(nil)
	hasher.Reset()
	newHash := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
	return &newHash
}

// FinalizeMerkleProofs finalizes the merkle proofs and returns the root hash and the merkle proofs.
func (t MerkleTree) FinalizeMerkleProofs() (bitcoin.Hash32, []*MerkleProof) {
	// Finalize merkle proofs
	if t.count == 0 {
		return bitcoin.Hash32{}, nil // zero hash should never be valid
	}
	if t.count == 1 {
		return t.layers[0].lastHash(), t.merkleProofs
	}

	// Check for odd length layer to calculate up from
	var next *bitcoin.Hash32
	s := sha256.New()
	for d, layer := range t.layers {
		l := layer.len()

		if next != nil {
			// Odd layer was below this. So keep calculating up.
			if l%2 == 0 {
				// Layer will be odd length with new hash, so hash next hash with itself
				next = t.processProofsLayer(s, *next, *next, true)

				continue
			}

			// Hash left (last) hash with next hash
			next = t.processProofsLayer(s, layer.lastHash(), *next, false)

			continue
		}

		if l%2 != 0 {
			if l == 1 && d == len(t.layers)-1 {
				// Last layer so this is the root hash
				return layer.lastHash(), t.merkleProofs
			}

			// Odd length layer. Calculate from here up.
			duplicate := layer.lastHash()
			next = t.processProofsLayer(s, duplicate, duplicate, true)
		}
	}

	if next == nil {
		return bitcoin.Hash32{}, nil // zero hash should never be valid
	}

	return *next, t.merkleProofs
}

// processProofsLayer adds a new layer the any merkle proofs appropriate while continuing to
// calculate the root hash.
func (t MerkleTree) processProofsLayer(hasher hash.Hash, l, r bitcoin.Hash32, isDuplicate bool) *bitcoin.Hash32 {
	hasher.Write(l[:])
	hasher.Write(r[:])
	b := hasher.Sum(nil)
	hasher.Reset()
	newHash := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256

	// Check active merkle proofs for next element in path
	for _, mp := range t.merkleProofs {
		if mp.Index == -1 {
			continue // txid not found yet
		}

		if isDuplicate {
			if mp.root.Equal(&l) {
				mp.AddDuplicate(newHash)
			}
			continue
		}

		if mp.root.Equal(&l) {
			mp.AddHash(r, newHash)
		} else if mp.root.Equal(&r) {
			mp.AddHash(l, newHash)
		}
	}

	return &newHash
}

// merkleNodeLayer is a level of hashes in a merkle tree.
type merkleNodeLayer struct {
	hashes []bitcoin.Hash32
	count  int
}

// newMerkleNodeLayer creates a new merkle node layer that starts with the specified hash.
func newMerkleNodeLayer(hash bitcoin.Hash32) *merkleNodeLayer {
	return &merkleNodeLayer{
		hashes: []bitcoin.Hash32{hash},
		count:  1,
	}
}

func (l *merkleNodeLayer) addHash(hash bitcoin.Hash32) {
	l.hashes = append(l.hashes, hash)
	l.count++
}

func (l *merkleNodeLayer) clear() {
	l.hashes = nil
}

// len returns the number of hashes in the merkle node layer.
func (l merkleNodeLayer) len() int {
	return l.count
}

// lastHash returns the last hash in the merkle node layer.
func (l merkleNodeLayer) lastHash() bitcoin.Hash32 {
	return l.hashes[len(l.hashes)-1]
}

// nextLastHash returns the hash previous to the last hash in the merkle node layer.
func (l merkleNodeLayer) nextLastHash() bitcoin.Hash32 {
	return l.hashes[len(l.hashes)-2]
}

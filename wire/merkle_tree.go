package wire

import (
	"crypto/sha256"
	"fmt"

	"github.com/tokenized/pkg/bitcoin"
)

// MerkleTree is an efficient structure for calculating a merkle root hash.
type MerkleTree struct {
	layers []*merkleNodeLayer // First layer, index zero, is the lowest level of the tree.
	prune  bool
	count  int

	merkleProofs []*MerkleProof
}

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

// AddHash adds a new hash to the merkle tree.
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

	next := hash
	t.count++

	// Calculate a new hash up the tree
	s := sha256.New()
	for _, layer := range t.layers {
		// Append to this row
		layer.addHash(next)

		l := layer.len()
		if l%2 != 0 {
			return // Above layers do not need to be updated.
		}

		left := layer.nextLastHash()
		right := next

		// Even number of hashes in layer. Hash last 2 hashes together to add to layer above.
		s.Write(left[:])
		s.Write(right[:])
		b := s.Sum(nil)
		s.Reset()
		next = bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
		if t.prune {
			layer.clear() // Clear out hashes that aren't needed anymore
		}

		// Check active merkle proofs for next element in path
		for _, mp := range t.merkleProofs {
			if mp.Index == -1 {
				continue // txid not found yet
			}

			if mp.root.Equal(&left) {
				mp.AddHash(right, next)
			} else if mp.root.Equal(&right) {
				mp.AddHash(left, next)
			}
		}
	}

	// Append new layer
	t.layers = append(t.layers, newMerkleNodeLayer(next))
}

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
				s.Write(next[:])
				s.Write(next[:])
				b := s.Sum(nil)
				s.Reset()
				h := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
				next = &h
				continue
			}

			// hash last hash with next hash
			s.Write(layer.lastBytes())
			s.Write(next[:])
			b := s.Sum(nil)
			s.Reset()
			h := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			next = &h
			continue
		}

		if l%2 != 0 {
			if l == 1 && d == len(t.layers)-1 {
				// Last layer so this is the root hash
				return layer.lastHash()
			}

			// Odd length layer. Calculate from here up.
			s.Write(layer.lastBytes())
			s.Write(layer.lastBytes())
			b := s.Sum(nil)
			s.Reset()
			h := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			next = &h
		}
	}

	if next == nil {
		return bitcoin.Hash32{} // zero hash should never be valid
	}
	return *next
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
				left := next

				// Layer will be odd length with new hash, so hash next hash with itself
				s.Write(next[:])
				s.Write(next[:])
				b := s.Sum(nil)
				s.Reset()
				h := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
				next = &h

				// Check active merkle proofs for next element in path
				for _, mp := range t.merkleProofs {
					if mp.Index == -1 {
						continue // txid not found yet
					}

					if mp.root.Equal(left) {
						mp.AddDuplicate(*next)
					}
				}

				continue
			}

			left := layer.lastHash()
			right := next

			// hash left (last) hash with next hash
			s.Write(left[:])
			s.Write(right[:])
			b := s.Sum(nil)
			s.Reset()
			h := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			next = &h

			// Check active merkle proofs for next element in path
			for _, mp := range t.merkleProofs {
				if mp.Index == -1 {
					continue // txid not found yet
				}

				if mp.root.Equal(&left) {
					mp.AddHash(*right, *next)
				} else if mp.root.Equal(right) {
					mp.AddHash(left, *next)
				}
			}

			continue
		}

		if l%2 != 0 {
			if l == 1 && d == len(t.layers)-1 {
				// Last layer so this is the root hash
				return layer.lastHash(), t.merkleProofs
			}

			duplicate := layer.lastHash()

			// Odd length layer. Calculate from here up.
			s.Write(duplicate[:])
			s.Write(duplicate[:])
			b := s.Sum(nil)
			s.Reset()
			h := bitcoin.Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			next = &h

			// Check active merkle proofs for next element in path
			for _, mp := range t.merkleProofs {
				if mp.Index == -1 {
					continue // txid not found yet
				}

				if mp.root.Equal(&duplicate) {
					mp.AddDuplicate(*next)
				}
			}
		}
	}

	if next == nil {
		return bitcoin.Hash32{}, nil // zero hash should never be valid
	}

	return *next, t.merkleProofs
}

type merkleNodeLayer struct {
	hashes []bitcoin.Hash32
	count  int
}

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

func (l merkleNodeLayer) len() int {
	return l.count
}

func (l merkleNodeLayer) lastHash() bitcoin.Hash32 {
	return l.hashes[len(l.hashes)-1]
}

func (l merkleNodeLayer) nextLastHash() bitcoin.Hash32 {
	return l.hashes[len(l.hashes)-2]
}

func (l merkleNodeLayer) lastBytes() []byte {
	return l.hashes[len(l.hashes)-1][:]
}

func (l merkleNodeLayer) nextLastBytes() []byte {
	return l.hashes[len(l.hashes)-2][:]
}

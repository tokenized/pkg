package bitcoin

import (
	"crypto/sha256"
)

// MerkleTree is an efficient structure for calculating a merkle root hash.
type MerkleTree struct {
	layers []merkleNodeLayer // First layer, index zero, is the lowest level of the tree.
	prune  bool
	count  int
}

type merkleNodeLayer []merkleNode

type merkleNode struct {
	hash  Hash32
	index int
}

func NewMerkleTree(prune bool) *MerkleTree {
	return &MerkleTree{
		prune: prune,
	}
}

// AddHash adds a new hash to the merkle tree.
func (t *MerkleTree) AddHash(hash Hash32) {

	if len(t.layers) == 0 {
		// First hash in tree
		t.layers = []merkleNodeLayer{
			merkleNodeLayer{
				merkleNode{
					hash:  hash,
					index: 0,
				},
			},
		}

		t.count = 1
		return
	}

	// Append hash to bottom layer
	t.layers[0] = append(t.layers[0], merkleNode{
		hash:  hash,
		index: len(t.layers[0]),
	})

	// If there are an even number of hashes then calculate the parent hashes in the tree.
	if len(t.layers[0])%2 == 0 {
		// Calculate a new hash up the tree
		var next Hash32
		s := sha256.New()
		for i, layer := range t.layers {
			if i > 0 {
				// Append to this row
				layer = append(layer, merkleNode{
					hash:  next,
					index: len(layer),
				})
			}

			l := len(layer)

			if l%2 == 0 {
				// hash last 2 hashes together
				s.Write(layer[l-2].hash[:])
				s.Write(layer[l-1].hash[:])
				b := s.Sum(nil)
				s.Reset()
				next = Hash32(sha256.Sum256(b)) // Sum again for double SHA256
				if t.prune {
					layer = nil // Clear out hashes that aren't needed anymore
				}
			} else {
				// hash last hash with itself
				s.Write(layer[l-1].hash[:])
				s.Write(layer[l-1].hash[:])
				b := s.Sum(nil)
				s.Reset()
				next = Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			}

			t.layers[i] = layer
		}

		// Append new layer
		t.layers = append(t.layers, merkleNodeLayer{
			merkleNode{
				hash:  next,
				index: 0,
			},
		})
	}

	t.count++
}

func (t *MerkleTree) RootHash() Hash32 {
	if t.count == 0 {
		return Hash32{} // zero hash
	}
	if t.count == 1 {
		return t.layers[0][0].hash
	}

	if t.count%2 == 0 {
		// Even hash already calculated
		d := len(t.layers)
		return t.layers[d-1][len(t.layers[d-1])-1].hash
	}

	// Calculate odd length hash
	next := t.layers[0][len(t.layers[0])-1].hash
	s := sha256.New()
	for _, layer := range t.layers {
		l := len(layer)

		if l%2 == 0 {
			// hash next hash with itself
			s.Write(next[:])
			s.Write(next[:])
			b := s.Sum(nil)
			s.Reset()
			next = Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			continue
		}

		// hash last hash with next hash
		s.Write(layer[l-1].hash[:])
		s.Write(next[:])
		b := s.Sum(nil)
		s.Reset()
		next = Hash32(sha256.Sum256(b)) // Sum again for double SHA256
	}

	return next
}

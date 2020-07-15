package bitcoin

import (
	"crypto/sha256"
)

// MerkleTree is an efficient structure for calculating a merkle root hash.
type MerkleTree struct {
	layers []*merkleNodeLayer // First layer, index zero, is the lowest level of the tree.
	prune  bool
	count  int
}

type merkleNodeLayer struct {
	nodes []Hash32
	count int
}

func NewMerkleTree(prune bool) *MerkleTree {
	return &MerkleTree{
		prune: prune,
	}
}

func newMerkleNodeLayer(hash Hash32) *merkleNodeLayer {
	return &merkleNodeLayer{
		nodes: []Hash32{hash},
		count: 1,
	}
}

func (l *merkleNodeLayer) addHash(hash Hash32) {
	l.nodes = append(l.nodes, hash)
	l.count++
}

func (l *merkleNodeLayer) clear() {
	l.nodes = nil
}

func (l merkleNodeLayer) len() int {
	return l.count
}

func (l merkleNodeLayer) lastHash() Hash32 {
	return l.nodes[len(l.nodes)-1]
}

func (l merkleNodeLayer) lastBytes() []byte {
	return l.nodes[len(l.nodes)-1][:]
}

func (l merkleNodeLayer) nextLast() []byte {
	return l.nodes[len(l.nodes)-2][:]
}

// AddHash adds a new hash to the merkle tree.
func (t *MerkleTree) AddHash(hash Hash32) {

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

		// Even number of hashes in layer. Hash last 2 hashes together to add to layer above.
		s.Write(layer.nextLast())
		s.Write(layer.lastBytes())
		b := s.Sum(nil)
		s.Reset()
		next = Hash32(sha256.Sum256(b)) // Sum again for double SHA256
		if t.prune {
			layer.clear() // Clear out hashes that aren't needed anymore
		}
	}

	// Append new layer
	t.layers = append(t.layers, newMerkleNodeLayer(next))
}

func (t *MerkleTree) RootHash() Hash32 {
	if t.count == 0 {
		return Hash32{} // zero hash
	}
	if t.count == 1 {
		return t.layers[0].lastHash()
	}

	// Check for odd length layer to calculate up from
	var next *Hash32
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
				h := Hash32(sha256.Sum256(b)) // Sum again for double SHA256
				next = &h
				continue
			}

			// hash last hash with next hash
			s.Write(layer.lastBytes())
			s.Write(next[:])
			b := s.Sum(nil)
			s.Reset()
			h := Hash32(sha256.Sum256(b)) // Sum again for double SHA256
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
			h := Hash32(sha256.Sum256(b)) // Sum again for double SHA256
			next = &h
		}
	}

	if next == nil {
		return Hash32{} // zero hash
	}
	return *next
}

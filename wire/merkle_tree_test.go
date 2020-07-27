package wire

import (
	"crypto/rand"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
)

var blocks = []struct {
	name     string
	hashes   []string
	rootHash string
}{
	{ // Merkle test (Bitcoin SV Block 570,666) - even tx count test
		name: "Block 570,666",
		hashes: []string{
			"9e7447228f71e65ac0bcce3898f3a9a3e3e3ef89f1a07045f9565d8ef8da5c6d",
			"26d732c0e4657e93b7143dcf7e25e93f61f630a5d465e3368f69708c57f69dd7",
			"5fe54352f91acb9a2aff9b1271a296331d3bed9867be430f21ee19ef054efb0c",
			"496eae8dbe3968884296b3bf078a6426de459afd710e8713645955d9660afad1",
			"5809a72ee084625365067ff140c0cfedd05adc7a8a5040399409e9cca8ab4255",
			"2a7927d2f953770fcd899902975ad7067a1adef3f572d5d8d196bfe0cbc7d954",
		},
		rootHash: "5f7b966b938cdb0dbf08a6bcd53e8854a6583b211452cf5dd5214dddd286e923",
	},
	{ // Merkle test (Bitcoin SV Block 642,818) - Single tx special calculation
		name: "Block 642,818",
		hashes: []string{
			"529e5d20ce6b8948af887fbaaa011b50a4ac5c6c4ae4d228dd7d5f6b1fe8cf29",
		},
		rootHash: "529e5d20ce6b8948af887fbaaa011b50a4ac5c6c4ae4d228dd7d5f6b1fe8cf29",
	},
	{ // Merkle test (Bitcoin SV Block 642,744) - even tx count test
		name: "Block 642,744",
		hashes: []string{
			"06bcb5b1b769f989e6aae30ca6aa0eb3e526f61a785ac2ff8d093afe5158a8ce",
			"95d06e3021bc8a55690910c56a452ecbe501f9789b8f371b6bc870fa9f1e0e4d",
		},
		rootHash: "ce89b19c927195854ffa91e64abef03f98254fdf5d3c2daeb06f1d3d3490207f",
	},
	{ // Merkle test (Bitcoin SV Block 642,743) - odd tx count test
		name: "Block 642,743",
		hashes: []string{
			"2c9153023ab9db8418120cf7d63087dec5d52d7fcad6f2fd3d0f2d7bd6def7a2",
			"f8b2d36de8172fb6bdebfc9d8731dfb8ce52f4e839fc20883b972578fdef7d52",
			"da2d7274b9b11c77a13ab800766e70dba21d8116ec5b4e9dc6642a8d7ddb104c",
			"6a2eaacf13dec0602b236ec171c68cdc936e88e7ea3803abc80766ce38eff3d0",
			"15be8f8d601ead7e91f45690522b7ccb08f2c7859aeecd9d8f86c8a8ac9c7161",
		},
		rootHash: "c29f39fe47585ad00911347de8b82e1cf7119e110ec16acc11b1697857a0e789",
	},
	{ // Merkle test (Bitcoin SV Block 643,738) - 3 tx count test
		name: "Block 643,738",
		hashes: []string{
			"b3bfc2bbc4cd22ed5a94de2312702765170d1a72ce20d398a76e8a7d456fe13f",
			"9df955f11dd4325172bd72e479f689d02e6a86acbdb1508cffb6e477d26c434a",
			"1e55cd18f8d9cfe91635a6de8a947aafddf8265de47c5329f823ab3c0162744a",
		},
		rootHash: "0d166b6b0423865b10adec80e55be4288bd9b3ab18cf529d624dfd9a2adb971b",
	},
}

func TestMerkleTree(t *testing.T) {
	for _, tt := range blocks {
		t.Run(tt.name, func(t *testing.T) {
			// Full --------------------------------------------------------------------------------
			mt := NewMerkleTree(false)

			for _, s := range tt.hashes {
				hash, err := bitcoin.NewHash32FromStr(s)
				if err != nil {
					t.Fatalf("Failed to parse hash string : %s", err)
				}

				mt.AddHash(*hash)
			}

			result, err := bitcoin.NewHash32FromStr(tt.rootHash)
			if err != nil {
				t.Fatalf("Failed to parse result hash string : %s", err)
			}

			rh := mt.RootHash()
			if !rh.Equal(result) {
				t.Errorf("Wrong result : \n  got  : %s\n  want : %s", rh.String(), result.String())
			}

			// Pruned ------------------------------------------------------------------------------
			pmt := NewMerkleTree(true)

			for _, s := range tt.hashes {
				hash, err := bitcoin.NewHash32FromStr(s)
				if err != nil {
					t.Fatalf("Failed to parse hash string : %s", err)
				}

				pmt.AddHash(*hash)
			}

			result, err = bitcoin.NewHash32FromStr(tt.rootHash)
			if err != nil {
				t.Fatalf("Failed to parse result hash string : %s", err)
			}

			rh = pmt.RootHash()
			if !rh.Equal(result) {
				t.Errorf("Wrong result (pruned) : \n  got  : %s\n  want : %s", rh.String(),
					result.String())
			}
		})
	}
}

func TestMerkleProof(t *testing.T) {
	tests := []struct {
		name       string
		blockIndex int
		txid       string
		result     []string
	}{
		{
			name:       "Block 570,666 tx 1",
			blockIndex: 0,
			txid:       "9e7447228f71e65ac0bcce3898f3a9a3e3e3ef89f1a07045f9565d8ef8da5c6d",
			result: []string{
				"26d732c0e4657e93b7143dcf7e25e93f61f630a5d465e3368f69708c57f69dd7",
				"7535e2e8cb59b8b1980b166fe3accf585052979d8c0ef981276808e174c122f1",
				"464e356b90742a03b17acd32c98b80da64d42199dc7da77a0e80ff94a3e7d62b",
			},
		},
		{
			name:       "Block 570,666 tx 2",
			blockIndex: 0,
			txid:       "26d732c0e4657e93b7143dcf7e25e93f61f630a5d465e3368f69708c57f69dd7",
			result: []string{
				"9e7447228f71e65ac0bcce3898f3a9a3e3e3ef89f1a07045f9565d8ef8da5c6d",
				"7535e2e8cb59b8b1980b166fe3accf585052979d8c0ef981276808e174c122f1",
				"464e356b90742a03b17acd32c98b80da64d42199dc7da77a0e80ff94a3e7d62b",
			},
		},
		{
			name:       "Block 570,666 tx 4",
			blockIndex: 0,
			txid:       "496eae8dbe3968884296b3bf078a6426de459afd710e8713645955d9660afad1",
			result: []string{
				"5fe54352f91acb9a2aff9b1271a296331d3bed9867be430f21ee19ef054efb0c",
				"2a9aec684e292359f216a4f682fa768dd246bfd62cfee9f10843891ee24ec3a0",
				"464e356b90742a03b17acd32c98b80da64d42199dc7da77a0e80ff94a3e7d62b",
			},
		},
		{
			name:       "Block 570,666 last tx",
			blockIndex: 0,
			txid:       "2a7927d2f953770fcd899902975ad7067a1adef3f572d5d8d196bfe0cbc7d954",
			result: []string{
				"5809a72ee084625365067ff140c0cfedd05adc7a8a5040399409e9cca8ab4255",
				"1797f5f70c2e1af71420b91339ed05c71e2c65e4af8ed5f69fa990369bbe47ce",
			},
		},
		{
			name:       "Block 642,818 only tx",
			blockIndex: 1,
			txid:       "529e5d20ce6b8948af887fbaaa011b50a4ac5c6c4ae4d228dd7d5f6b1fe8cf29",
			result:     []string{},
		},
		{
			name:       "Block 643,738 tx 1",
			blockIndex: 4,
			txid:       "b3bfc2bbc4cd22ed5a94de2312702765170d1a72ce20d398a76e8a7d456fe13f",
			result: []string{
				"9df955f11dd4325172bd72e479f689d02e6a86acbdb1508cffb6e477d26c434a",
				"7d9d5ff29f73a93998b670ef0a7237363d684555ccb84c900a6267e3cea98ad3",
			},
		},
		{
			name:       "Block 643,738 tx 2",
			blockIndex: 4,
			txid:       "9df955f11dd4325172bd72e479f689d02e6a86acbdb1508cffb6e477d26c434a",
			result: []string{
				"b3bfc2bbc4cd22ed5a94de2312702765170d1a72ce20d398a76e8a7d456fe13f",
				"7d9d5ff29f73a93998b670ef0a7237363d684555ccb84c900a6267e3cea98ad3",
			},
		},
		{
			name:       "Block 643,738 tx 3",
			blockIndex: 4,
			txid:       "1e55cd18f8d9cfe91635a6de8a947aafddf8265de47c5329f823ab3c0162744a",
			result: []string{
				"f2bf417fcc7b0b3b31a36719775f180253927983701e6bbe85572450c6e14350",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txid, err := bitcoin.NewHash32FromStr(tt.txid)
			if err != nil {
				t.Fatalf("Failed to parse txid hash string : %s", err)
			}

			result := make([]bitcoin.Hash32, 0, len(tt.result))
			for _, r := range tt.result {
				rh, err := bitcoin.NewHash32FromStr(r)
				if err != nil {
					t.Fatalf("Failed to parse result hash string : %s", err)
				}
				result = append(result, *rh)
			}

			blockRootHash, err := bitcoin.NewHash32FromStr(blocks[tt.blockIndex].rootHash)
			if err != nil {
				t.Fatalf("Failed to parse result hash string : %s", err)
			}

			// Full --------------------------------------------------------------------------------
			mt := NewMerkleTree(false)
			mt.AddMerkleProof(*txid)

			for _, s := range blocks[tt.blockIndex].hashes {
				hash, err := bitcoin.NewHash32FromStr(s)
				if err != nil {
					t.Fatalf("Failed to parse hash string : %s", err)
				}

				mt.AddHash(*hash)
			}

			rh, merkleProofs := mt.FinalizeMerkleProofs()

			if !blockRootHash.Equal(&rh) {
				t.Errorf("Wrong full root hash : \n  got  %s\n  want %s", blockRootHash.String(),
					rh.String())
			}

			if len(merkleProofs) != 1 {
				t.Fatalf("Wrong merkle proof count : got %d, want %d", len(merkleProofs), 1)
			}

			mprh, err := merkleProofs[0].CalculateRoot()
			if err != nil {
				t.Fatalf("Failed to calculate merkle proof root : %s", err)
			}
			if !mprh.Equal(&rh) {
				t.Errorf("Wrong merkle proof root hash : \n  got  %s\n  want %s", mprh.String(),
					rh.String())
			}

			if len(merkleProofs[0].Path) != len(result) {
				t.Fatalf("Wrong merkle proof path length : got %d, want %d",
					len(merkleProofs[0].Path), len(result))
			}
			for i, hash := range merkleProofs[0].Path {
				if !hash.Equal(&result[i]) {
					t.Errorf("Wrong Path Hash %d :\n  got  %s\n  want %s", i, hash.String(),
						result[i].String())
				}
			}

			// Pruned ------------------------------------------------------------------------------
			pmt := NewMerkleTree(true)
			pmt.AddMerkleProof(*txid)

			for _, s := range blocks[tt.blockIndex].hashes {
				hash, err := bitcoin.NewHash32FromStr(s)
				if err != nil {
					t.Fatalf("Failed to parse hash string : %s", err)
				}

				pmt.AddHash(*hash)
			}

			rh, merkleProofs = pmt.FinalizeMerkleProofs()

			if !blockRootHash.Equal(&rh) {
				t.Errorf("Wrong pruned root hash : \n  got  %s\n  want %s", blockRootHash.String(),
					rh.String())
			}

			if len(merkleProofs) != 1 {
				t.Fatalf("Wrong merkle proof count : got %d, want %d", len(merkleProofs), 1)
			}

			mprh, err = merkleProofs[0].CalculateRoot()
			if err != nil {
				t.Fatalf("Failed to calculate merkle proof root : %s", err)
			}
			if !mprh.Equal(&rh) {
				t.Errorf("Wrong merkle proof root hash : \n  got  %s\n  want %s", mprh.String(),
					rh.String())
			}

			if len(merkleProofs[0].Path) != len(result) {
				t.Fatalf("Wrong merkle proof path length : got %d, want %d",
					len(merkleProofs[0].Path), len(result))
			}
			for i, hash := range merkleProofs[0].Path {
				if !hash.Equal(&result[i]) {
					t.Errorf("Wrong Path Hash %d :\n  got  %s\n  want %s", i, hash.String(),
						result[i].String())
				}
			}
		})
	}
}

func BenchmarkMerkleTree(b *testing.B) {
	hashes := make([]bitcoin.Hash32, b.N)
	for i := range hashes {
		rand.Read(hashes[i][:])
	}

	b.ResetTimer()

	mt := NewMerkleTree(false)
	mt.AddMerkleProof(hashes[b.N/2])
	for _, hash := range hashes {
		mt.AddHash(hash)
	}

	mt.RootHash()
	mt.FinalizeMerkleProofs()
}

func BenchmarkMerkleTreePruned(b *testing.B) {
	hashes := make([]bitcoin.Hash32, b.N)
	for i := range hashes {
		rand.Read(hashes[i][:])
	}

	b.ResetTimer()

	mt := NewMerkleTree(true)
	mt.AddMerkleProof(hashes[b.N/2])
	for _, hash := range hashes {
		mt.AddHash(hash)
	}

	mt.RootHash()
	mt.FinalizeMerkleProofs()
}

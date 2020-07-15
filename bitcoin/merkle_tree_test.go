package bitcoin

import (
	"testing"
)

func TestMerkleTree(t *testing.T) {
	tests := []struct {
		name   string
		hashes []string
		result string
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
			result: "5f7b966b938cdb0dbf08a6bcd53e8854a6583b211452cf5dd5214dddd286e923",
		},
		{ // Merkle test (Bitcoin SV Block 642,818) - Single tx special calculation
			name: "Block 642,818",
			hashes: []string{
				"529e5d20ce6b8948af887fbaaa011b50a4ac5c6c4ae4d228dd7d5f6b1fe8cf29",
			},
			result: "529e5d20ce6b8948af887fbaaa011b50a4ac5c6c4ae4d228dd7d5f6b1fe8cf29",
		},
		{ // Merkle test (Bitcoin SV Block 642,744) - even tx count test
			name: "Block 642,744",
			hashes: []string{
				"06bcb5b1b769f989e6aae30ca6aa0eb3e526f61a785ac2ff8d093afe5158a8ce",
				"95d06e3021bc8a55690910c56a452ecbe501f9789b8f371b6bc870fa9f1e0e4d",
			},
			result: "ce89b19c927195854ffa91e64abef03f98254fdf5d3c2daeb06f1d3d3490207f",
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
			result: "c29f39fe47585ad00911347de8b82e1cf7119e110ec16acc11b1697857a0e789",
		},
		{ // Merkle test (Bitcoin SV Block 643,738) - 3 tx count test
			name: "Block 643,738",
			hashes: []string{
				"b3bfc2bbc4cd22ed5a94de2312702765170d1a72ce20d398a76e8a7d456fe13f",
				"9df955f11dd4325172bd72e479f689d02e6a86acbdb1508cffb6e477d26c434a",
				"1e55cd18f8d9cfe91635a6de8a947aafddf8265de47c5329f823ab3c0162744a",
			},
			result: "0d166b6b0423865b10adec80e55be4288bd9b3ab18cf529d624dfd9a2adb971b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Full --------------------------------------------------------------------------------
			mt := NewMerkleTree(false)

			for _, s := range tt.hashes {
				hash, err := NewHash32FromStr(s)
				if err != nil {
					t.Fatalf("Failed to parse hash string : %s", err)
				}

				mt.AddHash(*hash)
			}

			result, err := NewHash32FromStr(tt.result)
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
				hash, err := NewHash32FromStr(s)
				if err != nil {
					t.Fatalf("Failed to parse hash string : %s", err)
				}

				pmt.AddHash(*hash)
			}

			result, err = NewHash32FromStr(tt.result)
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

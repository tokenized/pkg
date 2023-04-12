package merkle_proof

import (
	"math/rand"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

func MockMerkleProofWithTx(tx *wire.MsgTx, txCount int) *MerkleProof {
	tree := NewMerkleTree(true)
	txid := *tx.TxHash()
	tree.AddMerkleProof(txid)

	offset := rand.Intn(txCount)
	for i := 0; i < txCount; i++ {
		if i == offset {
			tree.AddHash(txid)
			continue
		}

		var otherTxid bitcoin.Hash32
		rand.Read(otherTxid[:])
		tree.AddHash(otherTxid)
	}

	_, proofs := tree.FinalizeMerkleProofs()
	proofs[0].Tx = tx
	proofs[0].BlockHash = &bitcoin.Hash32{}
	rand.Read(proofs[0].BlockHash[:])
	return proofs[0]
}

func MockMerkleProofWithTxID(txid bitcoin.Hash32, txCount int) *MerkleProof {
	tree := NewMerkleTree(true)
	tree.AddMerkleProof(txid)

	offset := rand.Intn(txCount)
	for i := 0; i < txCount; i++ {
		if i == offset {
			tree.AddHash(txid)
			continue
		}

		var otherTxid bitcoin.Hash32
		rand.Read(otherTxid[:])
		tree.AddHash(otherTxid)
	}

	_, proofs := tree.FinalizeMerkleProofs()
	proofs[0].TxID = &txid
	proofs[0].BlockHash = &bitcoin.Hash32{}
	rand.Read(proofs[0].BlockHash[:])
	return proofs[0]
}

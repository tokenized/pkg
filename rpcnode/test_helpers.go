package rpcnode

import (
	"context"
	"fmt"
	"sync"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/txbuilder"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

// MockFundingUTXO generates a fake funding output for input to another tx.
func MockFundingUTXO(ctx context.Context, rpc *MockRpcNode,
	value uint64) (bitcoin.Key, bitcoin.UTXO) {

	tx := wire.NewMsgTx(1)

	key, address := MockKey()

	script, err := address.LockingScript()
	if err != nil {
		panic(err)
	}
	tx.AddTxOut(wire.NewTxOut(value, script))

	if err := rpc.SaveTX(ctx, tx); err != nil {
		panic(err)
	}

	result := bitcoin.UTXO{
		Hash:          *tx.TxHash(),
		Index:         0,
		Value:         value,
		LockingScript: script,
	}

	return key, result
}

// MockPaymentTx generates a tx paying to a specific address with inputs mocked in rpc.
func MockPaymentTx(ctx context.Context, rpc *MockRpcNode, value uint64,
	address bitcoin.RawAddress) *wire.MsgTx {

	tx := txbuilder.NewTxBuilder(1.0, 1.0)

	// Create mock UTXO to fund tx
	inputKey, utxo := MockFundingUTXO(ctx, rpc, value+500)

	// Empty signature script
	tx.AddInputUTXO(utxo)

	tx.AddPaymentOutput(address, value, false)

	if err := tx.Sign([]bitcoin.Key{inputKey}); err != nil {
		panic(err)
	}

	return tx.MsgTx
}

// MockKey generates a random key and raw address.
func MockKey() (bitcoin.Key, bitcoin.RawAddress) {
	key, err := bitcoin.GenerateKey(bitcoin.InvalidNet)
	if err != nil {
		panic(err)
	}

	ra, err := key.RawAddress()
	if err != nil {
		panic(err)
	}

	return key, ra
}

// MockRpcNode can be used in tests.
type MockRpcNode struct {
	txs  map[bitcoin.Hash32]*wire.MsgTx
	lock sync.Mutex
}

func NewMockRpcNode() *MockRpcNode {
	result := MockRpcNode{
		txs: make(map[bitcoin.Hash32]*wire.MsgTx),
	}
	return &result
}

func (r *MockRpcNode) SaveTX(ctx context.Context, tx *wire.MsgTx) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.txs[*tx.TxHash()] = tx.Copy()
	return nil
}

func (r *MockRpcNode) GetTX(ctx context.Context, txid *bitcoin.Hash32) (*wire.MsgTx, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	tx, ok := r.txs[*txid]
	if ok {
		return tx, nil
	}
	return nil, errors.New("Couldn't find tx in r")
}

func (r *MockRpcNode) GetOutputs(ctx context.Context, outpoints []wire.OutPoint) ([]bitcoin.UTXO, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	results := make([]bitcoin.UTXO, len(outpoints))
	for i, outpoint := range outpoints {
		tx, ok := r.txs[outpoint.Hash]
		if !ok {
			return results, fmt.Errorf("Couldn't find tx in r : %s", outpoint.Hash.String())
		}

		if int(outpoint.Index) >= len(tx.TxOut) {
			return results, fmt.Errorf("Invalid output index for txid %d/%d : %s", outpoint.Index,
				len(tx.TxOut), outpoint.Hash.String())
		}

		results[i] = bitcoin.UTXO{
			Hash:          outpoint.Hash,
			Index:         outpoint.Index,
			Value:         tx.TxOut[outpoint.Index].Value,
			LockingScript: tx.TxOut[outpoint.Index].PkScript,
		}
	}
	return results, nil
}

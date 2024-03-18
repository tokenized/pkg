package expanded_tx

import (
	"encoding/json"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

func Test_Unmarshal(t *testing.T) {
	inEtx := &ExpandedTx{
		Tx: wire.NewMsgTx(1),
	}

	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	ls, _ := key.LockingScript()
	inEtx.Tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, nil))
	inEtx.Tx.AddTxOut(wire.NewTxOut(1000, ls))
	inEtx.SpentOutputs = Outputs{
		{
			Value:         1100,
			LockingScript: ls,
		},
	}

	js, err := json.MarshalIndent(inEtx, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal json : %s", err)
	}

	t.Logf("JSON : %s", js)

	outEtx := &ExpandedTx{}
	if err := json.Unmarshal(js, outEtx); err != nil {
		t.Fatalf("Failed to unmarshal expanded tx : %s", err)
	}

	t.Logf("Expanded tx : %s", outEtx)
}

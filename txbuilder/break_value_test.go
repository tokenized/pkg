package txbuilder

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

func Test_BreakValue(t *testing.T) {
	feeRate := float32(1.0)
	dustFeeRate := float32(1.0)

	changeAddresses := make([]AddressKeyID, 5)
	for i := 0; i < len(changeAddresses); i++ {
		key, err := bitcoin.GenerateKey(bitcoin.MainNet)
		if err != nil {
			t.Fatalf("Failed to generate key : %s", err)
		}

		ra, err := key.RawAddress()
		if err != nil {
			t.Fatalf("Failed to generate address : %s", err)
		}

		ak := AddressKeyID{Address: ra}
		changeAddresses[i] = ak
	}

	changeValues := []uint64{
		100000000,
		50000000,
		25000000,
		10000000,
		5000000,
		2500000,
		1000000,
		500000,
		250000,
		100000,
		50000,
		25000,
		10000,
		5000,
		500,
	}
	breakValue := uint64(10000)

	for _, changeValue := range changeValues {
		t.Logf("Testing BreakValue %d/%d", changeValue, breakValue)

		outputs, err := BreakValue(changeValue, breakValue, changeAddresses, dustFeeRate, feeRate,
			true, true)
		if err != nil {
			t.Fatalf("Failed to break change : %s", err)
		}

		sum := uint64(0)
		txfees := uint64(0)
		for _, output := range outputs {
			sum += output.TxOut.Value
			txfees += uint64(float32(output.TxOut.SerializeSize()) * feeRate)
			t.Logf("Output %d : %x", output.TxOut.Value, output.TxOut.PkScript)
		}

		if sum > changeValue {
			t.Fatalf("Total output value too high : %d > %d", sum, changeValue)
		}

		if changeValue > 500 {
			if sum+txfees != changeValue {
				t.Fatalf("Total output + fees is wrong : got %d, want %d", sum+txfees, changeValue)
			}
		} else {
			// change is less than dust so it can't be included in an output
			if sum+txfees != 0 {
				t.Fatalf("Total output + fees is wrong : got %d, want %d", sum+txfees, 0)
			}
		}

		t.Logf("Total output value : %d (%d fees)", sum, txfees)
	}
}

func Test_BreakValueNoFee(t *testing.T) {
	feeRate := float32(0.0)
	dustFeeRate := float32(1.0)

	addresses := make([]AddressKeyID, 5)
	for i := 0; i < len(addresses); i++ {
		key, err := bitcoin.GenerateKey(bitcoin.MainNet)
		if err != nil {
			t.Fatalf("Failed to generate key : %s", err)
		}

		ra, err := key.RawAddress()
		if err != nil {
			t.Fatalf("Failed to generate address : %s", err)
		}

		ak := AddressKeyID{Address: ra}
		addresses[i] = ak
	}

	lockingScript, err := addresses[0].Address.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	_, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate, feeRate)

	values := []uint64{
		100000000,
		50000000,
		25000000,
		10000000,
		5000000,
		2500000,
		1000000,
		500000,
		250000,
		100000,
		50000,
		25000,
		10000,
		5000,
		500,
	}
	breakValue := uint64(10000)

	for _, value := range values {
		t.Logf("Testing BreakValue %d/%d", value, breakValue)

		outputs, err := BreakValue(value, breakValue, addresses, dustFeeRate, feeRate, true, true)
		if err != nil {
			t.Fatalf("Failed to break change : %s", err)
		}

		sum := uint64(0)
		for _, output := range outputs {
			sum += output.TxOut.Value
			t.Logf("Output %d : %x", output.TxOut.Value, output.TxOut.PkScript)
		}

		if value > dustLimit {
			if sum != value {
				t.Fatalf("Total output is wrong : got %d, want %d", sum, value)
			}
		} else {
			// change is less than dust so it can't be included in an output
			if sum != 0 {
				t.Fatalf("Total output + fees is wrong : got %d, want %d", sum, 0)
			}
		}

		t.Logf("Total output value : %d", sum)
	}
}

func Test_AddFundingBreakChange(t *testing.T) {
	changeAddresses := make([]AddressKeyID, 5)
	for i := 0; i < len(changeAddresses); i++ {
		key, err := bitcoin.GenerateKey(bitcoin.MainNet)
		if err != nil {
			t.Fatalf("Failed to generate key : %s", err)
		}

		ra, err := key.RawAddress()
		if err != nil {
			t.Fatalf("Failed to generate address : %s", err)
		}

		ak := AddressKeyID{Address: ra}
		changeAddresses[i] = ak
	}

	utxoSets := [][]bitcoin.UTXO{
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 0,
				Value: 10543,
			},
			bitcoin.UTXO{
				Index: 3,
				Value: 25080,
			},
			bitcoin.UTXO{
				Index: 1,
				Value: 103490,
			},
			bitcoin.UTXO{
				Index: 5,
				Value: 51200,
			},
			bitcoin.UTXO{
				Index: 2,
				Value: 450090,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 0,
				Value: 10600,
			},
			bitcoin.UTXO{
				Index: 3,
				Value: 25071,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 2,
				Value: 458400,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 2,
				Value: 5908000,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 0,
				Value: 10000,
			},
			bitcoin.UTXO{
				Index: 3,
				Value: 25000,
			},
			bitcoin.UTXO{
				Index: 2,
				Value: 5000000,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 2,
				Value: 115908000,
			},
		},
	}

	type Receiver struct {
		Address bitcoin.RawAddress
		Value   uint64
	}

	outputSets := [][]Receiver{
		[]Receiver{
			Receiver{
				Value: 23489,
			},
			Receiver{
				Value: 8142,
			},
		},
		[]Receiver{
			Receiver{
				Value: 9342,
			},
		},
		[]Receiver{
			Receiver{
				Value: 4601,
			},
			Receiver{
				Value: 10000,
			},
			Receiver{
				Value: 20492,
			},
		},
	}

	for setIndex, utxos := range utxoSets {
		for i, _ := range utxos {
			key, err := bitcoin.GenerateKey(bitcoin.MainNet)
			if err != nil {
				t.Fatalf("Failed to generate key : %s", err)
			}

			ra, err := key.RawAddress()
			if err != nil {
				t.Fatalf("Failed to create raw address : %s", err)
			}

			lockingScript, err := ra.LockingScript()
			if err != nil {
				t.Fatalf("Failed to create locking : %s", err)
			}

			rand.Read(utxos[i].Hash[:])
			utxos[i].LockingScript = lockingScript
		}

		utxoSets[setIndex] = utxos
	}

	for setIndex, receivers := range outputSets {
		for i, _ := range receivers {
			key, err := bitcoin.GenerateKey(bitcoin.MainNet)
			if err != nil {
				t.Fatalf("Failed to generate key : %s", err)
			}

			ra, err := key.RawAddress()
			if err != nil {
				t.Fatalf("Failed to create raw address : %s", err)
			}

			receivers[i].Address = ra
		}

		outputSets[setIndex] = receivers
	}

	for utxoIndex, utxos := range utxoSets {
		for receiverIndex, receivers := range outputSets {
			t.Logf("Testing utxo set %d, receiver set %d", utxoIndex, receiverIndex)

			tx := NewTxBuilder(1.0, 1.0)

			for _, receiver := range receivers {
				if err := tx.AddPaymentOutput(receiver.Address, receiver.Value, false); err != nil {
					t.Fatalf("Failed to add payment output : %s", err)
				}
			}

			if err := tx.AddFundingBreakChange(utxos, 10000, changeAddresses); err != nil {
				t.Fatalf("Failed to add funding : %s", err)
			}

			estimatedFee := tx.EstimatedFee()
			totalInput := uint64(0)
			for _, input := range tx.Inputs {
				totalInput += input.Value
			}
			t.Logf("Total Input %d", totalInput)

			totalOutput := uint64(0)
			totalChange := uint64(0)
			for i, output := range tx.MsgTx.TxOut {
				totalOutput += output.Value

				if i < len(receivers) {
					if output.Value != receivers[i].Value {
						t.Fatalf("Wrong payment output value : got %d, want %d", output.Value,
							receivers[i].Value)
					}
					t.Logf("Payment Output %d : %x", output.Value, output.PkScript)
				} else {
					totalChange += output.Value
					t.Logf("Change Output  %d : %x", output.Value, output.PkScript)
				}
			}

			t.Logf("Change %d, Estimated Fee %d, Actual Fee %d", totalChange, estimatedFee,
				totalInput-totalOutput)

			if totalChange == 0 {
				if totalInput-estimatedFee-totalOutput > 546 {
					t.Fatalf("Total output value leaves too much fee : output %d != input %d, fee %d", totalOutput,
						totalInput-estimatedFee, totalInput-estimatedFee-totalOutput)
				}
			} else if totalOutput != totalInput-estimatedFee {
				t.Fatalf("Total output value wrong : output %d != input %d", totalOutput,
					totalInput-estimatedFee)
			}
		}
	}
}

func Test_AddFundingBreakChangeInitiallyFunded(t *testing.T) {
	changeAddresses := make([]AddressKeyID, 5)
	for i := 0; i < len(changeAddresses); i++ {
		key, err := bitcoin.GenerateKey(bitcoin.MainNet)
		if err != nil {
			t.Fatalf("Failed to generate key : %s", err)
		}

		ra, err := key.RawAddress()
		if err != nil {
			t.Fatalf("Failed to generate address : %s", err)
		}

		ak := AddressKeyID{Address: ra}
		changeAddresses[i] = ak
	}

	utxoSets := [][]bitcoin.UTXO{
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 0,
				Value: 10543,
			},
			bitcoin.UTXO{
				Index: 3,
				Value: 25080,
			},
			bitcoin.UTXO{
				Index: 1,
				Value: 103490,
			},
			bitcoin.UTXO{
				Index: 5,
				Value: 51200,
			},
			bitcoin.UTXO{
				Index: 2,
				Value: 450090,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 0,
				Value: 10600,
			},
			bitcoin.UTXO{
				Index: 3,
				Value: 25071,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 2,
				Value: 458400,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 2,
				Value: 5908000,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 0,
				Value: 10000,
			},
			bitcoin.UTXO{
				Index: 3,
				Value: 25000,
			},
			bitcoin.UTXO{
				Index: 2,
				Value: 5000000,
			},
		},
		[]bitcoin.UTXO{
			bitcoin.UTXO{
				Index: 2,
				Value: 115908000,
			},
		},
	}

	type Receiver struct {
		Address bitcoin.RawAddress
		Value   uint64
	}

	outputSets := [][]Receiver{
		[]Receiver{
			Receiver{
				Value: 23489,
			},
			Receiver{
				Value: 8142,
			},
		},
		[]Receiver{
			Receiver{
				Value: 9342,
			},
		},
		[]Receiver{
			Receiver{
				Value: 4601,
			},
			Receiver{
				Value: 10000,
			},
			Receiver{
				Value: 20492,
			},
		},
	}

	for setIndex, utxos := range utxoSets {
		for i, _ := range utxos {
			key, err := bitcoin.GenerateKey(bitcoin.MainNet)
			if err != nil {
				t.Fatalf("Failed to generate key : %s", err)
			}

			ra, err := key.RawAddress()
			if err != nil {
				t.Fatalf("Failed to create raw address : %s", err)
			}

			lockingScript, err := ra.LockingScript()
			if err != nil {
				t.Fatalf("Failed to create locking : %s", err)
			}

			rand.Read(utxos[i].Hash[:])
			utxos[i].LockingScript = lockingScript
		}

		utxoSets[setIndex] = utxos
	}

	for setIndex, receivers := range outputSets {
		for i, _ := range receivers {
			key, err := bitcoin.GenerateKey(bitcoin.MainNet)
			if err != nil {
				t.Fatalf("Failed to generate key : %s", err)
			}

			ra, err := key.RawAddress()
			if err != nil {
				t.Fatalf("Failed to create raw address : %s", err)
			}

			receivers[i].Address = ra
		}

		outputSets[setIndex] = receivers
	}

	for utxoIndex, utxos := range utxoSets {
		for receiverIndex, receivers := range outputSets {
			t.Logf("Testing utxo set %d, receiver set %d", utxoIndex, receiverIndex)

			tx := NewTxBuilder(1.0, 1.0)

			tx.AddInputUTXO(utxos[0])

			for _, receiver := range receivers {
				if err := tx.AddPaymentOutput(receiver.Address, receiver.Value, false); err != nil {
					t.Fatalf("Failed to add payment output : %s", err)
				}
			}

			if err := tx.AddFundingBreakChange(utxos[1:], 10000, changeAddresses); err != nil {
				t.Fatalf("Failed to add funding : %s", err)
			}

			estimatedFee := tx.EstimatedFee()
			totalInput := uint64(0)
			for _, input := range tx.Inputs {
				totalInput += input.Value
			}
			t.Logf("Total Input %d", totalInput)

			totalOutput := uint64(0)
			totalChange := uint64(0)
			for i, output := range tx.MsgTx.TxOut {
				totalOutput += output.Value

				if i < len(receivers) {
					if output.Value != receivers[i].Value {
						t.Fatalf("Wrong payment output value : got %d, want %d", output.Value,
							receivers[i].Value)
					}
					t.Logf("Payment Output %d : %x", output.Value, output.PkScript)
				} else {
					totalChange += output.Value
					t.Logf("Change Output  %d : %x", output.Value, output.PkScript)
				}
			}

			t.Logf("Change %d, Estimated Fee %d, Actual Fee %d", totalChange, estimatedFee,
				totalInput-totalOutput)

			if totalChange == 0 {
				if totalInput-estimatedFee-totalOutput > 546 {
					t.Fatalf("Total output value leaves too much fee : output %d != input %d, fee %d", totalOutput,
						totalInput-estimatedFee, totalInput-estimatedFee-totalOutput)
				}
			} else if totalOutput != totalInput-estimatedFee {
				t.Fatalf("Total output value wrong : output %d != input %d", totalOutput,
					totalInput-estimatedFee)
			}
		}
	}
}

func Test_RandomizeOutputs(t *testing.T) {
	for j := 0; j < 20; j++ {
		tx := NewTxBuilder(1.0, 1.0)

		for i := 0; i < rand.Intn(10)+1; i++ {
			key, err := bitcoin.GenerateKey(bitcoin.MainNet)
			if err != nil {
				t.Fatalf("Failed to generate key : %s", err)
			}

			ra, err := key.RawAddress()
			if err != nil {
				t.Fatalf("Failed to create raw address : %s", err)
			}

			if err := tx.AddPaymentOutput(ra, uint64(rand.Intn(20000)+1000), false); err != nil {
				t.Fatalf("Failed to add payment output : %s", err)
			}
		}

		t.Logf("Original Tx : \n%s\n", tx.MsgTx.StringWithAddresses(bitcoin.MainNet))

		txouts := make([]*wire.TxOut, len(tx.MsgTx.TxOut))
		for i, txout := range tx.MsgTx.TxOut {
			newTxOut := *txout
			txouts[i] = &newTxOut
		}

		tx.RandomizeOutputs()

		t.Logf("Random Tx : \n%s\n", tx.MsgTx.StringWithAddresses(bitcoin.MainNet))

		for _, txout := range txouts {
			found := false
			for _, to := range tx.MsgTx.TxOut {
				if to.Value == txout.Value && bytes.Equal(to.PkScript, txout.PkScript) {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("Output not found : %d %x", txout.Value, txout.PkScript)
			}
		}
	}
}

func Test_RandomizeOutputsAfter(t *testing.T) {
	for j := 0; j < 20; j++ {
		tx := NewTxBuilder(1.0, 1.0)

		for i := 0; i < rand.Intn(10)+1; i++ {
			key, err := bitcoin.GenerateKey(bitcoin.MainNet)
			if err != nil {
				t.Fatalf("Failed to generate key : %s", err)
			}

			ra, err := key.RawAddress()
			if err != nil {
				t.Fatalf("Failed to create raw address : %s", err)
			}

			if err := tx.AddPaymentOutput(ra, uint64(rand.Intn(20000)+1000), false); err != nil {
				t.Fatalf("Failed to add payment output : %s", err)
			}
		}

		t.Logf("Original Tx : \n%s\n", tx.MsgTx.StringWithAddresses(bitcoin.MainNet))

		txouts := make([]*wire.TxOut, len(tx.MsgTx.TxOut))
		for i, txout := range tx.MsgTx.TxOut {
			newTxOut := *txout
			txouts[i] = &newTxOut
		}

		after := rand.Intn(len(txouts))
		t.Logf("Randomizing outputs after %d", after)
		tx.RandomizeOutputsAfter(after)

		t.Logf("Random Tx : \n%s\n", tx.MsgTx.StringWithAddresses(bitcoin.MainNet))

		for i, txout := range txouts {
			if i <= after {
				if tx.MsgTx.TxOut[i].Value != txout.Value ||
					!bytes.Equal(tx.MsgTx.TxOut[i].PkScript, txout.PkScript) {
					t.Fatalf("Unmoved output %d not matching : %d %x", i, txout.Value, txout.PkScript)
				}
				continue
			}

			found := false
			for _, to := range tx.MsgTx.TxOut {
				if to.Value == txout.Value && bytes.Equal(to.PkScript, txout.PkScript) {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("Output not found : %d %x", txout.Value, txout.PkScript)
			}
		}
	}
}

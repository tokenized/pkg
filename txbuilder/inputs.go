package txbuilder

import (
	"fmt"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

// InputSupplement contains data required to sign an input that is not already in the wire.MsgTx.
type InputSupplement struct {
	LockingScript []byte `json:"locking_script"`
	Value         uint64 `json:"value"`

	// Optional identifier for external use to track the key needed to sign the input.
	KeyID string `json:"key_id,omitempty"`
}

// InputAddress returns the address that is paying to the input.
func (tx *TxBuilder) InputAddress(index int) (bitcoin.RawAddress, error) {
	if index >= len(tx.Inputs) {
		return bitcoin.RawAddress{}, errors.New("Input index out of range")
	}
	return bitcoin.RawAddressFromLockingScript(tx.Inputs[index].LockingScript)
}

// AddInput adds an input to TxBuilder.
func (tx *TxBuilder) AddInputUTXO(utxo bitcoin.UTXO) error {
	// Check that utxo isn't already an input.
	for _, input := range tx.MsgTx.TxIn {
		if input.PreviousOutPoint.Hash.Equal(&utxo.Hash) &&
			input.PreviousOutPoint.Index == utxo.Index {
			return errors.Wrap(ErrDuplicateInput, "")
		}
	}

	input := InputSupplement{
		LockingScript: utxo.LockingScript,
		Value:         utxo.Value,
		KeyID:         utxo.KeyID,
	}
	tx.Inputs = append(tx.Inputs, &input)

	txin := wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: utxo.Hash, Index: utxo.Index},
		Sequence:         wire.MaxTxInSequenceNum,
	}
	tx.MsgTx.AddTxIn(&txin)
	return nil
}

// AddInput adds an input to TxBuilder.
//   outpoint reference the output being spent.
//   lockScript is the script from the output being spent.
//   value is the number of satoshis from the output being spent.
func (tx *TxBuilder) AddInput(outpoint wire.OutPoint, lockScript []byte, value uint64) error {
	// Check that outpoint isn't already an input.
	for _, input := range tx.MsgTx.TxIn {
		if input.PreviousOutPoint.Hash.Equal(&outpoint.Hash) &&
			input.PreviousOutPoint.Index == outpoint.Index {
			return errors.Wrap(ErrDuplicateInput, "")
		}
	}

	input := InputSupplement{
		LockingScript: lockScript,
		Value:         value,
	}
	tx.Inputs = append(tx.Inputs, &input)

	txin := wire.TxIn{
		PreviousOutPoint: outpoint,
		Sequence:         wire.MaxTxInSequenceNum,
	}
	tx.MsgTx.AddTxIn(&txin)
	return nil
}

// AddFunding adds inputs spending the specified UTXOs until the transaction has enough funding to
//   cover the fees and outputs.
// If SendMax is set then all UTXOs are added as inputs.
func (tx *TxBuilder) AddFunding(utxos []bitcoin.UTXO) error {
	inputValue := tx.InputValue()
	outputValue := tx.OutputValue(true)
	estFeeValue := tx.EstimatedFee()

	if !tx.SendMax && inputValue > outputValue && inputValue-outputValue >= estFeeValue {
		return tx.CalculateFee() // Already funded
	}

	if len(utxos) == 0 {
		return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("no more utxos: %d/%d",
			inputValue, outputValue+estFeeValue))
	}

	// Calculate additional funding needed. Include cost of first added input.
	// TODO Add support for input scripts other than P2PKH.
	neededFunding := estFeeValue + outputValue - inputValue
	estOutputFee := uint64(float32(P2PKHOutputSize) * tx.FeeRate)
	duplicateValue := uint64(0)

	// Calculate the dust limit used when determining if a change output will be added
	var changeDustLimit uint64
	for i, output := range tx.Outputs {
		if output.IsRemainder {
			continue
		}
		changeDustLimit = DustLimitForOutput(tx.MsgTx.TxOut[i], tx.DustFeeRate)
		if changeDustLimit > 0 {
			break
		}
	}
	if changeDustLimit == 0 && !tx.ChangeAddress.IsEmpty() {
		changeDustLimit = DustLimitForAddress(tx.ChangeAddress, tx.DustFeeRate)
	}
	if changeDustLimit == 0 {
		// Use P2PKH dust limit
		changeDustLimit = DustLimit(P2PKHOutputSize, tx.DustFeeRate)
	}

	for _, utxo := range utxos {
		if err := tx.AddInputUTXO(utxo); err != nil {
			if errors.Cause(err) == ErrDuplicateInput {
				duplicateValue += utxo.Value
				continue
			}
			return errors.Wrap(err, "adding input")
		}

		inputFee, err := tx.utxoFee(utxo)
		if err != nil {
			return errors.Wrap(err, "utxo fee")
		}
		neededFunding += inputFee // Add cost of input

		if tx.SendMax {
			continue
		}

		if neededFunding <= utxo.Value {
			// Funding complete
			change := utxo.Value - neededFunding
			if change > changeDustLimit {
				for i, output := range tx.Outputs {
					if output.IsRemainder {
						// Updating existing "change" output
						tx.MsgTx.TxOut[i].Value += change
						return nil
					}
				}

				if change > changeDustLimit+estOutputFee {
					// Add new change output
					change -= estOutputFee
					if tx.ChangeAddress.IsEmpty() {
						return errors.Wrap(ErrChangeAddressNeeded, fmt.Sprintf("Remaining: %d", change))
					}

					if err := tx.AddPaymentOutput(tx.ChangeAddress, change, true); err != nil {
						return errors.Wrap(err, "adding change")
					}
					tx.Outputs[len(tx.Outputs)-1].KeyID = tx.ChangeKeyID
				}
			}

			return nil
		}

		// More UTXOs required
		neededFunding -= utxo.Value // Subtract the value this input added
	}

	if tx.SendMax {
		return tx.CalculateFee()
	} else {
		available := uint64(0)
		for _, input := range tx.Inputs {
			available += input.Value
		}
		return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", available-duplicateValue,
			outputValue+tx.EstimatedFee()))
			outputValue+tx.EstimatedFee()))
	}

	return nil
}

// utxoFee calculates the tx fee for the input to spend the UTXO.
func (tx *TxBuilder) utxoFee(utxo bitcoin.UTXO) (uint64, error) {
	size, err := lockingScriptUnlockSize(utxo.LockingScript)
	if err != nil {
		return 0, errors.Wrap(err, "unlock size")
	}
	return uint64(float32(size) * tx.FeeRate), nil
}

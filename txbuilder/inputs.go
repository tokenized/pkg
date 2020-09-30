package txbuilder

import (
	"fmt"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

// InputAddress returns the address that is paying to the input.
func (tx *TxBuilder) InputAddress(index int) (bitcoin.RawAddress, error) {
	if index >= len(tx.Inputs) {
		return bitcoin.RawAddress{}, errors.New("Input index out of range")
	}
	return bitcoin.RawAddressFromLockingScript(tx.Inputs[index].LockingScript)
}

// AddInputUTXO adds an input to TxBuilder using a UTXO.
func (tx *TxBuilder) AddInputUTXO(utxo bitcoin.UTXO) error {
	// Check that utxo isn't already an input.
	for _, input := range tx.MsgTx.TxIn {
		if input.PreviousOutPoint.Hash.Equal(&utxo.Hash) &&
			input.PreviousOutPoint.Index == utxo.Index {
			return errors.Wrap(ErrDuplicateInput, "")
		}
	}

	input := &InputSupplement{
		LockingScript: utxo.LockingScript,
		Value:         utxo.Value,
		KeyID:         utxo.KeyID,
	}
	tx.Inputs = append(tx.Inputs, input)

	txin := wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: utxo.Hash, Index: utxo.Index},
		Sequence:         wire.MaxTxInSequenceNum,
	}
	tx.MsgTx.AddTxIn(&txin)
	return nil
}

// InsertInput inserts an input into TxBuilder at the specified index.
func (tx *TxBuilder) InsertInput(index int, utxo bitcoin.UTXO, txin *wire.TxIn) error {
	if index > len(tx.MsgTx.TxIn) {
		return errors.New("Input index out of range")
	}

	// Check that utxo isn't already an input.
	for _, input := range tx.MsgTx.TxIn {
		if input.PreviousOutPoint.Hash.Equal(&utxo.Hash) &&
			input.PreviousOutPoint.Index == utxo.Index {
			return errors.Wrap(ErrDuplicateInput, "")
		}
	}

	input := &InputSupplement{
		LockingScript: utxo.LockingScript,
		Value:         utxo.Value,
		KeyID:         utxo.KeyID,
	}

	afterInputs := make([]*InputSupplement, len(tx.Inputs)-index)
	copy(afterInputs, tx.Inputs[index:])
	tx.Inputs = append(append(tx.Inputs[:index], input), afterInputs...)

	afterTxIn := make([]*wire.TxIn, len(tx.MsgTx.TxIn)-index)
	copy(afterTxIn, tx.MsgTx.TxIn[index:])
	tx.MsgTx.TxIn = append(append(tx.MsgTx.TxIn[:index], txin), afterTxIn...)
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

	tx.MsgTx.AddTxIn(wire.NewTxIn(&outpoint, nil))
	return nil
}

func (tx *TxBuilder) RemoveInput(index int) error {
	if index >= len(tx.Inputs) || index >= len(tx.MsgTx.TxIn) {
		return errors.New("Input index out of range")
	}

	tx.Inputs = append(tx.Inputs[:index], tx.Inputs[index+1:]...)
	tx.MsgTx.TxIn = append(tx.MsgTx.TxIn[:index], tx.MsgTx.TxIn[index+1:]...)
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
	changeOutputFee := uint64(0)
	duplicateValue := uint64(0)

	// Calculate the dust limit used when determining if a change output will be added
	var changeDustLimit uint64
	for i, output := range tx.Outputs {
		if !output.IsRemainder {
			continue
		}

		changeOutputFee = uint64(tx.MsgTx.TxOut[i].SerializeSize())
		changeDustLimit = DustLimitForOutput(tx.MsgTx.TxOut[i], tx.DustFeeRate)
		if changeDustLimit > 0 {
			break
		}
	}
	if changeDustLimit == 0 && !tx.ChangeAddress.IsEmpty() {
		var err error
		changeOutputFee, err = AddressOutputFee(tx.ChangeAddress, tx.DustFeeRate)
		if err != nil {
			return errors.Wrap(err, "address output fee")
		}
		addressChangeDustLimit, err := DustLimitForAddress(tx.ChangeAddress, tx.DustFeeRate)
		if err == nil {
			changeDustLimit = addressChangeDustLimit
		}
	}
	if changeDustLimit == 0 {
		// Use P2PKH dust limit
		changeDustLimit = DustLimit(P2PKHOutputSize, tx.DustFeeRate)
		changeOutputFee = uint64(float32(P2PKHOutputSize) * tx.FeeRate)
	}

	for _, utxo := range utxos {
		if err := tx.AddInputUTXO(utxo); err != nil {
			if errors.Cause(err) == ErrDuplicateInput {
				duplicateValue += utxo.Value
				continue
			}
			return errors.Wrap(err, "adding input")
		}

		inputFee, err := UTXOFee(utxo, tx.FeeRate)
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

				if change > changeDustLimit+changeOutputFee {
					// Add new change output
					change -= changeOutputFee
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
	}

	return nil
}

// AddFundingBreakChange adds inputs spending the specified UTXOs until the transaction has enough
// funding to cover the fees and outputs.
// If SendMax is set then all UTXOs are added as inputs.
// If there is already an IsRemainder output, then it will get all of the "change" and it won't be
// broken up.
// tx.ChangeAddress is ignored.
// breakValue should be a fairly low value that is the smallest UTXO you want created other than
// the remainder.
// It is recommended to provide at least 5 change addresses. More addresses means more privacy, but
// also more UTXOs and more tx fees.
func (tx *TxBuilder) AddFundingBreakChange(utxos []bitcoin.UTXO, breakValue uint64,
	changeAddresses []AddressKeyID) error {

	// Calculate the dust limit used when determining if a change output will be added
	remainderIncluded := false
	var remainderDustLimit uint64
	for i, output := range tx.Outputs {
		if !output.IsRemainder {
			continue
		}

		remainderIncluded = true
		remainderDustLimit = DustLimitForOutput(tx.MsgTx.TxOut[i], tx.DustFeeRate)
		if remainderDustLimit == 0 {
			// Default to P2PKH dust limit
			remainderDustLimit = DustLimit(P2PKHOutputSize, tx.DustFeeRate)
		}
		break
	}

	inputValue := tx.InputValue()
	outputValue := tx.OutputValue(true)
	estFeeValue := tx.EstimatedFee()

	changeFee, _, err := OutputFeeAndDustForAddress(changeAddresses[0].Address,
		tx.DustFeeRate, tx.FeeRate)
	if err != nil {
		return errors.Wrap(err, "change address fee")
	}

	// Check if tx is already funded.
	if !tx.SendMax && inputValue > outputValue && inputValue-outputValue >= estFeeValue {
		if !remainderIncluded {
			// Ensure added change output is funded
			if inputValue-outputValue >= estFeeValue+changeFee {
				if err := tx.SetChangeAddress(changeAddresses[0].Address,
					changeAddresses[0].KeyID); err != nil {
					return errors.Wrap(err, "set change address")
				}
				return tx.CalculateFee() // Already funded
			}
		} else {
			return tx.CalculateFee() // Already funded
		}
	}

	if len(utxos) == 0 {
		return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("no more utxos: %d/%d",
			inputValue, outputValue+estFeeValue))
	}

	// Calculate additional funding needed. Include cost of first added input.
	// TODO Add support for input scripts other than P2PKH.
	neededFunding := estFeeValue + outputValue - inputValue
	duplicateValue := uint64(0)

	for _, utxo := range utxos {
		if err := tx.AddInputUTXO(utxo); err != nil {
			if errors.Cause(err) == ErrDuplicateInput {
				duplicateValue += utxo.Value
				continue
			}
			return errors.Wrap(err, "adding input")
		}

		inputFee, err := UTXOFee(utxo, tx.FeeRate)
		if err != nil {
			return errors.Wrap(err, "utxo fee")
		}
		neededFunding += inputFee // Add cost of input

		if tx.SendMax {
			continue
		}

		if neededFunding <= utxo.Value {
			// Funding complete
			changeValue := utxo.Value - neededFunding

			if remainderIncluded {
				for i, output := range tx.Outputs {
					if output.IsRemainder {
						// Updating existing "change" output
						tx.MsgTx.TxOut[i].Value += changeValue
						return nil
					}
				}

				return errors.New("Missing remainder that was previously there!")
			} else {
				// Break change between supplied addresses.
				outputs, err := BreakValue(changeValue, breakValue, changeAddresses,
					tx.DustFeeRate, tx.FeeRate, true)
				if err != nil {
					return errors.Wrap(err, "break change")
				}

				tx.AddOutputs(outputs)
			}

			return nil
		}

		// More UTXOs required
		neededFunding -= utxo.Value // Subtract the value this input added
	}

	if tx.SendMax {
		return tx.CalculateFee()
	}

	available := uint64(0)
	for _, input := range tx.Inputs {
		available += input.Value
	}
	return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", available-duplicateValue,
		outputValue+tx.EstimatedFee()))
}

// UTXOFee calculates the tx fee for the input to spend the UTXO.
func UTXOFee(utxo bitcoin.UTXO, feeRate float32) (uint64, error) {
	size, err := lockingScriptUnlockSize(utxo.LockingScript)
	if err != nil {
		return 0, errors.Wrap(err, "unlock size")
	}
	return uint64(float32(size) * feeRate), nil
}

// AddressOutputFee returns the tx fee to include an address as an output in a tx.
func AddressOutputFee(ra bitcoin.RawAddress, feeRate float32) (uint64, error) {
	lockingScript, err := ra.LockingScript()
	if err != nil {
		return 0, errors.Wrap(err, "locking script")
	}

	txout := wire.TxOut{PkScript: lockingScript}
	return uint64(txout.SerializeSize()), nil
}

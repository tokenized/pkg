package txbuilder

import (
	"fmt"
	"math"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

const (
	// InputBaseSize is the size of a tx input not including script
	//   Previous Transaction ID = 32 bytes
	//   Previous Transaction Output Index = 4 bytes
	InputBaseSize = 32 + 4

	// MaximumP2PKHInputSize is the maximum serialized size of a P2PKH tx input based on all of the
	// variable sized data.
	// P2PKH/P2SH input size 149
	//   Previous Transaction ID = 32 bytes
	//   Previous Transaction Output Index = 4 bytes
	//   script size = 1 byte
	//   Signature push to stack = 74
	//       push size = 1 byte
	//       signature up to = 72 bytes
	//       signature hash type = 1 byte
	//   Public key push to stack = 34
	//       push size = 1 byte
	//       public key size = 33 bytes
	//   Sequence number = 4
	MaximumP2PKHSigScriptSize = 1 + 74 + 34
	MaximumP2PKHInputSize     = InputBaseSize + MaximumP2PKHSigScriptSize + 4

	// MaximumP2RPHInputSize is the maximum serialized size of a P2RPH tx input based on all of the
	// variable sized data.
	// P2PKH/P2SH/P2RPH input size 149
	//   Previous Transaction ID = 32 bytes
	//   Previous Transaction Output Index = 4 bytes
	//   script size = 1 byte
	//   Public key push to stack = 34
	//       push size = 1 byte
	//       public key size = 33 bytes
	//   Signature push to stack = 74
	//       push size = 1 byte
	//       signature up to = 72 bytes
	//       signature hash type = 1 byte
	//   Sequence number = 4
	MaximumP2RPHSigScriptSize = 1 + 34 + 74
	MaximumP2RPHInputSize     = InputBaseSize + MaximumP2RPHSigScriptSize + 4

	// MaximumP2PKInputSize is the maximium serialized size of a P2PK tx input based on all of the
	// variable sized data.
	// P2PK input size 115
	//   Previous Transaction ID = 32 bytes
	//   Previous Transaction Output Index = 4 bytes
	//   script size = 1 byte
	//   Signature push to stack = 74
	//       push size = 1 byte
	//       signature up to = 72 bytes
	//       signature hash type = 1 byte
	//   Sequence number = 4
	MaximumP2PKSigScriptSize = 1 + 74
	MaximumP2PKInputSize     = InputBaseSize + MaximumP2PKSigScriptSize + 4

	// OutputBaseSize is the size of a tx output not including script
	OutputBaseSize = 8

	// P2PKHOutputSize is the serialized size of a P2PKH tx output.
	// P2PKH/P2SH output size 34
	//   amount = 8 bytes
	//   script size = 1 byte
	//   Script (25 bytes) OP_DUP OP_HASH160 <Push Data byte, PUB KEY/SCRIPT HASH (20 bytes)> OP_EQUALVERIFY
	//     OP_CHECKSIG
	P2PKHOutputScriptSize = 26
	P2PKHOutputSize       = OutputBaseSize + P2PKHOutputScriptSize

	// P2PKOutputSize is the serialized size of a P2PK tx output.
	// P2PK output size 44
	//   amount = 8 bytes
	//   script = 36
	//     script size = 1 byte ()
	//       Public key push to stack = 34
	//         push size = 1 byte
	//         public key size = 33 bytes
	//       OP_CHECKSIG = 1 byte
	P2PKOutputScriptSize = 36
	P2PKOutputSize       = OutputBaseSize + P2PKOutputScriptSize

	PublicKeyPushDataSize     = 34 // 1 byte push op code + 33 byte public key
	MaxSignaturesPushDataSize = 74 // 1 byte push op code + 72 byte sig + 1 byte sig hash type

	// DustInputSize is the fixed size of an input used in the calculation of the dust limit.
	// This is actually the estimated size of a P2PKH input, but is used for dust calculation of all
	//   locking scripts.
	DustInputSize = 148

	// BaseTxSize is the size of the tx not included in inputs and outputs.
	//   Version = 4 bytes
	//   LockTime = 4 bytes
	BaseTxSize = 8
)

// The fee should be estimated before signing, then after signing the fee should be checked.
// If the fee is too low after signing, then the fee should be adjusted and the tx re-signed.

func (tx *TxBuilder) Fee() uint64 {
	o := tx.OutputValue(true)
	i := tx.InputValue()
	if o > i {
		return 0
	}
	return i - o
}

// EstimatedSize returns the estimated size in bytes of the tx after signatures are added.
// It assumes all inputs are P2PKH, P2PK, or P2RPH.
func (tx *TxBuilder) EstimatedSize() int {
	result := BaseTxSize + wire.VarIntSerializeSize(uint64(len(tx.MsgTx.TxIn))) +
		wire.VarIntSerializeSize(uint64(len(tx.MsgTx.TxOut)))

	for _, input := range tx.Inputs {
		size, err := lockingScriptUnlockSize(input.LockingScript)
		if err != nil {
			result += MaximumP2PKHInputSize // Fall back to P2PKH
			continue
		}
		result += size
	}

	for _, output := range tx.MsgTx.TxOut {
		result += output.SerializeSize()
	}

	return result
}

func (tx *TxBuilder) EstimatedFee() uint64 {
	return uint64(float32(tx.EstimatedSize()) * tx.FeeRate)
}

func (tx *TxBuilder) CalculateFee() error {
	_, err := tx.adjustFee(int64(tx.EstimatedFee()) - int64(tx.Fee()))
	return err
}

// InputValue returns the sum of the values of the inputs.
func (tx *TxBuilder) InputValue() uint64 {
	inputValue := uint64(0)
	for _, input := range tx.Inputs {
		inputValue += input.Value
	}
	return inputValue
}

// OutputValue returns the sum of the values of the outputs.
func (tx *TxBuilder) OutputValue(includeChange bool) uint64 {
	outputValue := uint64(0)
	for i, output := range tx.MsgTx.TxOut {
		if includeChange || !tx.Outputs[i].IsRemainder {
			outputValue += uint64(output.Value)
		}
	}
	return outputValue
}

// changeSum returns the sum of the values of the outputs.
func (tx *TxBuilder) changeSum() uint64 {
	changeValue := uint64(0)
	for i, output := range tx.MsgTx.TxOut {
		if tx.Outputs[i].IsRemainder {
			changeValue += uint64(output.Value)
		}
	}
	return changeValue
}

// adjustFee adjusts the tx fee up or down depending on if the amount is negative or positive.
// It returns true if no further fee adjustments should be attempted.
func (tx *TxBuilder) adjustFee(amount int64) (bool, error) {
	if amount == int64(0) {
		return true, nil
	}

	done := false

	// Find change output
	changeOutputIndex := 0xffffffff
	for i, output := range tx.Outputs {
		if output.IsRemainder {
			changeOutputIndex = i
			break
		}
	}

	if amount > int64(0) {
		// Increase fee, transfer from change
		if changeOutputIndex == 0xffffffff {
			return false, errors.Wrap(ErrInsufficientValue, "No existing change for tx fee")
		}

		if tx.MsgTx.TxOut[changeOutputIndex].Value < uint64(amount) {
			return false, errors.Wrap(ErrInsufficientValue, "Not enough change for tx fee")
		}

		// Decrease change, thereby increasing the fee
		tx.MsgTx.TxOut[changeOutputIndex].Value -= uint64(amount)

		// Check if change is below dust
		if tx.MsgTx.TxOut[changeOutputIndex].Value <
			DustLimitForOutput(tx.MsgTx.TxOut[changeOutputIndex], tx.DustFeeRate) {
			if !tx.Outputs[changeOutputIndex].addedForFee {
				// Don't remove outputs unless they were added by fee adjustment
				return false, errors.Wrap(ErrInsufficientValue, "Not enough change for tx fee")
			}
			// Remove change output since it is less than dust. Dust will go to miner.
			tx.MsgTx.TxOut = append(tx.MsgTx.TxOut[:changeOutputIndex], tx.MsgTx.TxOut[changeOutputIndex+1:]...)
			tx.Outputs = append(tx.Outputs[:changeOutputIndex], tx.Outputs[changeOutputIndex+1:]...)
			done = true
		}
	} else {
		// Decrease fee, transfer to change
		if changeOutputIndex == 0xffffffff {
			// Add a change output if it would be more than the dust limit plus the fee to add the
			// output
			changeFee, dustLimit := OutputFeeAndDustForLockingScript(tx.ChangeScript,
				tx.DustFeeRate, tx.FeeRate)
			if uint64(-amount) > dustLimit+changeFee {
				if len(tx.ChangeScript) == 0 {
					return false, errors.Wrap(ErrChangeAddressNeeded, fmt.Sprintf("Remaining: %d",
						uint64(-amount)))
				}
				err := tx.AddOutput(tx.ChangeScript, uint64(-amount)-changeFee, true, false)
				if err != nil {
					return false, err
				}
				tx.Outputs[len(tx.Outputs)-1].KeyID = tx.ChangeKeyID
				tx.Outputs[len(tx.Outputs)-1].addedForFee = true
			} else {
				// Leave less than dust as additional tx fee
				done = true
			}
		} else {
			// Increase change, thereby decreasing the fee
			// (amount is negative so subracting it increases the change value)
			tx.MsgTx.TxOut[changeOutputIndex].Value += uint64(-amount)
		}
	}

	return done, nil
}

// lockingScriptUnlockFee returns the size (in bytes) of the input that spends it.
func lockingScriptUnlockSize(lockingScript bitcoin.Script) (int, error) {
	if lockingScript.IsP2PKH() {
		return MaximumP2PKHInputSize, nil
	}

	if lockingScript.IsP2PK() {
		return MaximumP2PKInputSize, nil
	}

	if required, total, err := lockingScript.MultiPKHCounts(); err == nil {
		// 1 op_code OP_FALSE or OP_TRUE for each signer (total) plus a public key and signature for
		// each required signer.
		scriptSize := total + (required * (PublicKeyPushDataSize + MaxSignaturesPushDataSize))
		return InputBaseSize + int(VarIntSerializeSize(uint64(scriptSize))) + int(scriptSize), nil
	}

	return 0, ErrWrongScriptTemplate
}

// VarIntSerializeSize returns the number of bytes it would take to serialize
// val as a variable length integer.
func VarIntSerializeSize(val uint64) int {
	// The value is small enough to be represented by itself, so it's
	// just 1 byte.
	if val < 0xfd {
		return 1
	}

	// Discriminant 1 byte plus 2 bytes for the uint16.
	if val <= math.MaxUint16 {
		return 3
	}

	// Discriminant 1 byte plus 4 bytes for the uint32.
	if val <= math.MaxUint32 {
		return 5
	}

	// Discriminant 1 byte plus 8 bytes for the uint64.
	return 9
}

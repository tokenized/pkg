package txbuilder

import (
	"bytes"
	"fmt"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

const (
	SubSystem = "TxBuilder" // For logger

	// DefaultVersion is the default TX version to use by the Buidler.
	DefaultVersion = int32(1)
)

var (
	// ErrChangeAddressNeeded means that a change address is needed to complete the tx.
	ErrChangeAddressNeeded = errors.New("Change address needed")

	// ErrInsufficientValue means that there is not enough bitcoin input to complete the tx.
	ErrInsufficientValue = errors.New("Insufficient Value")

	// ErrWrongPrivateKey means that the private key doesn't match the script it is being applied to.
	ErrWrongPrivateKey = errors.New("Wrong Private Key")

	// ErrMissingPrivateKey means that the required private key was not provided.
	ErrMissingPrivateKey = errors.New("Missing Private Key")

	// ErrWrongScriptTemplate means that the script template is not supported here.
	ErrWrongScriptTemplate = errors.New("Wrong Script Template")

	// ErrBelowDustValue means that a value provided for an output is below the dust limit and not
	// valid for the network.
	ErrBelowDustValue = errors.New("Below Dust Value")

	// ErrDuplicateInput means that the same UTXO is attempting to be spent more than once in the tx.
	ErrDuplicateInput = errors.New("Duplicate Input")

	// ErrMissingInputData means that data required to include an input in a tx was not provided.
	ErrMissingInputData = errors.New("Missing Input Data")
)

type TxBuilder struct {
	MsgTx        *wire.MsgTx
	Inputs       []*InputSupplement  // Input Data that is not in wire.MsgTx
	Outputs      []*OutputSupplement // Output Data that is not in wire.MsgTx
	ChangeScript bitcoin.Script      // The script to pay extra bitcoins to if a change output isn't specified
	FeeRate      float32             // The target fee rate in sat/byte
	SendMax      bool                // When set, AddFunding will add all UTXOs given

	// The fee rate used by miners to calculate dust. It is currently maintained as a different rate
	// than min accept and min propagate. Currently 1.0
	DustFeeRate float32

	// Optional identifier for external use to track the key needed to spend change
	ChangeKeyID string
}

type TransactionWithOutputs interface {
	TxID() bitcoin.Hash32
	GetMsgTx() *wire.MsgTx

	InputCount() int
	Input(index int) *wire.TxIn
	InputOutput(index int) (*wire.TxOut, error) // The output being spent by the input

	OutputCount() int
	Output(index int) *wire.TxOut
}

// NewTxBuilder returns a new TxBuilder with the specified change address.
func NewTxBuilder(feeRate, dustFeeRate float32) *TxBuilder {
	tx := wire.MsgTx{Version: DefaultVersion, LockTime: 0}
	result := TxBuilder{
		MsgTx:       &tx,
		FeeRate:     feeRate,
		DustFeeRate: dustFeeRate,
	}
	return &result
}

func NewTxBuilderFromTransactionWithOutputs(feeRate, dustFeeRate float32,
	tx TransactionWithOutputs) (*TxBuilder, error) {

	inputCount := tx.InputCount()
	result := TxBuilder{
		MsgTx:       tx.GetMsgTx(),
		FeeRate:     feeRate,
		DustFeeRate: dustFeeRate,
		Inputs:      make([]*InputSupplement, inputCount),
	}

	// Setup inputs
	var missingErr error
	for index := 0; index < inputCount; index++ {
		txin := tx.Input(index)
		if txin.PreviousOutPoint.Hash.IsZero() {
			result.Inputs[index] = &InputSupplement{}
			continue
		}

		output, err := tx.InputOutput(index)
		if err != nil {
			return nil, errors.Wrapf(err, "input %d output", index)
		}

		result.Inputs[index] = &InputSupplement{
			LockingScript: output.LockingScript,
			Value:         output.Value,
		}
	}

	// Setup outputs
	result.Outputs = make([]*OutputSupplement, len(result.MsgTx.TxOut))
	for index := range result.Outputs {
		result.Outputs[index] = &OutputSupplement{}
	}

	return &result, missingErr
}

// NewTxBuilderFromWire returns a new TxBuilder from a wire.MsgTx and the input txs.
func NewTxBuilderFromWire(feeRate, dustFeeRate float32, tx *wire.MsgTx,
	inputs []*wire.MsgTx) (*TxBuilder, error) {

	result := TxBuilder{
		MsgTx:       tx,
		FeeRate:     feeRate,
		DustFeeRate: dustFeeRate,
		Inputs:      make([]*InputSupplement, len(tx.TxIn)),
	}

	// Setup inputs
	var missingErr error
	for i, input := range result.MsgTx.TxIn {
		found := false
		for _, inputTx := range inputs {
			txHash := inputTx.TxHash()
			if bytes.Equal(txHash[:], input.PreviousOutPoint.Hash[:]) &&
				int(input.PreviousOutPoint.Index) < len(inputTx.TxOut) {
				// Add input
				result.Inputs[i] = &InputSupplement{
					LockingScript: inputTx.TxOut[input.PreviousOutPoint.Index].LockingScript,
					Value:         uint64(inputTx.TxOut[input.PreviousOutPoint.Index].Value),
				}
				found = true
				break
			}
		}
		if !found {
			missingErr = ErrMissingInputData
		}
	}

	// Setup outputs
	result.Outputs = make([]*OutputSupplement, len(result.MsgTx.TxOut))
	for i, _ := range result.Outputs {
		result.Outputs[i] = &OutputSupplement{}
	}

	return &result, missingErr
}

// NewTxBuilderFromWireUTXOs returns a new TxBuilder from a wire.MsgTx and the input UTXOs.
func NewTxBuilderFromWireUTXOs(feeRate, dustFeeRate float32, tx *wire.MsgTx,
	utxos []bitcoin.UTXO) (*TxBuilder, error) {

	result := TxBuilder{
		MsgTx:       tx,
		FeeRate:     feeRate,
		DustFeeRate: dustFeeRate,
		Inputs:      make([]*InputSupplement, len(tx.TxIn)),
	}

	// Setup inputs
	var missingErr error
	for i, input := range result.MsgTx.TxIn {
		found := false
		for _, utxo := range utxos {
			if utxo.Hash.Equal(&input.PreviousOutPoint.Hash) &&
				utxo.Index == input.PreviousOutPoint.Index {
				// Add input
				result.Inputs[i] = &InputSupplement{
					LockingScript: utxo.LockingScript,
					Value:         utxo.Value,
					KeyID:         utxo.KeyID,
				}
				found = true
				break
			}
		}
		if !found {
			missingErr = ErrMissingInputData
		}
	}

	// Setup outputs
	result.Outputs = make([]*OutputSupplement, len(result.MsgTx.TxOut))
	for i, _ := range result.Outputs {
		result.Outputs[i] = &OutputSupplement{}
	}

	return &result, missingErr
}

// Serialize returns the byte payload of the transaction.
func (tx *TxBuilder) Serialize() ([]byte, error) {
	var buf bytes.Buffer

	if err := tx.MsgTx.Serialize(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (tx *TxBuilder) String(net bitcoin.Network) string {
	result := fmt.Sprintf("TxId: %s (%d bytes) (%d estimated / %d fee)\n", tx.MsgTx.TxHash(),
		tx.MsgTx.SerializeSize(), tx.EstimatedSize(), tx.Fee())
	result += fmt.Sprintf("  Version: %d\n", tx.MsgTx.Version)
	result += "  Inputs:\n\n"
	for i, input := range tx.MsgTx.TxIn {
		result += fmt.Sprintf("    Outpoint: %d - %s\n", input.PreviousOutPoint.Index,
			input.PreviousOutPoint.Hash.String())
		result += fmt.Sprintf("    UnlockingScript: %s\n", input.UnlockingScript)
		result += fmt.Sprintf("    Sequence: %x\n", input.Sequence)

		ra, err := bitcoin.RawAddressFromLockingScript(tx.Inputs[i].LockingScript)
		if err == nil {
			result += fmt.Sprintf("    Address: %s\n",
				bitcoin.NewAddressFromRawAddress(ra, net).String())
		}
		result += fmt.Sprintf("    LockingScript: %s\n", tx.Inputs[i].LockingScript)
		result += fmt.Sprintf("    Value: %d\n", tx.Inputs[i].Value)
		if len(tx.Inputs[i].KeyID) > 0 {
			result += fmt.Sprintf("    KeyID : %s\n", tx.Inputs[i].KeyID)
		}
		result += "\n"
	}
	result += "  Outputs:\n\n"
	for i, output := range tx.MsgTx.TxOut {
		result += fmt.Sprintf("    Value: %.08f\n", float32(output.Value)/100000000.0)
		result += fmt.Sprintf("    LockingScript: %s\n", output.LockingScript)
		ra, err := bitcoin.RawAddressFromLockingScript(output.LockingScript)
		if err == nil {
			result += fmt.Sprintf("    Address: %s\n",
				bitcoin.NewAddressFromRawAddress(ra, net).String())
		}
		if len(tx.Outputs[i].KeyID) > 0 {
			result += fmt.Sprintf("    KeyID : %s\n", tx.Outputs[i].KeyID)
		}
		if tx.Outputs[i].IsRemainder {
			result += fmt.Sprintf("    IsRemainder\n")
		}
		if tx.Outputs[i].IsDust {
			result += fmt.Sprintf("    IsDust\n")
		}
		result += "\n"
	}
	result += fmt.Sprintf("  LockTime: %d\n", tx.MsgTx.LockTime)
	return result
}

func (tx *TxBuilder) TxID() bitcoin.Hash32 {
	return *tx.MsgTx.TxHash()
}

func (tx *TxBuilder) GetMsgTx() *wire.MsgTx {
	return tx.MsgTx
}

func (tx *TxBuilder) InputCount() int {
	return len(tx.MsgTx.TxIn)
}

func (tx *TxBuilder) Input(index int) *wire.TxIn {
	return tx.MsgTx.TxIn[index]
}

func (tx *TxBuilder) InputOutput(index int) (*wire.TxOut, error) {
	if index >= len(tx.Inputs) {
		return nil, errors.New("Missing input output")
	}

	input := tx.Inputs[index]
	return &wire.TxOut{
		LockingScript: input.LockingScript,
		Value:         input.Value,
	}, nil
}

func (tx *TxBuilder) OutputCount() int {
	return len(tx.MsgTx.TxOut)
}

func (tx *TxBuilder) Output(index int) *wire.TxOut {
	return tx.MsgTx.TxOut[index]
}

package expanded_tx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	// ErrNegativeFee means the tx has more output value than input value.
	ErrNegativeFee = errors.New("Negative Fee")
)

// Transaction is a transaction with only the wire encoded data provided.
type Transaction interface {
	TxID() bitcoin.Hash32
	GetMsgTx() *wire.MsgTx

	InputCount() int
	Input(index int) *wire.TxIn

	OutputCount() int
	Output(index int) *wire.TxOut
}

// TransactionWithOutputs is a transaction with spent outputs provided.
type TransactionWithOutputs interface {
	Transaction
	InputOutput(index int) (*wire.TxOut, error) // The output being spent by the input
}

// ExpandedTx is a Bitcoin transaction with ancestor information.
// All ancestor transactions back to merkle proofs should be provided.
type ExpandedTx struct {
	Tx        *wire.MsgTx `bsor:"1" json:"tx"`                  // marshals as hex bytes
	Ancestors AncestorTxs `bsor:"2" json:"ancestors,omitempty"` // ancestor history of outputs up to merkle proofs

	// SpentOutputs are the outputs spent by the inputs of Tx. If any outputs are specified then the
	// length of the slice must match the number of inputs and the indexes must align. For example,
	// the second output must correspond to the second input of Tx.
	SpentOutputs []*Output `bsor:"3" json:"spent_outputs,omitempty"`
}

// Output represents an output in a bitcoin transaction.
type Output struct {
	Value         uint64         `bsor:"1" json:"value"`
	LockingScript bitcoin.Script `bsor:"2" json:"locking_script"`
}

// jsonOutput is used to override the text marshalers when encoding and decoding Outputs.
type jsonOutput struct {
	Value         uint64         `bsor:"1" json:"value"`
	LockingScript bitcoin.Script `bsor:"2" json:"locking_script"`
}

func (o Output) String() string {
	return fmt.Sprintf("%d: %s", o.Value, o.LockingScript)
}

func (o Output) MarshalText() ([]byte, error) {
	return []byte(o.String()), nil
}

func (o *Output) UnmarshalText(text []byte) error {
	parts := strings.Split(string(text), ":")
	if len(parts) != 2 {
		return errors.New("Wrong \":\" count")
	}

	value, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return errors.Wrap(err, "value")
	}
	o.Value = value

	script, err := bitcoin.StringToScript(strings.TrimSpace(parts[1]))
	if err != nil {
		return errors.Wrap(err, "locking script")
	}
	o.LockingScript = script

	return nil
}

// MarshalJSON converts to json.
func (o Output) MarshalJSON() ([]byte, error) {
	jo := jsonOutput{
		Value:         o.Value,
		LockingScript: o.LockingScript,
	}
	return json.Marshal(jo)
}

// UnmarshalJSON converts from json.
func (o *Output) UnmarshalJSON(data []byte) error {
	jo := &jsonOutput{}
	if err := json.Unmarshal(data, jo); err != nil {
		return err
	}

	o.Value = jo.Value
	o.LockingScript = jo.LockingScript

	return nil
}

// NewExpandedTxFromTransactionWithOutputs creates a simple expanded tx from a
// TransactionWithOutputs interface. It will only have spent outputs and not ancestors.
func NewExpandedTxFromTransactionWithOutputs(tx TransactionWithOutputs) (*ExpandedTx, error) {
	inputCount := tx.InputCount()
	result := &ExpandedTx{
		Tx:           tx.GetMsgTx(),
		SpentOutputs: make([]*Output, inputCount),
	}

	// Setup inputs
	var missingErr error
	for index := 0; index < inputCount; index++ {
		output, err := tx.InputOutput(index)
		if err != nil {
			return nil, errors.Wrapf(err, "input %d output", index)
		}

		result.SpentOutputs[index] = &Output{
			LockingScript: output.LockingScript,
			Value:         output.Value,
		}
	}

	return result, missingErr
}

func (etx ExpandedTx) String() string {
	result := &bytes.Buffer{}
	if etx.Tx != nil {
		result.Write([]byte(fmt.Sprintf("%s\n", etx.Tx.String())))
	}

	fee, err := etx.CalculateFee()
	if err == nil {
		result.Write([]byte(fmt.Sprintf("Fee: %d (%04f sat/byte)\n", fee,
			float32(fee)/float32(etx.Tx.SerializeSize()))))
	} else {
		result.Write([]byte(fmt.Sprintf("Fee: %s\n", err)))
	}

	result.Write([]byte(etx.Ancestors.String()))

	if len(etx.SpentOutputs) > 0 {
		result.Write([]byte("Spent Outputs:\n"))
		for _, output := range etx.SpentOutputs {
			result.Write([]byte(fmt.Sprintf("  %s\n", output)))
		}
	}

	return string(result.Bytes())
}

func (etx ExpandedTx) StringWithAddresses(net bitcoin.Network) string {
	result := &bytes.Buffer{}
	if etx.Tx != nil {
		result.Write([]byte(fmt.Sprintf("%s\n", etx.Tx.StringWithAddresses(net))))
	}

	result.Write([]byte(etx.Ancestors.StringWithAddresses(net)))

	return string(result.Bytes())
}

// CalculateFee calculates the mining fee paid by the tx.
// Note: If transactions contianing outputs spent by the inputs are not included in ancestors then
// `MissingInput` will be returned.
func (etx ExpandedTx) CalculateFee() (uint64, error) {
	inputValue := uint64(0)
	for index := range etx.Tx.TxIn {
		value, err := etx.InputValue(index)
		if err != nil {
			return 0, errors.Wrapf(err, "input %d", index)
		}

		inputValue += value
	}

	outputValue := uint64(0)
	for _, txout := range etx.Tx.TxOut {
		outputValue += txout.Value
	}

	if outputValue > inputValue {
		return 0, ErrNegativeFee
	}

	return inputValue - outputValue, nil
}

func (etx ExpandedTx) TxID() bitcoin.Hash32 {
	return *etx.Tx.TxHash()
}

func (etx ExpandedTx) InputCount() int {
	return len(etx.Tx.TxIn)
}

func (etx ExpandedTx) Input(index int) *wire.TxIn {
	return etx.Tx.TxIn[index]
}

// VerifyInputs returns MissingInput error if all spent outputs are not included in the tx.
func (etx ExpandedTx) VerifyInputs() error {
	if etx.Tx == nil {
		return errors.Wrap(MissingInput, "missing tx")
	}

	for index := range etx.Tx.TxIn {
		if index < len(etx.SpentOutputs) {
			continue // output in spent outputs
		}

		// Check for spent output in ancestors
		txin := etx.Tx.TxIn[index]

		parentTx := etx.Ancestors.GetTx(txin.PreviousOutPoint.Hash)
		if parentTx == nil {
			return errors.Wrap(MissingInput, "parent: "+txin.PreviousOutPoint.Hash.String())
		}

		tx := parentTx.GetTx()
		if tx == nil {
			return errors.Wrap(MissingInput, "parent tx: "+txin.PreviousOutPoint.Hash.String())
		}

		if txin.PreviousOutPoint.Index >= uint32(len(tx.TxOut)) {
			return errors.Wrapf(MissingInput, "outpoint index out of range: %s",
				txin.PreviousOutPoint)
		}
	}

	return nil
}

// VerifyAncestors returns a wrapped MissingInput error if ancestors are not provided for all inputs
// in the tx. Note that it will return in error even if spent outputs are provided since they are
// harder to verify.
func (etx ExpandedTx) VerifyAncestors() error {
	if etx.Tx == nil {
		return errors.Wrap(MissingInput, "missing tx")
	}

	for index := range etx.Tx.TxIn {
		// Check for spent output in ancestors
		txin := etx.Tx.TxIn[index]

		parentTx := etx.Ancestors.GetTx(txin.PreviousOutPoint.Hash)
		if parentTx == nil {
			return errors.Wrap(MissingInput, "parent: "+txin.PreviousOutPoint.Hash.String())
		}

		tx := parentTx.GetTx()
		if tx == nil {
			return errors.Wrap(MissingInput, "parent tx: "+txin.PreviousOutPoint.Hash.String())
		}

		if txin.PreviousOutPoint.Index >= uint32(len(tx.TxOut)) {
			return errors.Wrapf(MissingInput, "outpoint index out of range: %s",
				txin.PreviousOutPoint)
		}
	}

	return nil
}

func (etx ExpandedTx) InputLockingScript(index int) (bitcoin.Script, error) {
	if index >= len(etx.Tx.TxIn) {
		return nil, errors.New("Index out of range")
	}

	if index < len(etx.SpentOutputs) {
		return etx.SpentOutputs[index].LockingScript, nil
	}

	txin := etx.Tx.TxIn[index]

	parentTx := etx.Ancestors.GetTx(txin.PreviousOutPoint.Hash)
	if parentTx == nil {
		return nil, errors.Wrap(MissingInput, "parent:"+txin.PreviousOutPoint.Hash.String())
	}

	tx := parentTx.GetTx()
	if tx == nil {
		return nil, errors.Wrap(MissingInput, "parent tx:"+txin.PreviousOutPoint.Hash.String())
	}

	if txin.PreviousOutPoint.Index >= uint32(len(tx.TxOut)) {
		return nil, errors.Wrap(MissingInput, txin.PreviousOutPoint.String())
	}

	return tx.TxOut[txin.PreviousOutPoint.Index].LockingScript, nil
}

func (etx ExpandedTx) InputValue(index int) (uint64, error) {
	if index >= len(etx.Tx.TxIn) {
		return 0, errors.New("Index out of range")
	}

	if index < len(etx.SpentOutputs) {
		return etx.SpentOutputs[index].Value, nil
	}

	txin := etx.Tx.TxIn[index]

	parentTx := etx.Ancestors.GetTx(txin.PreviousOutPoint.Hash)
	if parentTx == nil {
		return 0, errors.Wrap(MissingInput, "parent:"+txin.PreviousOutPoint.Hash.String())
	}

	tx := parentTx.GetTx()
	if tx == nil {
		return 0, errors.Wrap(MissingInput, "parent tx:"+txin.PreviousOutPoint.Hash.String())
	}

	if txin.PreviousOutPoint.Index >= uint32(len(tx.TxOut)) {
		return 0, errors.Wrap(MissingInput, txin.PreviousOutPoint.String())
	}

	return tx.TxOut[txin.PreviousOutPoint.Index].Value, nil
}

func (etx ExpandedTx) GetMsgTx() *wire.MsgTx {
	return etx.Tx
}

func (etx ExpandedTx) InputOutput(index int) (*wire.TxOut, error) {
	if index >= len(etx.Tx.TxIn) {
		return nil, errors.New("Index out of range")
	}

	if index < len(etx.SpentOutputs) {
		return &wire.TxOut{
			LockingScript: etx.SpentOutputs[index].LockingScript,
			Value:         etx.SpentOutputs[index].Value,
		}, nil
	}

	txin := etx.Tx.TxIn[index]

	parentTx := etx.Ancestors.GetTx(txin.PreviousOutPoint.Hash)
	if parentTx == nil {
		return nil, errors.Wrap(MissingInput, "parent:"+txin.PreviousOutPoint.Hash.String())
	}

	tx := parentTx.GetTx()
	if tx == nil {
		return nil, errors.Wrap(MissingInput, "parent tx:"+txin.PreviousOutPoint.Hash.String())
	}

	if txin.PreviousOutPoint.Index >= uint32(len(tx.TxOut)) {
		return nil, errors.Wrap(MissingInput, txin.PreviousOutPoint.String())
	}

	return tx.TxOut[txin.PreviousOutPoint.Index], nil
}

func (etx ExpandedTx) OutputCount() int {
	return len(etx.Tx.TxOut)
}

func (etx ExpandedTx) Output(index int) *wire.TxOut {
	return etx.Tx.TxOut[index]
}

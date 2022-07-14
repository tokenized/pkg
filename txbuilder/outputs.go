package txbuilder

import (
	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

// DustLimit calculates the dust limit
func DustLimit(outputSize int, feeRate float32) uint64 {
	if feeRate == 0 {
		return uint64(1)
	}

	dust := float32((outputSize+DustInputSize)*3) * feeRate
	if dust < 1 {
		return uint64(1)
	}
	return uint64(dust)
}

// DustLimitForOutput calculates the dust limit
func DustLimitForOutput(output *wire.TxOut, feeRate float32) uint64 {
	return DustLimit(output.SerializeSize(), feeRate)
}

// DustLimitForAddress calculates the dust limit
func DustLimitForAddress(ra bitcoin.RawAddress, feeRate float32) (uint64, error) {
	lockingScript, err := ra.LockingScript()
	if err != nil {
		return 0, errors.Wrap(err, "address locking script")
	}
	output := &wire.TxOut{
		LockingScript: lockingScript,
	}
	return DustLimitForOutput(output, feeRate), nil
}

// DustLimitForLockingScript calculates the dust limit
func DustLimitForLockingScript(lockingScript bitcoin.Script, feeRate float32) uint64 {
	output := &wire.TxOut{
		LockingScript: lockingScript,
	}
	return DustLimitForOutput(output, feeRate)
}

// OutputFeeAndDustForLockingScript returns the tx fee required to include the locking script as an
// output in a tx and the dust limit of that output.
func OutputFeeAndDustForLockingScript(lockingScript bitcoin.Script,
	dustFeeRate, feeRate float32) (uint64, uint64) {

	output := &wire.TxOut{
		LockingScript: lockingScript,
	}
	outputSize := output.SerializeSize()

	return EstimatedFeeValue(uint64(outputSize), float64(feeRate)), DustLimit(outputSize, dustFeeRate)
}

// OutputFeeAndDustForAddress returns the tx fee required to include the address as an output in a
// tx and the dust limit of that output.
func OutputFeeAndDustForAddress(ra bitcoin.RawAddress, dustFeeRate,
	feeRate float32) (uint64, uint64, error) {

	lockingScript, err := ra.LockingScript()
	if err != nil {
		return 0, 0, errors.Wrap(err, "address locking script")
	}
	f, d := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate, feeRate)
	return f, d, nil
}

// OutputAddress returns the address that the output is paying to.
func (tx *TxBuilder) OutputAddress(index int) (bitcoin.RawAddress, error) {
	if index >= len(tx.MsgTx.TxOut) {
		return bitcoin.RawAddress{}, errors.New("Output index out of range")
	}
	return bitcoin.RawAddressFromLockingScript(tx.MsgTx.TxOut[index].LockingScript)
}

func (tx *TxBuilder) SetChangeAddress(address bitcoin.RawAddress, keyID string) error {
	changeScript, err := address.LockingScript()
	if err != nil {
		return err
	}

	return tx.SetChangeLockingScript(changeScript, keyID)
}

func (tx *TxBuilder) SetChangeLockingScript(lockingScript bitcoin.Script, keyID string) error {
	tx.ChangeScript = lockingScript
	tx.ChangeKeyID = keyID

	// Update existing outputs
	for i, output := range tx.MsgTx.TxOut {
		if output.LockingScript.Equal(lockingScript) {
			tx.Outputs[i].IsRemainder = true
			tx.Outputs[i].KeyID = keyID
		}
	}

	return nil
}

// AddPaymentOutput adds an output to TxBuilder with the specified value and a script paying the
//   specified address.
// isRemainder marks the output to receive remaining bitcoin after fees are taken.
func (tx *TxBuilder) AddPaymentOutput(address bitcoin.RawAddress, value uint64,
	isRemainder bool) error {

	script, err := address.LockingScript()
	if err != nil {
		return err
	}
	return tx.AddOutput(script, value, isRemainder, false)
}

// AddP2PKHDustOutput adds an output to TxBuilder with the dust limit amount and a script paying the
//   specified address.
// isRemainder marks the output to receive remaining bitcoin.
// These dust outputs are meant as "notifiers" so that an address will see this transaction and
//   process the data in it. If value is later added to this output, the value replaces the dust
//   limit amount rather than adding to it.
func (tx *TxBuilder) AddDustOutput(address bitcoin.RawAddress, isRemainder bool) error {
	script, err := address.LockingScript()
	if err != nil {
		return err
	}
	return tx.AddOutput(script, 0, isRemainder, true)
}

// AddMaxOutput adds an output to TxBuilder with a script paying the specified address and the
// remainder flag set so that it gets all remaining value after fees are taken.
func (tx *TxBuilder) AddMaxOutput(address bitcoin.RawAddress) error {
	tx.SendMax = true
	script, err := address.LockingScript()
	if err != nil {
		return err
	}
	return tx.AddOutput(script, 0, true, false)
}

// AddMaxOutput adds an output to TxBuilder with a script paying the specified address and the
// remainder flag set so that it gets all remaining value after fees are taken.
func (tx *TxBuilder) AddMaxScriptOutput(script bitcoin.Script) error {
	tx.SendMax = true
	return tx.AddOutput(script, 0, true, false)
}

// AddOutput adds an output to TxBuilder with the specified script and value.
// isRemainder marks the output to receive remaining bitcoin after fees are taken.
// isDust marks the output as a dust amount which will be replaced by any non-dust amount if an
//   amount is added later. It also sets the amount to the calculated dust value.
func (tx *TxBuilder) AddOutput(lockScript bitcoin.Script, value uint64,
	isRemainder, isDust bool) error {

	output := &OutputSupplement{
		IsRemainder: isRemainder,
		IsDust:      isDust,
	}

	txout := &wire.TxOut{
		Value:         value,
		LockingScript: lockScript,
	}

	dust := DustLimitForOutput(txout, tx.DustFeeRate)
	if isDust {
		txout.Value = dust
	} else if value < dust && (!tx.SendMax || !isRemainder) &&
		!bitcoin.LockingScriptIsUnspendable(lockScript) {
		// Below dust and not send max output
		return ErrBelowDustValue
	}

	tx.Outputs = append(tx.Outputs, output)
	tx.MsgTx.AddTxOut(txout)
	return nil
}

// InsertOutput inserts an output into TxBuilder at the specified index.
func (tx *TxBuilder) InsertOutput(index int, lockScript bitcoin.Script, value uint64,
	isRemainder, isDust bool) error {

	output := &OutputSupplement{
		IsRemainder: isRemainder,
		IsDust:      isDust,
	}

	txout := &wire.TxOut{
		Value:         value,
		LockingScript: lockScript,
	}

	dust := DustLimitForOutput(txout, tx.DustFeeRate)
	if isDust {
		txout.Value = dust
	} else if value < dust && (!tx.SendMax || !isRemainder) &&
		!bitcoin.LockingScriptIsUnspendable(lockScript) {
		// Below dust and not send max output
		return ErrBelowDustValue
	}

	afterOutputs := make([]*OutputSupplement, len(tx.Outputs)-index)
	copy(afterOutputs, tx.Outputs[index:])
	tx.Outputs = append(append(tx.Outputs[:index], output), afterOutputs...)

	afterTxOut := make([]*wire.TxOut, len(tx.MsgTx.TxOut)-index)
	copy(afterTxOut, tx.MsgTx.TxOut[index:])
	tx.MsgTx.TxOut = append(append(tx.MsgTx.TxOut[:index], txout), afterTxOut...)

	return nil
}

func (tx *TxBuilder) RemoveOutput(index int) error {
	if index >= len(tx.Outputs) || index >= len(tx.MsgTx.TxOut) {
		return errors.New("Output index out of range")
	}

	tx.Outputs = append(tx.Outputs[:index], tx.Outputs[index+1:]...)
	tx.MsgTx.TxOut = append(tx.MsgTx.TxOut[:index], tx.MsgTx.TxOut[index+1:]...)
	return nil
}

// AddValueToOutput adds more bitcoin to an existing output.
func (tx *TxBuilder) AddValueToOutput(index uint32, value uint64) error {
	if int(index) >= len(tx.MsgTx.TxOut) {
		return errors.New("Output index out of range")
	}

	if tx.Outputs[index].IsDust {
		if value < DustLimitForOutput(tx.MsgTx.TxOut[index], tx.DustFeeRate) {
			return ErrBelowDustValue
		}
		tx.Outputs[index].IsDust = false
		tx.MsgTx.TxOut[index].Value = value
	} else {
		tx.MsgTx.TxOut[index].Value += value
	}
	return nil
}

// SetOutputToDust sets an outputs value to dust.
func (tx *TxBuilder) SetOutputToDust(index uint32) error {
	if int(index) >= len(tx.MsgTx.TxOut) {
		return errors.New("Output index out of range")
	}

	tx.Outputs[index].IsDust = true
	tx.MsgTx.TxOut[index].Value = DustLimitForOutput(tx.MsgTx.TxOut[index], tx.DustFeeRate)
	return nil
}

// UpdateOutput updates the locking script of an output.
func (tx *TxBuilder) UpdateOutput(index uint32, lockingScript bitcoin.Script) error {
	if int(index) >= len(tx.MsgTx.TxOut) {
		return errors.New("Output index out of range")
	}

	tx.MsgTx.TxOut[index].LockingScript = lockingScript
	return nil
}

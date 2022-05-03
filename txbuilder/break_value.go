package txbuilder

import (
	"math/rand"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

var (
	// BreakIncrements are the base values used to generate random output values.
	BreakIncrements = []uint64{1, 2, 5}
)

// BreakValue breaks the value up into psuedo random values based on pre-defined increments of
// powers of the break value.
// It is recommended to provide at least 5 change addresses. More addresses means more privacy, but
// also more UTXOs and more tx fees.
// breakValue should be a fairly low value that is the smallest UTXO you want created other than
// the remainder.
func BreakValue(value, breakValue uint64, addresses []AddressKeyID,
	dustFeeRate, feeRate float32, lastIsRemainder bool, subtractFee bool) ([]*Output, error) {
	// Choose random multiples of breakValue until the value is taken up.

	// Find the average value to break the value into the provided addresses
	average := 2 * (value / uint64(len(addresses)-1))

	// Find the power to use for choosing random values
	factor := average / breakValue
	var exponent int
	if factor >= 500 {
		exponent = 4
	} else if factor >= 50 {
		exponent = 3
	} else if factor >= 5 {
		exponent = 2
	} else {
		exponent = 1
	}

	// Calculate some random values
	remaining := value
	rand.Seed(time.Now().UnixNano())
	result := make([]*Output, 0, len(addresses))
	nextIndex := 0
	for _, address := range addresses[:len(addresses)-1] {
		lockingScript, err := address.Address.LockingScript()
		if err != nil {
			return nil, errors.Wrap(err, "locking script")
		}

		outputFee, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate,
			feeRate)

		if subtractFee {
			if remaining <= outputFee {
				break // remaining amount is less than fee to include another output
			}
			remaining -= outputFee

			if remaining <= dustLimit || remaining < breakValue {
				remaining += outputFee // abort adding this output, so add the fee for it back in
				break                  // remaining amount is less than dust required to include next address
			}
		} else if remaining <= dustLimit || remaining < breakValue {
			break // remaining amount is less than dust required to include next address
		}

		inc := BreakIncrements[rand.Intn(len(BreakIncrements))]
		outputValue := breakValue * inc
		switch rand.Intn(exponent) {
		case 0: // *= 1
		case 1:
			outputValue *= 10
		case 2:
			outputValue *= 100
		case 3:
			outputValue *= 1000
		}

		if rand.Intn(2) == 1 {
			outputValue = outputValue - (outputValue / 10) +
				uint64(rand.Int63n(int64(outputValue/5)))
		}

		if outputValue > remaining {
			outputValue = remaining
		}

		remaining -= outputValue

		result = append(result, &Output{
			TxOut: wire.TxOut{
				Value:         outputValue,
				LockingScript: lockingScript,
			},
			Supplement: OutputSupplement{
				KeyID: address.KeyID,
			},
		})
		nextIndex++
	}

	// Add any remainder to last output
	lockingScript, err := addresses[nextIndex].Address.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "locking script")
	}

	if subtractFee {
		outputFee, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate,
			feeRate)

		if remaining > outputFee+dustLimit {
			remaining -= outputFee

			result = append(result, &Output{
				TxOut: wire.TxOut{
					Value:         remaining,
					LockingScript: lockingScript,
				},
				Supplement: OutputSupplement{
					IsRemainder: lastIsRemainder,
					KeyID:       addresses[nextIndex].KeyID,
				},
			})
		} else if len(result) > 0 {
			// Add to last output
			result[len(result)-1].Supplement.IsRemainder = lastIsRemainder
			result[len(result)-1].TxOut.Value += remaining
		}
	} else {
		_, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate, feeRate)

		if remaining > dustLimit {
			result = append(result, &Output{
				TxOut: wire.TxOut{
					Value:         remaining,
					LockingScript: lockingScript,
				},
				Supplement: OutputSupplement{
					IsRemainder: lastIsRemainder,
					KeyID:       addresses[nextIndex].KeyID,
				},
			})
		} else if len(result) > 0 {
			// Add to last output
			result[len(result)-1].Supplement.IsRemainder = lastIsRemainder
			result[len(result)-1].TxOut.Value += remaining
		}
	}

	// Random sort outputs
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

func BreakValueLockingScripts(value, breakValue uint64, lockingScripts []bitcoin.Script,
	dustFeeRate, feeRate float32, lastIsRemainder bool, subtractFee bool) ([]*Output, error) {
	// Choose random multiples of breakValue until the value is taken up.

	// Find the average value to break the value into the provided lockingScripts
	average := 2 * (value / uint64(len(lockingScripts)-1))

	// Find the power to use for choosing random values
	factor := average / breakValue
	var exponent int
	if factor >= 500 {
		exponent = 4
	} else if factor >= 50 {
		exponent = 3
	} else if factor >= 5 {
		exponent = 2
	} else {
		exponent = 1
	}

	// Calculate some random values
	remaining := value
	rand.Seed(time.Now().UnixNano())
	result := make([]*Output, 0, len(lockingScripts))
	nextIndex := 0
	for _, lockingScript := range lockingScripts[:len(lockingScripts)-1] {
		outputFee, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate,
			feeRate)

		if subtractFee {
			if remaining <= outputFee {
				break // remaining amount is less than fee to include another output
			}
			remaining -= outputFee

			if remaining <= dustLimit || remaining < breakValue {
				remaining += outputFee // abort adding this output, so add the fee for it back in
				break                  // remaining amount is less than dust required to include next address
			}
		} else if remaining <= dustLimit || remaining < breakValue {
			break // remaining amount is less than dust required to include next address
		}

		inc := BreakIncrements[rand.Intn(len(BreakIncrements))]
		outputValue := breakValue * inc
		switch rand.Intn(exponent) {
		case 0: // *= 1
		case 1:
			outputValue *= 10
		case 2:
			outputValue *= 100
		case 3:
			outputValue *= 1000
		}

		if rand.Intn(2) == 1 {
			outputValue = outputValue - (outputValue / 10) +
				uint64(rand.Int63n(int64(outputValue/5)))
		}

		if outputValue > remaining {
			outputValue = remaining
		}

		remaining -= outputValue

		result = append(result, &Output{
			TxOut: wire.TxOut{
				Value:         outputValue,
				LockingScript: lockingScript,
			},
		})
		nextIndex++
	}

	// Add any remainder to last output
	lockingScript := lockingScripts[nextIndex]
	if subtractFee {
		outputFee, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate,
			feeRate)

		if remaining > outputFee+dustLimit {
			remaining -= outputFee

			result = append(result, &Output{
				TxOut: wire.TxOut{
					Value:         remaining,
					LockingScript: lockingScript,
				},
				Supplement: OutputSupplement{
					IsRemainder: lastIsRemainder,
				},
			})
		} else if len(result) > 0 {
			// Add to last output
			result[len(result)-1].Supplement.IsRemainder = lastIsRemainder
			result[len(result)-1].TxOut.Value += remaining
		}
	} else {
		_, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate, feeRate)

		if remaining > dustLimit {
			result = append(result, &Output{
				TxOut: wire.TxOut{
					Value:         remaining,
					LockingScript: lockingScript,
				},
				Supplement: OutputSupplement{
					IsRemainder: lastIsRemainder,
				},
			})
		} else if len(result) > 0 {
			// Add to last output
			result[len(result)-1].Supplement.IsRemainder = lastIsRemainder
			result[len(result)-1].TxOut.Value += remaining
		}
	}

	// Random sort outputs
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

// BreakQuantity breaks the quantity into randomized values. It is like BreakValue, but is not
// concerned with dust or anything like that.
func BreakQuantity(value, breakValue uint64, count int) ([]uint64, error) {
	// Choose random multiples of breakValue until the value is taken up.

	// Find the average value to break the value into the provided addresses
	average := 2 * (value / uint64(count-1))

	// Find the power to use for choosing random values
	factor := average / breakValue
	var exponent int
	if factor >= 500 {
		exponent = 4
	} else if factor >= 50 {
		exponent = 3
	} else if factor >= 5 {
		exponent = 2
	} else {
		exponent = 1
	}

	// Calculate some random values
	remaining := value
	rand.Seed(time.Now().UnixNano())
	result := make([]uint64, 0, count)
	nextIndex := 0
	for i := 0; i < count; i++ {
		if remaining < breakValue {
			break // remaining amount is less than dust required to include next address
		}

		inc := BreakIncrements[rand.Intn(len(BreakIncrements))]
		quantity := breakValue * inc
		switch rand.Intn(exponent) {
		case 0: // *= 1
		case 1:
			quantity *= 10
		case 2:
			quantity *= 100
		case 3:
			quantity *= 1000
		}

		if quantity >= 10 && rand.Intn(2) == 1 {
			quantity = quantity - (quantity / 10) + uint64(rand.Int63n(int64(quantity/5)))
		}

		if quantity > remaining {
			quantity = remaining
		}

		remaining -= quantity

		result = append(result, quantity)
		nextIndex++
	}

	if remaining > 0 {
		result = append(result, remaining)
	} else if len(result) > 1 {
		// Add to last quantity
		result[len(result)-1] += remaining
	}

	// Random sort outputs
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

// AddOutputs appends the specified outputs to the tx.
func (tx *TxBuilder) AddOutputs(outputs []*Output) {
	for _, output := range outputs {
		tx.MsgTx.AddTxOut(&output.TxOut)
		tx.Outputs = append(tx.Outputs, &output.Supplement)
	}
}

// RandomizeOutputs randomly sorts the outputs of the tx. Be careful not to do this on txs that
// depend on outputs being at a specific index.
func (tx *TxBuilder) RandomizeOutputs() {
	outputs := make([]*Output, len(tx.MsgTx.TxOut))
	for i, txout := range tx.MsgTx.TxOut {
		outputs[i] = &Output{
			TxOut:      *txout,
			Supplement: *tx.Outputs[i],
		}
	}

	rand.Shuffle(len(outputs), func(i, j int) {
		outputs[i], outputs[j] = outputs[j], outputs[i]
	})

	tx.MsgTx.TxOut = make([]*wire.TxOut, len(outputs))
	tx.Outputs = make([]*OutputSupplement, len(outputs))
	for i, _ := range outputs {
		tx.MsgTx.TxOut[i] = &outputs[i].TxOut
		tx.Outputs[i] = &outputs[i].Supplement
	}
}

// RandomizeOutputsAfter randomly sorts only the outputs of the tx after the specified index. For
// example an index of zero will leave the first output in place. Be careful not to do this on txs
// that depend on outputs being at a specific index.
func (tx *TxBuilder) RandomizeOutputsAfter(index int) {
	randLength := len(tx.MsgTx.TxOut) - (index + 1)
	randOutputs := make([]*Output, randLength)
	for i, txout := range tx.MsgTx.TxOut[index+1:] {
		randOutputs[i] = &Output{
			TxOut:      *txout,
			Supplement: *tx.Outputs[i],
		}
	}

	tx.MsgTx.TxOut = tx.MsgTx.TxOut[:index+1]
	tx.Outputs = tx.Outputs[:index+1]

	rand.Shuffle(randLength, func(i, j int) {
		randOutputs[i], randOutputs[j] = randOutputs[j], randOutputs[i]
	})

	for i, _ := range randOutputs {
		tx.MsgTx.TxOut = append(tx.MsgTx.TxOut, &randOutputs[i].TxOut)
		tx.Outputs = append(tx.Outputs, &randOutputs[i].Supplement)
	}
}

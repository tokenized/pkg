package txbuilder

import (
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"github.com/tokenized/pkg/wire"
)

var (
	BreakIncrements = []uint64{1, 2, 5}
)

// BreakChange breaks the value up into psuedo random values based on pre-defined increments of
// powers of the break value.
// It is recommended to provide at least 5 change addresses. More addresses means more privacy, but
// also more UTXOs and more tx fees.
// breakValue should be a fairly low value that is the smallest UTXO you want created other than
// the remainder.
func BreakValue(value, breakValue uint64, changeAddresses []AddressKeyID,
	dustFeeRate, feeRate float32) ([]*Output, error) {
	// Choose random multiples of breakValue until the change is taken up.

	// Find the average value to break the change into the provided addresses
	average := 2 * (value / uint64(len(changeAddresses)-1))

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
	result := make([]*Output, 0, len(changeAddresses))
	nextIndex := 0
	for _, changeAddress := range changeAddresses[:len(changeAddresses)-1] {
		lockingScript, err := changeAddress.Address.LockingScript()
		if err != nil {
			return nil, errors.Wrap(err, "locking script")
		}

		outputFee, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate,
			feeRate)

		if remaining <= outputFee {
			break // remaining amount is less than fee to include another output
		}
		remaining -= outputFee

		if remaining <= dustLimit || remaining < breakValue {
			remaining += outputFee
			break // remaining amount is less than dust required to include next change address
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
			outputValue = outputValue - (outputValue / 10) + uint64(rand.Int63n(int64(outputValue/5)))
		}

		if outputValue > remaining {
			outputValue = remaining
		}

		remaining -= outputValue

		result = append(result, &Output{
			TxOut: wire.TxOut{
				Value:    outputValue,
				PkScript: lockingScript,
			},
			Supplement: OutputSupplement{
				KeyID: changeAddress.KeyID,
			},
		})
		nextIndex++
	}

	// Add any remainder to last output
	lockingScript, err := changeAddresses[nextIndex].Address.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "locking script")
	}

	outputFee, dustLimit := OutputFeeAndDustForLockingScript(lockingScript, dustFeeRate,
		feeRate)

	if remaining > outputFee+dustLimit {
		remaining -= outputFee

		result = append(result, &Output{
			TxOut: wire.TxOut{
				Value:    remaining,
				PkScript: lockingScript,
			},
			Supplement: OutputSupplement{
				IsRemainder: true,
				KeyID:       changeAddresses[nextIndex].KeyID,
			},
		})
	} else if len(result) > 1 {
		// Add to last output
		result[len(result)-1].Supplement.IsRemainder = true
		result[len(result)-1].TxOut.Value += remaining
	}

	// Random sort outputs
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

func (tx *TxBuilder) AddOutputs(outputs []*Output) {
	for _, output := range outputs {
		tx.MsgTx.AddTxOut(&output.TxOut)
		tx.Outputs = append(tx.Outputs, &output.Supplement)
	}
}

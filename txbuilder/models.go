package txbuilder

import (
	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

// InputSupplement contains data required to sign an input that is not already in the wire.MsgTx.
type InputSupplement struct {
	LockingScript []byte `json:"locking_script"`
	Value         uint64 `json:"value"`

	// Optional identifier for external use to track the key needed to sign the input.
	KeyID string `json:"key_id,omitempty"`
}

type AddressKeyID struct {
	Address bitcoin.RawAddress
	KeyID   string
}

// OutputSupplement contains data that is not contained in a tx message, but that is needed to
//   perform operations on the tx, like fee and change calculation.
type OutputSupplement struct {
	// Used when calculating fees to put remaining input value in.
	IsRemainder bool `json:"is_remainder"`

	// Used as a notification payment, but if value is added, then the previous dust amount isn't
	//   added to the new amount.
	IsDust bool `json:"is_dust"`

	// This output was added by the fee calculation and can be removed by the fee calculation.
	addedForFee bool

	// Optional identifier for external use to track the key needed to spend.
	KeyID string `json:"key_id,omitempty"`
}

type Output struct {
	wire.TxOut
	Supplement OutputSupplement
}

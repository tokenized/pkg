package client

import (
	"context"

	"github.com/pkg/errors"
)

var (
	ErrUnknownMessageType = errors.New("Unknown Message Type")
	ErrInvalid            = errors.New("Invalid")
	ErrIncomplete         = errors.New("Incomplete") // merkle proof is incomplete
	ErrWrongHash          = errors.New("Wrong Hash") // Non-matching merkle root hash
	ErrNotConnected       = errors.New("Not Connected")
	ErrWrongKey           = errors.New("Wrong Key") // The wrong key was provided during auth
	ErrBadSignature       = errors.New("Bad Signature")
	ErrTimeout            = errors.New("Timeout")
	ErrReject             = errors.New("Reject")
)

// Handler provides an interface for handling data from the spynode client.
type Handler interface {
	HandleTx(context.Context, *Tx)
	HandleTxUpdate(context.Context, *TxUpdate)
	HandleHeaders(context.Context, *Headers)
}

// Client is the interface for interacting with a spynode.
type Client interface {
	RegisterHandler(handler client.Handler)

	SubscribePushDatas(ctx context.Context, pushDatas [][]byte) error
	UnsubscribePushDatas(ctx context.Context, pushDatas [][]byte) error
	SubscribeContracts(ctx context.Context) error
	UnsubscribeContracts(ctx context.Context) error
	SubscribeHeaders(ctx context.Context) error
	UnsubscribeHeaders(ctx context.Context) error

	Ready(ctx context.Context) error
}

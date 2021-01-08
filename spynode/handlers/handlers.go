package handlers

import (
	"context"

	"github.com/tokenized/pkg/spynode/client"
	"github.com/tokenized/pkg/spynode/handlers/data"
	"github.com/tokenized/pkg/spynode/handlers/storage"
	"github.com/tokenized/pkg/wire"
)

// MessageHandler defines an interface for handing commands/messages received from
// peers over the Bitcoin P2P network.
type MessageHandler interface {
	Handle(context.Context, wire.Message) ([]wire.Message, error)
}

type StateReady interface {
	IsReady() bool
}

type IsRelevant interface {
	IsRelevent(context.Context, *wire.MsgTx) bool
}

// NewTrustedMessageHandlers returns a mapping of commands and Handler's.
func NewTrustedMessageHandlers(ctx context.Context, config data.Config, state *data.State,
	peers *storage.PeerRepository, blockRepo *storage.BlockRepository, blockRefeeder *BlockRefeeder,
	txRepo *storage.TxRepository, reorgRepo *storage.ReorgRepository, tracker *data.TxTracker,
	memPool *data.MemPool, unconfTxChannel *TxChannel, txStateChannel *TxStateChannel,
	handlers []client.Handler) map[string]MessageHandler {

	return map[string]MessageHandler{
		wire.CmdPing:    NewPingHandler(),
		wire.CmdVersion: NewVersionHandler(state, config.NodeAddress),
		wire.CmdAddr:    NewAddressHandler(peers),
		wire.CmdInv:     NewInvHandler(state, txRepo, tracker, memPool),
		wire.CmdTx:      NewTXHandler(state, unconfTxChannel),
		wire.CmdBlock:   NewBlockHandler(state, blockRefeeder),
		wire.CmdHeaders: NewHeadersHandler(config, state, txStateChannel, blockRepo, txRepo,
			reorgRepo, handlers),
		wire.CmdReject: NewRejectHandler(),
	}
}

// NewUntrustedMessageHandlers returns a mapping of commands and Handler's.
func NewUntrustedMessageHandlers(ctx context.Context, trustedState *data.State,
	untrustedState *data.UntrustedState, peers *storage.PeerRepository,
	blockRepo *storage.BlockRepository, tracker *data.TxTracker,
	memPool *data.MemPool, txChannel *TxChannel,
	isRelevant IsRelevant, address string) map[string]MessageHandler {

	return map[string]MessageHandler{
		wire.CmdPing:    NewPingHandler(),
		wire.CmdVersion: NewUntrustedVersionHandler(untrustedState, address),
		wire.CmdAddr:    NewAddressHandler(peers),
		wire.CmdInv:     NewUntrustedInvHandler(untrustedState, tracker, memPool),
		wire.CmdTx:      NewUntrustedTXHandler(untrustedState, txChannel),
		wire.CmdBlock:   NewBlockHandler(trustedState, nil),
		wire.CmdHeaders: NewUntrustedHeadersHandler(untrustedState, peers, address, blockRepo),
		wire.CmdReject:  NewRejectHandler(),
	}
}

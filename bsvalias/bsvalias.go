package bsvalias

import (
	"context"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

const (
	// URLNamePKI is the name used to identity the PKI (Public Key Infrastructure) URL and
	// capability.
	URLNamePKI = "pki"

	// URLNamePaymentDestination is the name used to identity the payment destination URL and
	// capability.
	URLNamePaymentDestination = "paymentDestination"

	// URLNamePaymentRequest is the name used to identity the payment request URL and
	// capability.
	URLNamePaymentRequest = "f7ecaab847eb"

	// RequireNameSenderValidation is a Capabilities key value that specifies if sender's are
	// required to include a sender handle and signature to validate the sender.
	// Set the value to true (a boolean, not string)
	RequireNameSenderValidation = "6745385c3fc0"

	// URLNameP2PPaymentDestination is the name used to identify the peer to peer payment
	// destination URL and capability.
	URLNameP2PPaymentDestination = "2a40af698840"

	// URLNameP2PTransactions is the name used to identify the peer to peer transactions URL and
	// capability.
	URLNameP2PTransactions = "5f1323cddf31"

	// URLNameListTokenizedAssetAlias is the name used to identify the list Tokenized asset alias
	// URL and capability.
	URLNameListTokenizedAssetAlias = "e243785d1f17"
)

var (
	// ErrInvalidHandle means the handle is formatted incorrectly or just invalid.
	ErrInvalidHandle = errors.New("Invalid handle")

	// ErrNotCapable means the host site does not support a feature being requested.
	ErrNotCapable = errors.New("Not capable")

	// ErrInvalidSignature means a signature is invalid.
	ErrInvalidSignature = errors.New("Invalid signature")

	// ErrNotFound means the requested entity was not found.
	ErrNotFound = errors.New("Not Found")

	// ErrWrongOutputCount means that the outputs supplied with a payment request do not match the
	// number of inputs.
	ErrWrongOutputCount = errors.New("Wrong Output Count")
)

// Factory is the interface for creating new bsvalias clients.
// This is mainly used for testing so actual HTTP calls can be replaced with an internal system.
type Factory interface {
	// NewClient creates a new client.
	NewClient(ctx context.Context, handle string) (Client, error)
}

// Client is the interface for interacting with an bsvalias oracle service.
type Client interface {
	// GetPublicKey gets the identity public key for the handle.
	GetPublicKey(ctx context.Context) (*bitcoin.PublicKey, error)

	// GetPaymentDestination requests a locking script that can be used to send bitcoin.
	// If senderKey is not nil then it must be associated with senderHandle and will be used to add
	// a signature to the request.
	GetPaymentDestination(ctx context.Context, senderName, senderHandle, purpose string,
		amount uint64, senderKey *bitcoin.Key) ([]byte, error)

	// GetPaymentRequest requests a payment request that can be used to send bitcoin or an asset.
	//   senderHandle is required.
	//   assetID can be empty or "BSV" to request bitcoin.
	// If senderKey is not nil then it must be associated with senderHandle and will be used to add
	// a signature to the request.
	GetPaymentRequest(ctx context.Context, senderName, senderHandle, purpose, assetID string,
		amount uint64, senderKey *bitcoin.Key) (*PaymentRequest, error)

	// GetP2PPaymentDestination requests a peer to peer payment destination.
	GetP2PPaymentDestination(ctx context.Context,
		value uint64) (*P2PPaymentDestinationOutputs, error)

	// PostP2PTransaction posts a P2P transaction to the handle being paid. The same as that used by
	// the corresponding GetP2PPaymentDestination. Returns a note that is returned from the
	// endpoint.
	PostP2PTransaction(ctx context.Context, senderHandle, note, reference string,
		senderKey *bitcoin.Key, tx *wire.MsgTx) (string, error)

	// ListTokenizedAssets returns the list of asset aliases for this paymail handle.
	ListTokenizedAssets(ctx context.Context) ([]AssetAlias, error)
}

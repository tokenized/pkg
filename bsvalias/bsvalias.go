package bsvalias

import (
	"context"

	"github.com/tokenized/pkg/bitcoin"

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
	GetPaymentDestination(senderName, senderHandle, purpose string, amount uint64,
		senderKey *bitcoin.Key) ([]byte, error)

	// GetPaymentRequest requests a payment request that can be used to send bitcoin or an asset.
	//   senderHandle is required.
	//   assetID can be empty or "BSV" to request bitcoin.
	// If senderKey is not nil then it must be associated with senderHandle and will be used to add
	// a signature to the request.
	GetPaymentRequest(senderName, senderHandle, purpose, assetID string, amount uint64,
		senderKey *bitcoin.Key) (*PaymentRequest, error)
}

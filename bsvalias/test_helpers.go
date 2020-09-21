package bsvalias

import (
	"context"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// MockClient represents a client for a paymail/bsvalias service for testing software built on top.
type MockClient struct {
	user *mockUser
}

// MockFactory is a factory for creating mock clients.
type MockFactory struct {
	users []*mockUser
}

func NewMockFactory() *MockFactory {
	return &MockFactory{}
}

// NewClient creates a new client.
func (f *MockFactory) NewClient(ctx context.Context, handle string) (Client, error) {
	for _, user := range f.users {
		if user.handle == handle {
			return &MockClient{user: user}, nil
		}
	}

	return nil, errors.Wrap(ErrInvalidHandle, "not found")
}

// mockUser is a mock user for testing systems that use paymail.
type mockUser struct {
	handle      string
	identityKey bitcoin.Key
	addressKey  bitcoin.Key
}

// AddMockUser adds a new mock user.
func (f *MockFactory) AddMockUser(handle string, identityKey, addressKey bitcoin.Key) {
	f.users = append(f.users, &mockUser{
		handle:      handle,
		identityKey: identityKey,
		addressKey:  addressKey,
	})
}

// GenerateMockUser generates a mock user and returns the user's handle, public key, and address.
func (f *MockFactory) GenerateMockUser(host string,
	net bitcoin.Network) (*string, *bitcoin.PublicKey, *bitcoin.RawAddress, error) {

	result := &mockUser{
		handle: uuid.New().String() + "@" + host,
	}

	var err error
	result.identityKey, err = bitcoin.GenerateKey(net)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "generate identity key")
	}

	result.addressKey, err = bitcoin.GenerateKey(net)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "generate address key")
	}

	pk := result.identityKey.PublicKey()
	ra, err := result.addressKey.RawAddress()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "generate address")
	}

	f.users = append(f.users, result)
	return &result.handle, &pk, &ra, nil
}

// GetPublicKey gets the identity public key for the handle.
func (c *MockClient) GetPublicKey(ctx context.Context) (*bitcoin.PublicKey, error) {
	pk := c.user.identityKey.PublicKey()
	return &pk, nil
}

// GetPaymentDestination gets a locking script that can be used to send bitcoin.
// If senderKey is not nil then it must be associated with senderHandle and will be used to add
// a signature to the request.
func (c *MockClient) GetPaymentDestination(senderName, senderHandle, purpose string, amount uint64,
	senderKey *bitcoin.Key) ([]byte, error) {

	ra, err := c.user.addressKey.RawAddress()
	if err != nil {
		return nil, errors.Wrap(err, "raw address")
	}

	script, err := ra.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "locking script")
	}

	return script, nil
}

// GetPaymentRequest gets a payment request from the identity.
//   senderHandle is required.
//   assetID can be empty or "BSV" to request bitcoin.
// If senderKey is not nil then it must be associated with senderHandle and will be used to add
// a signature to the request.
func (c *MockClient) GetPaymentRequest(senderName, senderHandle, purpose, assetID string,
	amount uint64, senderKey *bitcoin.Key) (*PaymentRequest, error) {

	ra, err := c.user.addressKey.RawAddress()
	if err != nil {
		return nil, errors.Wrap(err, "raw address")
	}

	script, err := ra.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "locking script")
	}

	tx := wire.NewMsgTx(1)

	if assetID == "BSV" {
		tx.AddTxOut(wire.NewTxOut(amount, script))
	} else {
		// Note: requires contract address of asset and possibly access to mock identity oracle
		// client.
		return nil, errors.Wrap(ErrNotCapable, "not implemented in mock client")
	}

	return &PaymentRequest{
		Tx:      tx,
		Outputs: nil,
	}, nil
}

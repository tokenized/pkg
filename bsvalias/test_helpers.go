package bsvalias

import (
	"context"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/logger"
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
	handle            string
	identityKey       bitcoin.Key
	addressKey        bitcoin.Key
	p2pTxs            map[string][]*wire.MsgTx
	instrumentAliases []InstrumentAlias
}

// AddMockUser adds a new mock user.
func (f *MockFactory) AddMockUser(handle string, identityKey, addressKey bitcoin.Key) {
	f.users = append(f.users, &mockUser{
		handle:      handle,
		identityKey: identityKey,
		addressKey:  addressKey,
		p2pTxs:      make(map[string][]*wire.MsgTx),
	})
}

// AddMockUser adds a new mock user.
func (f *MockFactory) AddMockInstrument(handle string, instrumentAlias, instrumentID string) {
	for _, user := range f.users {
		if user.handle != handle {
			continue
		}

		user.instrumentAliases = append(user.instrumentAliases, InstrumentAlias{
			InstrumentAlias: instrumentAlias,
			InstrumentID:    instrumentID,
		})
		return
	}
}

// GenerateMockUser generates a mock user and returns the user's handle, public key, and address.
func (f *MockFactory) GenerateMockUser(host string,
	net bitcoin.Network) (*string, *bitcoin.PublicKey, *bitcoin.RawAddress, error) {

	result := &mockUser{
		handle: uuid.New().String() + "@" + host,
		p2pTxs: make(map[string][]*wire.MsgTx),
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

func (c *MockClient) IsCapable(url string) (bool, error) {
	return true, nil
}

// GetPublicKey gets the identity public key for the handle.
func (c *MockClient) GetPublicKey(ctx context.Context) (*bitcoin.PublicKey, error) {
	pk := c.user.identityKey.PublicKey()
	return &pk, nil
}

// GetPaymentDestination gets a locking script that can be used to send bitcoin.
// If senderKey is not nil then it must be associated with senderHandle and will be used to add
// a signature to the request.
func (c *MockClient) GetPaymentDestination(ctx context.Context, senderName, senderHandle,
	purpose string, amount uint64, senderKey *bitcoin.Key) (bitcoin.Script, error) {

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
//   instrumentID can be empty or "BSV" to request bitcoin.
// If senderKey is not nil then it must be associated with senderHandle and will be used to add
// a signature to the request.
func (c *MockClient) GetPaymentRequest(ctx context.Context, senderName, senderHandle, purpose,
	instrumentID string, amount uint64, senderKey *bitcoin.Key) (*PaymentRequest, error) {

	ra, err := c.user.addressKey.RawAddress()
	if err != nil {
		return nil, errors.Wrap(err, "raw address")
	}

	script, err := ra.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "locking script")
	}

	tx := wire.NewMsgTx(1)

	if instrumentID == "BSV" {
		tx.AddTxOut(wire.NewTxOut(amount, script))
	} else {
		// Note: requires contract address of instrument and possibly access to mock identity oracle
		// client.
		return nil, errors.Wrap(ErrNotCapable, "not implemented in mock client")
	}

	return &PaymentRequest{
		Tx:      tx,
		Outputs: nil,
	}, nil
}

// GetP2PPaymentDestination requests a peer to peer payment destination.
func (c *MockClient) GetP2PPaymentDestination(ctx context.Context,
	value uint64) (*P2PPaymentDestinationOutputs, error) {

	ra, err := c.user.addressKey.RawAddress()
	if err != nil {
		return nil, errors.Wrap(err, "raw address")
	}

	script, err := ra.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "locking script")
	}

	result := &P2PPaymentDestinationOutputs{
		Outputs: []*wire.TxOut{
			&wire.TxOut{
				Value:         value,
				LockingScript: script,
			},
		},
		Reference: uuid.New().String(),
	}

	c.user.p2pTxs[result.Reference] = nil

	return result, nil
}

// PostP2PTransaction posts a P2P transaction to the handle being paid. The same as that used by the
// corresponding GetP2PPaymentDestination.
func (c *MockClient) PostP2PTransaction(ctx context.Context, senderHandle, note, reference string,
	senderKey *bitcoin.Key, tx *wire.MsgTx) (string, error) {

	txs, exists := c.user.p2pTxs[reference]
	if !exists {
		return "", errors.New("Unknown reference")
	}

	c.user.p2pTxs[reference] = append(txs, tx)

	return "Accepted", nil
}

func (c *MockClient) CheckP2PTx(txid bitcoin.Hash32) error {
	for _, txs := range c.user.p2pTxs {
		for _, tx := range txs {
			if tx.TxHash().Equal(&txid) {
				return nil // tx is posted
			}
		}
	}

	return errors.New("Not posted")
}

// ListTokenizedInstruments returns the list of instrument aliases for this paymail handle.
func (c *MockClient) ListTokenizedInstruments(ctx context.Context) ([]InstrumentAlias, error) {
	return c.user.instrumentAliases, nil
}

func (c *MockClient) GetPublicProfile(ctx context.Context) (*PublicProfile, error) {
	return nil, errors.New("Not implemented")
}

func (c *MockClient) PostNegotiationTx(ctx context.Context,
	negotiationTx *NegotiationTransaction) (*NegotiationTransaction, error) {

	if err := negotiationTx.Tx.VerifyInputs(); err != nil {
		return nil, errors.Wrap(err, "verify inputs")
	}

	logger.InfoWithFields(ctx, []logger.Field{
		logger.Stringer("posted_txid", negotiationTx.Tx.Tx.TxHash()),
	}, "Posted negotiation tx")

	return nil, nil
}

func (c *MockClient) PostMerkleProofs(ctx context.Context, merkleProofs MerkleProofs) error {
	return errors.New("Not implemented")
}

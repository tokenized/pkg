package client

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/logger"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
	"github.com/uber-go/zap"
)

var (
	Endian = binary.LittleEndian

	// RemoteClientVersion is the current version of the communication protocol
	RemoteClientVersion = uint8(0)
)

// RemoteClient is a client for interacting with the spynode service.
type RemoteClient struct {
	conn net.Conn

	config        *Config
	nextMessageID uint64

	// Session
	hash             bitcoin.Hash32    // for generating session key
	serverSessionKey bitcoin.PublicKey // for this session
	sessionKey       bitcoin.Key

	handlers []Handler

	// Requests
	sendTxRequests []*sendTxRequest
	getTxRequests  []*getTxRequest

	accepted, ready   bool
	lock, requestLock sync.Mutex
}

type sendTxRequest struct {
	txid     bitcoin.Hash32
	response *Message
	lock     sync.Mutex
}

type getTxRequest struct {
	txid     bitcoin.Hash32
	response *Message
	lock     sync.Mutex
}

// NewClient creates a client from root keys.
func NewRemoteClient(config *Config) (*RemoteClient, error) {
	result := &Client{
		config:        config,
		nextMessageID: 1,
	}

	return result, nil
}

func (c *RemoteClient) RegisterHandler(h Handler) {
	c.lock.Lock()
	c.handlers = append(c.handlers, h)
	c.lock.Unlock()
}

func (c *RemoteClient) IsAccepted(ctx context.Context) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.accepted
}

// SubscribeTx subscribes to information for a specific transaction. Indexes are the indexes of the
// outputs that need to be monitored for spending.
func (c *RemoteClient) SubscribeTx(ctx context.Context, txid bitcoin.Hash32, indexes []uint32) error {
	m := &SubscribeTx{
		TxID:    txid,
		Indexes: indexes,
	}

	logger.Info(ctx, "Sending subscribe tx message")
	return c.sendMessage(ctx, m)
}

// UnsubscribeTx unsubscribes to information for a specific transaction.
func (c *RemoteClient) UnsubscribeTx(ctx context.Context, txid bitcoin.Hash32) error {
	m := &UnsubscribeTx{
		TxID: txid,
	}

	logger.Info(ctx, "Sending unsubscribe tx message")
	return c.sendMessage(ctx, m)
}

// SubscribePushData subscribes to transactions containing the specified push datas.
func (c *RemoteClient) SubscribePushData(ctx context.Context, pushDatas [][]byte) error {
	m := &SubscribePushData{
		PushDatas: pushDatas,
	}

	logger.Info(ctx, "Sending subscribe push data message")
	return c.sendMessage(ctx, m)
}

// UnsubscribePushData unsubscribes to transactions containing the specified push datas.
func (c *RemoteClient) UnsubscribePushData(ctx context.Context, pushDatas [][]byte) error {
	m := &UnsubscribePushData{
		PushDatas: pushDatas,
	}

	logger.Info(ctx, "Sending unsubscribe push data message")
	return c.sendMessage(ctx, m)
}

// SubscribeHeaders subscribes to information on new block headers.
func (c *RemoteClient) SubscribeHeaders(ctx context.Context) error {
	m := &SubscribeHeaders{}

	logger.Info(ctx, "Sending subscribe headers message")
	return c.sendMessage(ctx, m)
}

// UnsubscribeHeaders unsubscribes to information on new block headers.
func (c *RemoteClient) UnsubscribeHeaders(ctx context.Context) error {
	m := &UnsubscribeHeaders{}

	logger.Info(ctx, "Sending unsubscribe headers message")
	return c.sendMessage(ctx, m)
}

// SubscribeContracts subscribes to information on contracts.
func (c *RemoteClient) SubscribeContracts(ctx context.Context) error {
	m := &SubscribeContracts{}

	logger.Info(ctx, "Sending subscribe contracts message")
	return c.sendMessage(ctx, m)
}

// UnsubscribeContracts unsubscribes to information on contracts.
func (c *RemoteClient) UnsubscribeContracts(ctx context.Context) error {
	m := &UnsubscribeContracts{}

	logger.Info(ctx, "Sending unsubscribe contracts message")
	return c.sendMessage(ctx, m)
}

// Ready tells the spynode the client is ready to start receiving updates. Call this after
// connecting and subscribing to all relevant push data.
func (c *RemoteClient) Ready(ctx context.Context) error {
	m := &Ready{}

	logger.Info(ctx, "Sending ready message")
	if err := c.sendMessage(ctx, m); err != nil {
		return err
	}

	c.lock.Lock()
	c.ready = true
	c.lock.Unlock()
	return nil
}

// SendTx sends a tx message to the bitcoin network. It is synchronous meaning it will wait for a
// response before returning.
func (c *RemoteClient) SendTx(ctx context.Context, tx *wire.MsgTx) error {
	// Register with listener for response
	request := &sendTxRequest{
		txid: *tx.TxHash(),
	}

	c.requestLock.Lock()
	c.sendTxRequests = append(c.sendTxRequests, request)
	c.requestLock.Unlock()

	logger.Info(ctx, "Sending send tx message : %s", tx.TxHash())
	m := &SendTx{Tx: tx}
	if err := c.sendMessage(ctx, m); err != nil {
		return err
	}

	// Wait for response
	timeout := time.Now().Add(10 * time.Second)
	for time.Now().Before(timeout) {
		request.lock.Lock()
		if request.response != nil {
			request.lock.Unlock()

			// Remove
			c.requestLock.Lock()
			for i, r := range c.sendTxRequests {
				if r == request {
					c.sendTxRequests = append(c.sendTxRequests[:i], c.sendTxRequests[i+1:]...)
					break
				}
			}
			c.requestLock.Unlock()

			switch msg := request.response.Payload.(type) {
			case *Reject:
				return errors.Wrap(ErrReject, msg.Message)
			case *Accept:
				return nil
			default:
				return fmt.Errorf("Unknown response : %d", request.response.Payload.Type())
			}
		}
		request.lock.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return ErrTimeout
}

// GetTx requests a tx from the bitcoin network. It is synchronous meaning it will wait for a
// response before returning.
func (c *RemoteClient) GetTx(ctx context.Context, txid bitcoin.Hash32) (*Tx, error) {
	// Register with listener for response tx
	request := &getTxRequest{
		txid: txid,
	}

	c.requestLock.Lock()
	c.getTxRequests = append(c.getTxRequests, request)
	c.requestLock.Unlock()

	logger.Info(ctx, "Sending get tx message : %s", txid)
	m := &GetTx{TxID: txid}
	if err := c.sendMessage(ctx, m); err != nil {
		return nil, err
	}

	// Wait for response
	timeout := time.Now().Add(10 * time.Second)
	for time.Now().Before(timeout) {
		request.lock.Lock()
		if request.response != nil {
			request.lock.Unlock()
			// Remove
			c.requestLock.Lock()
			for i, r := range c.getTxRequests {
				if r == request {
					c.getTxRequests = append(c.getTxRequests[:i], c.getTxRequests[i+1:]...)
					break
				}
			}
			c.requestLock.Unlock()

			switch msg := request.response.Payload.(type) {
			case *Reject:
				return nil, errors.Wrap(ErrReject, msg.Message)
			case *Tx:
				return msg, nil
			default:
				return nil, fmt.Errorf("Unknown response : %d", request.response.Payload.Type())
			}
		}
		request.lock.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return nil, ErrTimeout
}

// sendMessage wraps and sends a message to the server.
func (c *RemoteClient) sendMessage(ctx context.Context, payload MessagePayload) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.conn == nil {
		return ErrNotConnected
	}

	message := &Message{
		Payload: payload,
	}

	// TODO Possibly add streaming encryption here. --ce

	if err := message.Serialize(c.conn); err != nil {
		return errors.Wrap(err, "send message")
	}

	return nil
}

func (c *RemoteClient) Connect(ctx context.Context) error {
	if err := c.generateSession(); err != nil {
		return errors.Wrap(err, "session")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.config.ServerAddress)
	if err != nil {
		return errors.Wrap(err, "connect")
	}

	// Send initial message
	register := &Register{
		Version:          Version,
		Key:              c.config.ClientKey.PublicKey(),
		Hash:             c.hash,
		StartBlockHeight: c.config.StartBlockHeight,
		NextMessageID:    c.nextMessageID,
	}

	sigHash, err := register.SigHash()
	if err != nil {
		return errors.Wrap(err, "sig hash")
	}

	register.Signature, err = c.config.ClientKey.Sign(sigHash.Bytes())
	if err != nil {
		return errors.Wrap(err, "sign")
	}

	message := Message{Payload: register}
	if err := message.Serialize(conn); err != nil {
		return errors.Wrap(err, "send register")
	}

	logger.Info(ctx, "Sent register message")

	c.lock.Lock()
	c.conn = conn
	c.lock.Unlock()
	return nil
}

// Listen listens for incoming messages.
func (c *RemoteClient) Listen(ctx context.Context) error {
	for {
		c.lock.Lock()
		conn := c.conn
		c.lock.Unlock()

		if conn == nil {
			logger.Info(ctx, "Connection closed")
			return nil // connection closed
		}

		m := &Message{}
		if err := m.Deserialize(conn); err != nil {
			if errors.Cause(err) == io.EOF {
				logger.Warn(ctx, "Server disconnected")
				return nil
			}

			logger.Warn(ctx, "Failed to read incoming message : %s", err)
			c.lock.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.lock.Unlock()
			return nil
		}

		// Handle message
		switch msg := m.Payload.(type) {
		case *AcceptRegister:
			if !msg.Key.Equal(c.serverSessionKey) {
				logger.Error(ctx, "Wrong server session key returned : got %s, want %s", msg.Key,
					c.serverSessionKey)
				c.Close(ctx)
				return ErrWrongKey
			}

			sigHash, err := msg.SigHash(c.hash)
			if err != nil {
				logger.Error(ctx, "Failed to create accept sig hash : %s", err)
				c.Close(ctx)
				return errors.Wrap(err, "accept sig hash")
			}

			if !msg.Signature.Verify(sigHash.Bytes(), msg.Key) {
				logger.Error(ctx, "Invalid server signature")
				c.Close(ctx)
				return ErrBadSignature
			}

			c.lock.Lock()
			c.accepted = true
			c.lock.Unlock()
			logger.Info(ctx, "Server accepted connection : %+v", msg)

		case *Tx:
			txid := *msg.Tx.TxHash()
			logger.InfoWithZapFields(ctx, []zap.Field{
				zap.Stringer("txid", txid),
				zap.Uint64("message_id", msg.ID),
			}, "Received tx %d", msg.ID)
			// logger.Info(ctx, "Received tx message %d : %s", msg.ID, msg.Tx.TxHash())

			if c.nextMessageID != msg.ID {
				logger.WarnWithZapFields(ctx, []zap.Field{
					zap.Uint64("expected_message_id", c.nextMessageID),
					zap.Uint64("message_id", msg.ID),
				}, "Wrong message ID")
				continue
			}

			if msg.ID == 0 { // non-sequential message (from a request)
				c.requestLock.Lock()
				for _, request := range c.sendTxRequests {
					if request.txid.Equal(&txid) {
						request.response = m
						break
					}
				}
				c.requestLock.Unlock()
			} else {
				c.nextMessageID = msg.ID + 1

				ctx := logger.ContextWithLogTrace(ctx, txid.String())

				c.lock.Lock()
				for _, handler := range c.handlers {
					handler.HandleTx(ctx, msg)
				}
				c.lock.Unlock()
			}

		case *TxUpdate:
			logger.InfoWithZapFields(ctx, []zap.Field{
				zap.Stringer("txid", msg.TxID),
				zap.Uint64("message_id", msg.ID),
			}, "Received tx state")

			if c.nextMessageID != msg.ID {
				logger.WarnWithZapFields(ctx, []zap.Field{
					zap.Uint64("expected_message_id", c.nextMessageID),
					zap.Uint64("message_id", msg.ID),
				}, "Wrong message ID")
				continue
			}

			c.nextMessageID = msg.ID + 1

			ctx := logger.ContextWithLogTrace(ctx, msg.TxID.String())

			c.lock.Lock()
			for _, handler := range c.handlers {
				handler.HandleTxUpdate(ctx, msg)
			}
			c.lock.Unlock()

		case *Headers:
			logger.InfoWithZapFields(ctx, []zap.Field{
				zap.Int("header_count", len(msg.Headers)),
				zap.Uint32("start_height", msg.StartHeight),
			}, "Received headers")

			c.lock.Lock()
			for _, handler := range c.handlers {
				handler.HandleHeaders(ctx, msg)
			}
			c.lock.Unlock()

		case *ChainTip:
			logger.InfoWithZapFields(ctx, []zap.Field{
				zap.Stringer("hash", msg.Hash),
				zap.Uint32("height", msg.Height),
			}, "Received chain tip")

		case *Accept:
			// MessageType uint8           // type of the message being rejected
			// Hash        *bitcoin.Hash32 // optional identifier for the rejected item (tx)

			if msg.Hash != nil && msg.MessageType == MessageTypeSendTx {
				found := false

				c.requestLock.Lock()
				for _, request := range c.sendTxRequests {
					request.lock.Lock()
					if request.txid.Equal(msg.Hash) {
						request.response = m
						request.lock.Unlock()
						found = true
						break
					}
					request.lock.Unlock()
				}
				c.requestLock.Unlock()

				if found {
					continue
				}
			}

		case *Reject:
			// MessageType uint8           // type of the message being rejected
			// Hash        *bitcoin.Hash32 // optional identifier for the rejected item (tx)
			// Code        uint32          // code representing the reason for the reject
			// Message     string

			if msg.Hash != nil {
				found := false

				if msg.MessageType == MessageTypeSendTx {
					c.requestLock.Lock()
					for _, request := range c.sendTxRequests {
						request.lock.Lock()
						if request.txid.Equal(msg.Hash) {
							request.response = m
							request.lock.Unlock()
							found = true
							break
						}
						request.lock.Unlock()
					}
					c.requestLock.Unlock()

					if found {
						continue
					}
				}

				if msg.MessageType == MessageTypeGetTx {
					c.requestLock.Lock()
					for _, request := range c.getTxRequests {
						request.lock.Lock()
						if request.txid.Equal(msg.Hash) {
							request.response = m
							request.lock.Unlock()
							found = true
							break
						}
						request.lock.Unlock()
					}
					c.requestLock.Unlock()

					if found {
						continue
					}
				}
			}

		default:
			logger.Error(ctx, "Unknown message type")

		}
	}
}

func (c *RemoteClient) Close(ctx context.Context) {
	c.lock.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.lock.Unlock()
}

// generateSession generates session keys from root keys.
func (c *RemoteClient) generateSession() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	for { // loop through any out of range keys
		var err error

		// Generate random hash
		c.hash, err = bitcoin.GenerateSeedValue()
		if err != nil {
			return errors.Wrap(err, "generate hash")
		}

		// Derive session keys
		c.serverSessionKey, err = bitcoin.NextPublicKey(c.config.ServerKey, c.hash)
		if err != nil {
			if errors.Cause(err) == bitcoin.ErrOutOfRangeKey {
				continue // try with a new hash
			}
			return errors.Wrap(err, "next public key")
		}

		c.sessionKey, err = bitcoin.NextKey(c.config.ClientKey, c.hash)
		if err != nil {
			if errors.Cause(err) == bitcoin.ErrOutOfRangeKey {
				continue // try with a new hash
			}
			return errors.Wrap(err, "next key")
		}

		return nil
	}
}

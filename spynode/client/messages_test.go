package client

import (
	"bytes"
	"crypto/rand"
	"reflect"
	"testing"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

func TestSerializeMessages(t *testing.T) {
	k, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate key : %s", err)
	}

	pk := k.PublicKey()

	sigHash := make([]byte, 32)
	rand.Read(sigHash)

	sig, err := k.Sign(sigHash)
	if err != nil {
		t.Fatalf("Failed to sign : %s", err)
	}

	var hash bitcoin.Hash32
	rand.Read(hash[:])

	tx := wire.NewMsgTx(1)

	unlockingScript := make([]byte, 134)
	rand.Read(unlockingScript)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&hash, 0), unlockingScript))

	lockingScript := make([]byte, 34)
	rand.Read(lockingScript)
	txout := wire.NewTxOut(1039, lockingScript)

	tm := time.Unix(time.Now().Unix(), 0) // Must create via Unix time for reflect.DeepEqual

	var messages = []struct {
		name string
		t    uint64
		m    MessagePayload
	}{
		{
			name: "Register",
			t:    MessageTypeRegister,
			m: &Register{
				Version:          1,
				Key:              pk,
				Hash:             hash,
				StartBlockHeight: 12345,
				NextMessageID:    2,
				ChainTip:         hash,
				Signature:        sig,
			},
		},
		{
			name: "SubscribeTx",
			t:    MessageTypeSubscribeTx,
			m: &SubscribeTx{
				TxID:    hash,
				Indexes: []uint32{0, 94847},
			},
		},
		{
			name: "UnsubscribeTx",
			t:    MessageTypeUnsubscribeTx,
			m: &UnsubscribeTx{
				TxID: hash,
			},
		},
		{
			name: "SubscribePushData",
			t:    MessageTypeSubscribePushData,
			m: &SubscribePushData{
				PushDatas: [][]byte{
					sigHash,
					hash.Bytes(),
				},
			},
		},
		{
			name: "UnsubscribePushData",
			t:    MessageTypeUnsubscribePushData,
			m: &UnsubscribePushData{
				PushDatas: [][]byte{
					sigHash,
					hash.Bytes(),
				},
			},
		},
		{
			name: "SubscribeHeaders",
			t:    MessageTypeSubscribeHeaders,
			m:    &SubscribeHeaders{},
		},
		{
			name: "UnsubscribeHeaders",
			t:    MessageTypeUnsubscribeHeaders,
			m:    &UnsubscribeHeaders{},
		},
		{
			name: "SubscribeContracts",
			t:    MessageTypeSubscribeContracts,
			m:    &SubscribeContracts{},
		},
		{
			name: "UnsubscribeContracts",
			t:    MessageTypeUnsubscribeContracts,
			m:    &UnsubscribeContracts{},
		},
		{
			name: "Ready",
			t:    MessageTypeReady,
			m:    &Ready{},
		},
		{
			name: "GetChainTip",
			t:    MessageTypeGetChainTip,
			m:    &GetChainTip{},
		},
		{
			name: "GetHeaders",
			t:    MessageTypeGetHeaders,
			m: &GetHeaders{
				StartHeight: -1,
				MaxCount:    1000,
			},
		},
		{
			name: "SendTx",
			t:    MessageTypeSendTx,
			m: &SendTx{
				Tx: tx,
			},
		},
		{
			name: "GetTx",
			t:    MessageTypeGetTx,
			m: &GetTx{
				TxID: hash,
			},
		},
		{
			name: "AcceptRegister",
			t:    MessageTypeAcceptRegister,
			m: &AcceptRegister{
				Key:           pk,
				PushDataCount: 3,
				UTXOCount:     40,
				MessageCount:  1050,
				Signature:     sig,
			},
		},
		{
			name: "Tx",
			t:    MessageTypeTx,
			m: &Tx{
				ID:      3938472,
				Tx:      tx,
				Outputs: []*wire.TxOut{txout},
				State: TxState{
					Safe:             true,
					UnSafe:           true,
					Cancelled:        true,
					UnconfirmedDepth: 1,
				},
			},
		},
		{
			name: "TxUpdate",
			t:    MessageTypeTxUpdate,
			m: &TxUpdate{
				ID:   3938472,
				TxID: hash,
				State: TxState{
					Safe:             true,
					UnSafe:           true,
					Cancelled:        true,
					UnconfirmedDepth: 1,
					MerkleProof: &MerkleProof{
						Index: 1,
						Path:  []bitcoin.Hash32{hash, hash},
						BlockHeader: wire.BlockHeader{
							Timestamp: tm,
						},
						DuplicatedIndexes: []uint64{},
					},
				},
			},
		},
		{
			name: "Headers",
			t:    MessageTypeHeaders,
			m: &Headers{
				StartHeight: 0,
				Headers: []*wire.BlockHeader{
					&wire.BlockHeader{
						Timestamp: tm,
					},
					&wire.BlockHeader{
						Timestamp: tm,
					},
				},
			},
		},
		{
			name: "ChainTip",
			t:    MessageTypeChainTip,
			m: &ChainTip{
				Height: 1000,
				Hash:   hash,
			},
		},
		{
			name: "Accept",
			t:    MessageTypeAccept,
			m: &Accept{
				MessageType: MessageTypeRegister,
				Hash:        &hash,
			},
		},
		{
			name: "Accept (No hash)",
			t:    MessageTypeAccept,
			m: &Accept{
				MessageType: MessageTypeRegister,
				Hash:        nil,
			},
		},
		{
			name: "Reject",
			t:    MessageTypeReject,
			m: &Reject{
				MessageType: MessageTypeRegister,
				Hash:        &hash,
				Code:        1,
				Message:     "test reject",
			},
		},
		{
			name: "Reject (No hash)",
			t:    MessageTypeReject,
			m: &Reject{
				MessageType: MessageTypeRegister,
				Hash:        nil,
				Code:        1,
				Message:     "test reject",
			},
		},
	}

	for _, tt := range messages {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tt.m.Serialize(&buf); err != nil {
				t.Fatalf("Failed to serialize : %s", err)
			}

			if tt.m.Type() != tt.t {
				t.Fatalf("Wrong type : got %d, want %d", tt.m.Type(), tt.t)
			}

			read := PayloadForType(tt.t)

			if read == nil {
				t.Fatalf("No payload structure for type : %d", tt.t)
			}

			if err := read.Deserialize(&buf); err != nil {
				t.Fatalf("Failed to deserialize : %s", err)
			}

			if !reflect.DeepEqual(tt.m, read) {
				t.Fatalf("Deserialize not equal : \n  got  %+v\n  want %+v", read, tt.m)
			}
		})
	}
}

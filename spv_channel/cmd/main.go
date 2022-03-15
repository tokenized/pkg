package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json_envelope"
	"github.com/tokenized/pkg/logger"
	"github.com/tokenized/pkg/merchant_api"
	"github.com/tokenized/pkg/merkle_proof"
	"github.com/tokenized/pkg/spv_channel"
)

func main() {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")

	if len(os.Args) < 2 {
		logger.Fatal(ctx, "Not enough arguments. Need command (create_account, create_channel, listen)")
	}

	switch os.Args[1] {
	case "create_account":
		CreateAccount(ctx, os.Args[2:])

	case "create_channel":
		CreateChannel(ctx, os.Args[2:])

	case "listen": // listen on channel
		Listen(ctx, os.Args[2:])
	}
}

func CreateAccount(ctx context.Context, args []string) {
	if len(args) != 2 {
		logger.Fatal(ctx, "Wrong argument count: create_account [URL] [Token]")
	}

	url := args[0]
	token := args[1]

	accountID, accessToken, err := spv_channel.CreateAccount(ctx, url, token)
	if err != nil {
		fmt.Printf("Failed to create account : %s\n", err)
		return
	}

	fmt.Printf("Created Account : %s\n", accountID)
	fmt.Printf("Access Token : %s\n", accessToken)
}

func CreateChannel(ctx context.Context, args []string) {
	if len(args) != 3 {
		logger.Fatal(ctx, "Wrong argument count: create_channel [URL] [Account] [Token]")
	}

	url := args[0]
	accountID := args[1]
	token := args[2]

	channel, err := spv_channel.CreateChannel(ctx, url, accountID, token)
	if err != nil {
		fmt.Printf("Failed to create channel : %s\n", err)
		return
	}

	js, _ := json.MarshalIndent(*channel, "", "  ")
	fmt.Printf("Created Channel : %s\n", js)
}

func Listen(ctx context.Context, args []string) {
	if len(args) != 3 {
		logger.Fatal(ctx, "Wrong argument count: listen [URL] [Channel] [Token]")
	}

	url := args[0]
	channelID := args[1]
	token := args[2]

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
		logger.String("channel", channelID),
	}, "Starting listening to SPV channel")

	listenInterrupt := make(chan interface{})
	listenComplete := make(chan interface{})
	incoming := make(chan spv_channel.SPVMessage, 5)

	go func() {
		if err := spv_channel.Listen(ctx, url, channelID, token, incoming,
			listenInterrupt); err != nil {
			logger.Error(ctx, "Failed to listen : %s", err)
		}

		close(incoming)
		close(listenComplete)
	}()

	go func() {
		for msg := range incoming {
			fmt.Printf("Sequence : %d\n", msg.Sequence)
			fmt.Printf("Content Type : %s\n", msg.ContentType)
			fmt.Printf("Received : %s\n", msg.Received)
			buf := &bytes.Buffer{}
			json.Indent(buf, []byte(msg.Payload), "", "  ")
			fmt.Printf("Payload: %s\n", string(buf.Bytes()))

			processMessage(ctx, msg)
		}
	}()

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	select {
	case <-listenComplete:
		fmt.Printf("Complete (without interrupt)\n")

	case <-osSignals:
		close(listenInterrupt)

		select {
		case <-listenComplete:
			fmt.Printf("Complete (after interrupt)\n")
		case <-time.After(3 * time.Second):
			fmt.Printf("Shut down timed out\n")
		}
	}
}

func processMessage(ctx context.Context, msg spv_channel.SPVMessage) {
	envelope := &json_envelope.JSONEnvelope{}
	if err := json.Unmarshal([]byte(msg.Payload), envelope); err != nil {
		fmt.Printf("Failed to unmarshal envelope : %s\n", err)
		return
	}

	if err := envelope.Verify(); err != nil {
		fmt.Printf("JSON Envelope didn't verify : %s", err)
	}
	fmt.Printf("JSON Envelope verified!\n")

	callback := &merchant_api.SubmitTxCallbackResponse{}
	if err := json.Unmarshal([]byte(envelope.Payload), callback); err != nil {
		fmt.Printf("Failed to unmarshal callback : %s\n", err)
		return
	}

	switch callback.Reason {
	case merchant_api.CallBackReasonMerkleProof:
		merkleProof := &merkle_proof.MerkleProof{}
		if err := json.Unmarshal(callback.Payload, merkleProof); err != nil {
			fmt.Printf("Failed to unmarshal merkle proof : %s\n", err)
			return
		}

		js, _ := json.MarshalIndent(merkleProof, "", "  ")
		fmt.Printf("Merkle Proof : %s\n", js)

		if err := merkleProof.Verify(); err != nil {
			fmt.Printf("Merkle proof did not verify : %s\n", err)
			return
		}

		fmt.Printf("Merkle proof is verified!\n")
		return

	case merchant_api.CallBackReasonDoubleSpend, merchant_api.CallBackReasonDoubleSpendAttempt:
		buf := &bytes.Buffer{}
		json.Indent(buf, callback.Payload, "", "  ")
		fmt.Printf("Callback Payload: %s\n", string(buf.Bytes()))

		doubleSpend := &merchant_api.CallBackDoubleSpend{}
		if err := json.Unmarshal(callback.Payload, doubleSpend); err != nil {
			fmt.Printf("Failed to unmarshal double spend : %s\n", err)
			return
		}

		title := "Double spend"
		if callback.Reason == merchant_api.CallBackReasonDoubleSpendAttempt {
			title += " attempt"
		}

		if doubleSpend.Tx != nil {
			fmt.Printf("%s by : %s\n", title, doubleSpend.Tx.StringWithAddresses(bitcoin.MainNet))
		} else {
			fmt.Printf("%s by %s\n", title, callback.TxID)
		}

	default:
		fmt.Printf("Unkown callback reason : %s\n", callback.Reason)
		return
	}
}

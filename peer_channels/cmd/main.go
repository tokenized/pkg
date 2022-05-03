package main

import (
	"bytes"
	"context"
	"encoding/hex"
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
	"github.com/tokenized/pkg/peer_channels"
)

func main() {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")

	if len(os.Args) < 2 {
		logger.Fatal(ctx, "Not enough arguments. Need command (create_account, create_channel,"+
			" listen, channel_listen, post, post_binary)")
	}

	switch os.Args[1] {
	case "create_account":
		CreateAccount(ctx, os.Args[2:])

	case "create_channel":
		CreateChannel(ctx, os.Args[2:])

	case "listen":
		Listen(ctx, os.Args[2:])

	case "channel_listen":
		ChannelListen(ctx, os.Args[2:])

	case "post":
		Post(ctx, os.Args[2:])

	case "post_binary":
		PostBinary(ctx, os.Args[2:])
	}
}

func CreateAccount(ctx context.Context, args []string) {
	if len(args) != 2 {
		logger.Fatal(ctx, "Wrong argument count: create_account [URL] [Token]")
	}

	url := args[0]
	token := args[1]

	accountID, accessToken, err := peer_channels.CreateAccount(ctx, url, token)
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

	channel, err := peer_channels.CreateChannel(ctx, url, accountID, token)
	if err != nil {
		fmt.Printf("Failed to create channel : %s\n", err)
		return
	}

	js, _ := json.MarshalIndent(*channel, "", "  ")
	fmt.Printf("Created Channel : %s\n", js)
}

func Post(ctx context.Context, args []string) {
	if len(args) != 4 {
		logger.Fatal(ctx, "Wrong argument count: listen [URL] [Channel] [Token] [Message]")
	}

	url := args[0]
	channelID := args[1]
	token := args[2]
	message := args[3]

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
		logger.String("channel", channelID),
	}, "Posting message to peer channel")

	buf := &bytes.Buffer{}
	var msg *peer_channels.Message
	if err := json.Indent(buf, []byte(message), "", "  "); err == nil {
		msg, err = peer_channels.PostJSONMessage(ctx, url, channelID, token, message)
		if err != nil {
			logger.Fatal(ctx, "Failed to post message : %s", err)
		}
	} else {
		msg, err = peer_channels.PostTextMessage(ctx, url, channelID, token, message)
		if err != nil {
			logger.Fatal(ctx, "Failed to post message : %s", err)
		}
	}

	js, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Printf("Posted message : %s\n", js)
}

func PostBinary(ctx context.Context, args []string) {
	if len(args) != 4 {
		logger.Fatal(ctx, "Wrong argument count: listen [URL] [Channel] [Token] [Message]")
	}

	url := args[0]
	channelID := args[1]
	token := args[2]
	message, err := hex.DecodeString(args[3])
	if err != nil {
		fmt.Printf("Failed to decode hex payload : %s", err)
	}

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
		logger.String("channel", channelID),
	}, "Posting message to peer channel")

	msg, err := peer_channels.PostBinaryMessage(ctx, url, channelID, token, message)
	if err != nil {
		logger.Fatal(ctx, "Failed to post message : %s", err)
	}

	js, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Printf("Posted message : %s\n", js)
}

func Listen(ctx context.Context, args []string) {
	if len(args) != 3 {
		logger.Fatal(ctx, "Wrong argument count: listen [URL] [Account] [Token]")
	}

	url := args[0]
	accountID := args[1]
	token := args[2]

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
		logger.String("account", accountID),
	}, "Starting listening to peer channel account")

	listenInterrupt := make(chan interface{})
	listenComplete := make(chan interface{})
	incoming := make(chan peer_channels.Message, 5)

	go func() {
		if err := peer_channels.AccountListen(ctx, url, accountID, token, incoming,
			listenInterrupt); err != nil {
			logger.Error(ctx, "Failed to listen : %s", err)
		}

		close(incoming)
		close(listenComplete)
	}()

	go func() {
		for msg := range incoming {
			js, _ := json.MarshalIndent(msg, "", "  ")
			fmt.Printf("Received message : %s\n", js)

			// processMessage(ctx, msg)

			if err := peer_channels.MarkMessages(ctx, url, msg.ChannelID.String(), token,
				msg.Sequence, true, true); err != nil {
				fmt.Printf("Failed to mark message as read : %s", err)
			}
			fmt.Printf("Marked sequence %d as read\n", msg.Sequence)
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

func ChannelListen(ctx context.Context, args []string) {
	if len(args) != 3 {
		logger.Fatal(ctx, "Wrong argument count: listen [URL] [Channel] [Token]")
	}

	url := args[0]
	channelID := args[1]
	token := args[2]

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
		logger.String("channel", channelID),
	}, "Starting listening to peer channel")

	listenInterrupt := make(chan interface{})
	listenComplete := make(chan interface{})
	incoming := make(chan peer_channels.Message, 5)

	go func() {
		if err := peer_channels.ChannelListen(ctx, url, channelID, token, incoming,
			listenInterrupt); err != nil {
			logger.Error(ctx, "Failed to listen : %s", err)
		}

		close(incoming)
		close(listenComplete)
	}()

	go func() {
		for msg := range incoming {
			js, _ := json.MarshalIndent(msg, "", "  ")
			fmt.Printf("Received message : %s\n", js)

			// processMessage(ctx, msg)

			if err := peer_channels.MarkMessages(ctx, url, channelID, token, msg.Sequence, true,
				true); err != nil {
				fmt.Printf("Failed to mark message as read : %s", err)
			}
			fmt.Printf("Marked sequence %d as read\n", msg.Sequence)
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

func processMerchantAPIMessage(ctx context.Context, msg peer_channels.Message) {
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

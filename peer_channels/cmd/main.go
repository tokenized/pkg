package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/tokenized/logger"
	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json_envelope"
	"github.com/tokenized/pkg/merchant_api"
	"github.com/tokenized/pkg/merkle_proof"
	"github.com/tokenized/pkg/peer_channels"
	"github.com/tokenized/threads"

	"github.com/pkg/errors"
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

	account, err := peer_channels.HTTPCreateAccount(ctx, url, token)
	if err != nil {
		fmt.Printf("Failed to create account : %s\n", err)
		return
	}

	fmt.Printf("Created Account : %s\n", account.AccountID)
	fmt.Printf("Access Token : %s\n", account.Token)
	fmt.Printf("Account URL : %s\n", account)
}

func CreateChannel(ctx context.Context, args []string) {
	if len(args) != 3 {
		logger.Fatal(ctx, "Wrong argument count: create_channel [URL] [Account] [Token]")
	}

	url := args[0]
	accountID := args[1]
	token := args[2]

	account, err := peer_channels.NewAccount(url, accountID, token)
	if err != nil {
		fmt.Printf("Failed to create peer channels account : %s", err)
		return
	}

	factory := peer_channels.NewFactory()
	accountClient, err := factory.NewAccountClient(*account)
	if err != nil {
		fmt.Printf("Failed to create peer channels client : %s", err)
		return
	}

	channel, err := accountClient.CreateChannel(ctx)
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

	factory := peer_channels.NewFactory()
	client, err := factory.NewClient(url)
	if err != nil {
		fmt.Printf("Failed to create peer channels client : %s", err)
		return
	}
	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
		logger.String("channel", channelID),
	}, "Posting message to peer channel")

	buf := &bytes.Buffer{}
	var contentType string
	if err := json.Indent(buf, []byte(message), "", "  "); err == nil {
		contentType = peer_channels.ContentTypeJSON
	} else {
		contentType = peer_channels.ContentTypeText
	}

	if err := client.WriteMessage(ctx, channelID, token, contentType,
		bytes.NewReader([]byte(message))); err != nil {
		logger.Fatal(ctx, "Failed to post message : %s", err)
	}

	fmt.Printf("Posted message\n")
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

	factory := peer_channels.NewFactory()
	client, err := factory.NewClient(url)
	if err != nil {
		fmt.Printf("Failed to create peer channels client : %s", err)
		return
	}
	if err := client.WriteMessage(ctx, channelID, token, peer_channels.ContentTypeBinary,
		bytes.NewReader(message)); err != nil {
		logger.Fatal(ctx, "Failed to post message : %s", err)
	}

	fmt.Printf("Posted message\n")
}

func Listen(ctx context.Context, args []string) {
	if len(args) != 2 {
		logger.Fatal(ctx, "Wrong argument count: listen [URL] [Token]")
	}

	url := args[0]
	token := args[1]

	logger.InfoWithFields(ctx, []logger.Field{
		logger.String("url", url),
	}, "Starting listening")

	incoming := make(chan peer_channels.Message, 5)

	factory := peer_channels.NewFactory()
	client, err := factory.NewClient(url)
	if err != nil {
		fmt.Printf("Failed to create peer channels client : %s", err)
		return
	}

	var wait sync.WaitGroup

	listenThread, listenComplete := threads.NewInterruptableThreadComplete("Listen",
		func(ctx context.Context, interrupt <-chan interface{}) error {
			return client.Listen(ctx, token, true, incoming, interrupt)
		}, &wait)

	handleThread, handleComplete := threads.NewUninterruptableThreadComplete("Handle",
		func(ctx context.Context) error {
			for msg := range incoming {
				js, _ := json.MarshalIndent(msg, "", "  ")
				fmt.Printf("Received message : %s\n", js)

				// processMessage(ctx, msg)

				if err := client.MarkMessages(ctx, msg.ChannelID, token, msg.Sequence, true,
					true); err != nil {
					return errors.Wrap(err, "mark message")
				}
				fmt.Printf("Marked sequence %d as read\n", msg.Sequence)
			}

			return nil
		}, &wait)

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	listenThread.Start(ctx)
	handleThread.Start(ctx)

	select {
	case <-listenComplete:
		fmt.Printf("Listen complete (without interrupt)\n")

	case <-handleComplete:
		fmt.Printf("Handle complete (without interrupt)\n")

	case <-osSignals:
	}

	listenThread.Stop(ctx)
	close(incoming)

	wait.Wait()
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

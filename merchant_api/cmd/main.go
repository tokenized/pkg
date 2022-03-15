package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tokenized/config"
	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/logger"
	"github.com/tokenized/pkg/merchant_api"
	"github.com/tokenized/pkg/wire"
)

type Config struct {
	URL           string `envconfig:"URL" json:"url"`
	CallBackURL   string `envconfig:"CALLBACK_URL" json:"callback_url"`
	CallBackToken string `envconfig:"CALLBACK_TOKEN" json:"callback_token"`
}

func main() {
	ctx := logger.ContextWithLogger(context.Background(), true, true, "")

	cfg := &Config{}
	if err := config.LoadConfig(ctx, cfg); err != nil {
		logger.Fatal(ctx, "Failed to load config : %s", err)
	}

	if len(os.Args) < 2 {
		logger.Fatal(ctx, "Not enough arguments. Need command (send_tx, tx_status)")
	}

	switch os.Args[1] {
	case "send_tx":
		SendTx(ctx, cfg, os.Args[2:])

	case "tx_status":
		TxStatus(ctx, cfg, os.Args[2:])
	}
}

func TxStatus(ctx context.Context, cfg *Config, args []string) {
	if len(args) != 1 {
		logger.Fatal(ctx, "Wrong argument count: tx_status [TxID]")
	}

	txid, err := bitcoin.NewHash32FromStr(args[0])
	if err != nil {
		fmt.Printf("Invalid txid : %s\n", err)
		return
	}

	status, err := merchant_api.GetTxStatus(ctx, cfg.URL, *txid)
	if err != nil {
		fmt.Printf("Failed to get tx status : %s\n", err)
		return
	}

	js, _ := json.MarshalIndent(status, "", "  ")
	fmt.Printf("Tx Status : %s\n", js)

	if err := status.Success(); err != nil {
		fmt.Printf("Invalid tx status : %s\n", err)
		return
	}

	fmt.Printf("Tx is valid\n")
}

func SendTx(ctx context.Context, cfg *Config, args []string) {
	if len(args) != 1 {
		logger.Fatal(ctx, "Wrong argument count: send_tx [Hex]")
	}

	h := args[0]
	b, err := hex.DecodeString(h)
	if err != nil {
		fmt.Printf("Failed to decode tx hex : %s\n", err)
		return
	}

	tx := &wire.MsgTx{}
	if err := tx.Deserialize(bytes.NewReader(b)); err != nil {
		fmt.Printf("Failed to decode tx : %s\n", err)
		return
	}

	mpFormat := merchant_api.CallBackMerkleProofFormat

	request := merchant_api.SubmitTxRequest{
		Tx:                tx,
		CallBackURL:       &cfg.CallBackURL,
		CallBackToken:     &cfg.CallBackToken,
		SendMerkleProof:   true,
		MerkleProofFormat: &mpFormat,
		DoubleSpendCheck:  true,
	}

	js, _ := json.MarshalIndent(request, "", "  ")
	fmt.Printf("Submit Tx Request : %s\n", js)

	response, err := merchant_api.SubmitTx(ctx, cfg.URL, request)
	if err != nil {
		fmt.Printf("Failed to create account : %s\n", err)
		return
	}

	js, _ = json.MarshalIndent(response, "", "  ")
	fmt.Printf("Submit Tx Response : %s\n", js)

	if err := response.Success(); err != nil {
		fmt.Printf("Tx submission failed : %s\n", err)
	} else {
		fmt.Printf("Tx submitted successfully!\n")
	}
}

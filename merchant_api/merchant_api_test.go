package merchant_api

import (
	"context"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json"
)

func TestGetFeeQuote(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		url  string
		auth string
	}{
		// {
		// 	name: "Taal",
		// 	url:  "https://merchantapi.taal.com/",
		// 	auth: "",
		// },
		// {
		// 	name: "GorillaPool",
		// 	url:  "https://mapi.gorillapool.io/",
		// 	auth: "",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fees, err := GetFeeQuoteWithAuth(ctx, tt.url, tt.auth)
			if err != nil {
				t.Fatalf("Failed to get fees : %s", err)
			}

			js, err := json.MarshalIndent(fees, "", "  ")
			if err != nil {
				t.Errorf("Failed to marshal : %s", err)
			}

			t.Logf("Deserialized : %s", string(js))
		})
	}
}

func TestGetTxStatus(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		url  string
		txid string
	}{
		// {
		// 	name: "Taal",
		// 	url:  "https://merchantapi.taal.com/",
		// 	txid: "413b111f0cb63f0a95fabcc9eb1b439aeb7957c5d2b3189c1fa8f36f5cb26e0d",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txid, err := bitcoin.NewHash32FromStr(tt.txid)
			if err != nil {
				t.Fatalf("Failed to parse txid : %s", err)
			}

			status, err := GetTxStatus(ctx, tt.url, *txid)
			if err != nil {
				t.Fatalf("Failed to get fees : %s", err)
			}

			js, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				t.Errorf("Failed to marshal : %s", err)
			}

			t.Logf("Deserialized : %s", string(js))

			if status.BlockHash == nil {
				t.Fatalf("Missing block hash")
			}

			if status.BlockHash.String() !=
				"00000000000000000aa8fd4465c7d2636857413652dbe372851d6ce461b828a7" {
				t.Errorf("Wrong block hash : got %s, want %s", status.BlockHash,
					"00000000000000000aa8fd4465c7d2636857413652dbe372851d6ce461b828a7")
			}

			if status.BlockHeight == nil {
				t.Fatalf("Missing block height")
			}

			if *status.BlockHeight != 692815 {
				t.Errorf("Wrong block height : got %d, want %d", *status.BlockHeight, 692815)
			}
		})
	}
}

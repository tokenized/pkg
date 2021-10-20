package bsvalias

import (
	"context"
	"strings"
	"testing"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

// Keep commented so we don't hit moneybutton.com on every test run.
var (
	handles = []string{
		// "test@localhost:8080",
		// "loosethinker@moneybutton.com",
	}
)

func TestP2PTransactionSignature(t *testing.T) {
	keyText := "037d391ec99f5fbc48894986391d3d2388045bcf85409ce2e2a92a683dc7a76581"
	signatureText := "H84lMGpH1L2iFf8BzPuxzevTjyij8i2lej1OBmBVBqF1ZwnLn5+R3VdgLFUS+fUQgMkZmPCb85xlw1R6PtSe0DY="
	txidText := "43b83509a310acbbcdb91164285829505ae415ad476e773f1e9ce49023387ac8"

	key, err := bitcoin.PublicKeyFromStr(keyText)
	if err != nil {
		t.Fatalf("Failed to parse key : %s", err)
	}

	signature, err := bitcoin.SignatureFromCompact(signatureText)
	if err != nil {
		t.Fatalf("Failed to parse signature : %s", err)
	}

	txid, err := bitcoin.NewHash32FromStr(txidText)
	if err != nil {
		t.Fatalf("Failed to parse txid : %s", err)
	}

	sigHash, err := SignatureHashForMessage(txid.String())
	if err != nil {
		t.Fatalf("Failed to create txid message sig hash : %s", err)
	}

	if !signature.Verify(sigHash, key) {
		t.Errorf("Not message hash")
	} else {
		t.Logf("Signature is valid")
	}
}

func TestCapabilities(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		fields := strings.Split(handle, "@")
		site, err := GetSite(ctx, fields[1])
		if err != nil {
			t.Fatalf("Failed to get site : %s", err)
		}

		t.Logf("%s Site : %+v", fields[1], site)
	}
}

func TestIdentity(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		fields := strings.Split(handle, "@")

		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		if id.Alias != fields[0] {
			t.Fatalf("Failed to parse alias : %s != %s", id.Alias, fields[0])
		}

		if id.Hostname != fields[1] {
			t.Fatalf("Failed to parse domain : %s != %s", id.Hostname, fields[1])
		}

		t.Logf("ID : %+v", id)
	}
}

func TestPublicKey(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		publicKey, err := id.GetPublicKey(ctx)
		if err != nil {
			t.Fatalf("Failed to get public key : %s", err)
		}

		t.Logf("Public Key : %s", publicKey.String())
	}
}

func TestPaymentDestination(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		script, err := id.GetPaymentDestination(ctx, "John Bitcoin", "john@bitcoin.com",
			"Test payment", 10000, nil)
		if err != nil {
			t.Fatalf("Failed to get payment destination : %s", err)
		}
		t.Logf("Script : %x", script)

		ra, err := bitcoin.RawAddressFromLockingScript(script)
		if err != nil {
			t.Fatalf("Failed to parse locking script : %s", err)
		}

		ad := bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet)
		t.Logf("Address : %s", ad.String())
	}
}

func TestP2PPaymentDestination(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)
		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		outputs, err := id.GetP2PPaymentDestination(ctx, 10000)
		if err != nil {
			t.Fatalf("Failed to get p2p payment destination : %s", err)
		}

		for _, output := range outputs.Outputs {
			t.Logf("Output Value %d : Script %x", output.Value, output.LockingScript)

			ra, err := bitcoin.RawAddressFromLockingScript(output.LockingScript)
			if err != nil {
				t.Fatalf("Failed to parse locking script : %s", err)
			}

			ad := bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet)
			t.Logf("Address : %s", ad.String())
		}
	}
}

func TestPaymentRequest(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		request, err := id.GetPaymentRequest(ctx, "John Bitcoin", "john@bitcoin.com",
			"Test payment", "", 10000, nil)
		if err != nil {
			if errors.Cause(err) == ErrNotCapable {
				t.Logf("Payment Request Not Supported")
				continue
			}
			t.Fatalf("Failed to get payment request : %s", err)
		}

		t.Logf("Payment Request : %s", request.Tx.StringWithAddresses(bitcoin.MainNet))
	}
}

func TestAssetAlias(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		request, err := id.ListTokenizedAssets(ctx)
		if err != nil {
			if errors.Cause(err) == ErrNotCapable {
				t.Logf("Payment Request Not Supported")
				continue
			}
			t.Fatalf("Failed to get payment request : %s", err)
		}

		for _, asset := range request {
			t.Logf("Asset alias %s : %s", asset.AssetAlias, asset.AssetID)
		}
	}
}

func TestBRFCID(t *testing.T) {
	// Example
	title := "BRFC Specifications"
	author := "andy (nChain)"
	version := "1"

	hash, _ := bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "57dd1f54fc67" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "57dd1f54fc67")
	}
	t.Logf("BRFC ID : %s", hash.String()[:12])

	// Our payment request transaction BRFC ID
	title = "Payment Requst Transaction"
	author = "Curtis Ellis (Tokenized)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "f7ecaab847eb" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "f7ecaab847eb")
	}
	t.Logf("Payment Request BRFC ID : %s", hash.String()[:12])

	// Our payment request transaction BRFC ID
	title = "List Tokenized Asset Alias"
	author = "Jonathan Vaage (Tokenized)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "e243785d1f17" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "e243785d1f17")
	}
	t.Logf("List Asset Alias BRFC ID : %s", hash.String()[:12])
}

func TestMessageSignature(t *testing.T) {
	request := PaymentDestinationRequest{
		SenderName:   "Curtis Ellis",
		SenderHandle: "loosethinker@moneybutton.com",
		DateTime:     "2020-06-08T20:25:38.199Z",
		Amount:       0,
		Purpose:      "Payment with Money Button",
		Signature:    "H5+9lO39t20kL5GaGJFjauX9by/o4ljlYRMIIIVKY4JqLFPVMVfVCb8nxPOotSJZUppNsckleoqF2VaylpOQYeI=",
	}

	pubKey, err := bitcoin.PublicKeyFromStr("037d391ec99f5fbc48894986391d3d2388045bcf85409ce2e2a92a683dc7a76581")
	if err != nil {
		t.Fatalf("Failed to parse pub key : %s", err)
	}

	if err := request.CheckSignature(pubKey); err != nil {
		t.Fatalf("Failed to check sig : %s", err)
	}

	t.Logf("Signature is valid")
}

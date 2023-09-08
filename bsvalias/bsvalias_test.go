package bsvalias

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

// Keep commented so we don't hit on every test run.
var (
	handles = []string{
		// "test@localhost:8080",
		// "loosethinker@moneybutton.com",
		// "karltheprogrammer@handcash.io",
		// "loosethinker@centbee.com",
		// "centbee@centbee.com",
	}
)

func Test_P2PTransactionSignature(t *testing.T) {
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

func Test_Capabilities(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

		fields := strings.Split(handle, "@")
		site, err := GetSite(ctx, fields[1])
		if err != nil {
			t.Fatalf("Failed to get site : %s", err)
		}

		js, _ := json.MarshalIndent(site, "", "  ")
		t.Logf("Site %s : %s", handle, js)

		t.Logf("%s Site : %+v", fields[1], site)
	}
}

func Test_Identity(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

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

func Test_PublicKey(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

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

func Test_PaymentDestination(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

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

func Test_P2PPaymentDestination(t *testing.T) {
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

func Test_PaymentRequest(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

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

func Test_InstrumentAlias(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		request, err := id.ListTokenizedInstruments(ctx)
		if err != nil {
			if errors.Cause(err) == ErrNotCapable {
				t.Logf("Instrument Alias Not Supported")
				continue
			}
			t.Fatalf("Failed to get instrument alias : %s", err)
		}

		for _, instrument := range request {
			t.Logf("Instrument alias %s : %s", instrument.InstrumentAlias, instrument.InstrumentID)
		}
	}
}

func Test_PublicProfile(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		request, err := id.GetPublicProfile(ctx)
		if err != nil {
			if errors.Cause(err) == ErrNotCapable {
				t.Logf("Public Profile Not Supported")
				continue
			}
			t.Fatalf("Failed to get Public Profile : %s", err)
		}

		js, _ := json.MarshalIndent(request, "", "  ")
		t.Logf("Response : %s", js)

		if request.Name != nil {
			t.Logf("Name : %s", *request.Name)
		}

		if request.AvatarURL != nil {
			t.Logf("AvatarURL : %s", *request.AvatarURL)
		}
	}
}

func Test_NegotiationCapabilities(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		t.Logf("Handle : %s", handle)

		id, err := NewHTTPClient(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		request, err := id.GetNegotiationCapabilities(ctx)
		if err != nil {
			if errors.Cause(err) == ErrNotCapable {
				t.Logf("Public Profile Not Supported")
				continue
			}
			t.Fatalf("Failed to get Public Profile : %s", err)
		}

		js, _ := json.MarshalIndent(request, "", "  ")
		t.Logf("Response : %s", js)
	}
}

func Test_BRFCID(t *testing.T) {
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

	// Our asset alias BRFC ID
	title = "List Tokenized Asset Alias"
	author = "Jonathan Vaage (Tokenized)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "e243785d1f17" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "e243785d1f17")
	}
	t.Logf("List Instrument Alias BRFC ID : %s", hash.String()[:12])

	// Public Profile BRFC ID
	title = "Public Profile (Name & Avatar)"
	author = "Ryan X. Charles (Money Button)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "f12f968c92d6" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "f12f968c92d6")
	}
	t.Logf("Public Profile BRFC ID : %s", hash.String()[:12])

	// Paymail for a full tx used to negotiate a payment or payment request transaction BRFC ID
	title = "Negotiation Transaction"
	author = "Curtis Ellis (Tokenized)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "27d8bd77c113" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "27d8bd77c113")
	}
	t.Logf("Negotiation Transaction BRFC ID : %s", hash.String()[:12])

	// Paymail for capabilities within transaction negotiatiation BRFC ID
	title = "Negotiation Capabilities"
	author = "Curtis Ellis (Tokenized)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "f636191c8fe6" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "f636191c8fe6")
	}
	t.Logf("Negotiation Capabilities BRFC ID : %s", hash.String()[:12])

	// Paymail for providing a merkle proof BRFC ID
	title = "Merkle Proofs"
	author = "Curtis Ellis (Tokenized)"
	version = "1"

	hash, _ = bitcoin.NewHash32(bitcoin.DoubleSha256([]byte(title + author + version)))

	if hash.String()[:12] != "b38a1b09c3ce" {
		t.Fatalf("Invalid ID : got %s, want %s", hash.String()[:12], "b38a1b09c3ce")
	}
	t.Logf("Merkle Proofs BRFC ID : %s", hash.String()[:12])
}

func Test_MessageSignature(t *testing.T) {
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

// Test_CentBee_Signature is an actual attempted payment from CentBee.
func Test_CentBee_Signature(t *testing.T) {
	t.Skip() // I am not sure why this doesn't work. --ce

	// SenderName:centbee@centbee.com
	// SenderHandle:centbee@centbee.com
	// DateTime:2023-05-11T21:31:45.221Z
	// Amount:1
	// Purpose:Payment from centbee@centbee.com
	// Signature:IE8u2xrBCO9DUiUOXWBoBOv9OQG8vyrdVjdQAn27lcV+F1wccvjBBUhIxnj6HsKLvBwfwL2cXyIYhpnL3k86qZs=
	request := PaymentDestinationRequest{
		SenderName:   "centbee@centbee.com",
		SenderHandle: "centbee@centbee.com",
		DateTime:     "2023-05-11T21:31:45.221Z",
		Amount:       1,
		Purpose:      "Payment from centbee@centbee.com",
		Signature:    "IE8u2xrBCO9DUiUOXWBoBOv9OQG8vyrdVjdQAn27lcV+F1wccvjBBUhIxnj6HsKLvBwfwL2cXyIYhpnL3k86qZs=",
	}

	publicKey, err := bitcoin.PublicKeyFromStr("0327948e2082892dd56c707fcde34a0d4e041eca91e6ec80ba6cead44f1ded2cc0")
	if err != nil {
		t.Fatalf("Failed to create public key : %s", err)
	}

	if err := request.CheckSignature(publicKey); err != nil {
		t.Fatalf("Failed to verify signature : %s", err)
	}

	t.Logf("Verified")
}

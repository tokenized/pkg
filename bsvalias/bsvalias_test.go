package bsvalias

import (
	"context"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/tokenized/pkg/bitcoin"
)

// Keep commented so we don't hit moneybutton.com on every test run.
var (
	handles = []string{
		// "test@localhost:8080",
		// "loosethinker@moneybutton.com",
	}
)

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

		id, err := NewIdentity(ctx, handle)
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
		id, err := NewIdentity(ctx, handle)
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
		id, err := NewIdentity(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		script, err := id.GetPaymentDestination("John Bitcoin", "john@bitcoin.com", "Test payment", 10000, nil)
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

func TestPaymentRequest(t *testing.T) {
	ctx := context.Background()

	for _, handle := range handles {
		id, err := NewIdentity(ctx, handle)
		if err != nil {
			t.Fatalf("Failed to get identity : %s", err)
		}

		request, err := id.GetPaymentRequest("John Bitcoin", "john@bitcoin.com", "Test payment", "",
			10000, nil)
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
	t.Logf("BRFC ID : %s", hash.String()[:12])
}

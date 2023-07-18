package main

import (
	"encoding/hex"
	"os"

	"github.com/tokenized/pkg/bitcoin"
)

func main() {
	key, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		println("Failed to generate key:", err.Error())
		os.Exit(1)
	}

	println("Key:", key.String())

	ra, err := key.RawAddress()
	if err == nil {
		println("Address:", bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet).String())
	}

	ls, err := key.LockingScript()
	if err == nil {
		println("Locking Script:", ls.String())
		println("Locking Script (hex):", hex.EncodeToString([]byte(ls)))
	}
}

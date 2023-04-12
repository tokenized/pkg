package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/tokenized/logger"
	"github.com/tokenized/pkg/bitcoin"
)

const usage = ""

func main() {
	ctx := logger.ContextWithLogger(context.Background(), true, false, "")

	if len(os.Args) != 2 {
		fmt.Printf("One argument required : %s\n", usage)
		os.Exit(1)
	}

	if err := convert(ctx, os.Args[1]); err != nil {
		fmt.Printf("Failed to convert value\n")
		os.Exit(1)
	}
}

func convert(ctx context.Context, arg string) error {
	if b, err := hex.DecodeString(arg); err == nil {
		return errors.Wrap(convertBytes(ctx, b), "bytes")
	}

	if b, err := base64.StdEncoding.DecodeString(arg); err == nil {
		return errors.Wrap(convertBytes(ctx, b), "bytes")
	}

	if key, err := bitcoin.KeyFromStr(arg); err == nil {
		return errors.Wrap(printKey(key), "print key")
	}

	if address, err := bitcoin.DecodeAddress(arg); err == nil {
		return errors.Wrap(printAddress(address), "print address")
	}

	if script, err := bitcoin.StringToScript(arg); err == nil {
		return errors.Wrap(printScript(script), "print script")
	} else {
		println("not script", err.Error())
	}

	fmt.Printf("Unknown format\n")
	return nil
}

func convertBytes(ctx context.Context, b []byte) error {
	if len(b) == 33 && (b[0] == 0x02 || b[0] == 0x03) {
		return errors.Wrap(convertPublicKey(ctx, b), "public key")
	}

	if ra, err := bitcoin.DecodeRawAddress(b); err == nil {
		if err := printRawAddress(ra); err != nil {
			return errors.Wrap(err, "print raw address")
		}

		ad := bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet)
		if err := printAddress(ad); err != nil {
			return errors.Wrap(err, "print address")
		}

		return nil
	}

	if _, err := bitcoin.ParseScript(bytes.NewReader(b)); err == nil {
		if err := printScript(b); err != nil {
			return errors.Wrap(err, "print script")
		}

		return nil
	}

	fmt.Printf("Unknown bytes format\n")
	return nil
}

func convertPublicKey(ctx context.Context, b []byte) error {
	publicKey, err := bitcoin.PublicKeyFromBytes(b)
	if err != nil {
		return errors.Wrap(err, "public key from bytes")
	}

	if err := printPublicKey(publicKey); err != nil {
		return errors.Wrap(err, "print public key")
	}

	return nil
}

func printScript(b []byte) error {
	fmt.Printf("Script Size %d bytes\n", len(b))
	fmt.Printf("Script: %s\n", bitcoin.ScriptToString(bitcoin.Script(b)))
	fmt.Printf("Script Hex: %s\n", hex.EncodeToString(b))
	fmt.Printf("Script Base64: %s\n", base64.StdEncoding.EncodeToString(b))

	if bitcoin.Script(b).IsP2PKH() {
		hashes, _ := bitcoin.PKHsFromLockingScript(b)
		if err := printPublicKeyHash(hashes[0]); err != nil {
			return errors.Wrap(err, "print public key hash")
		}
	}

	ra, err := bitcoin.RawAddressFromLockingScript(b)
	if err == nil {
		ad := bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet)
		fmt.Printf("Address: %s\n", ad)
	}

	return nil
}

func printPublicKeyHash(pkh bitcoin.Hash20) error {
	fmt.Printf("Public Key Hash: %s\n", pkh)
	fmt.Printf("Public Key Hash Base64: %s\n", base64.StdEncoding.EncodeToString(pkh.Bytes()))
	return nil
}

func printKey(key bitcoin.Key) error {
	fmt.Printf("WIF: %s\n", key)
	fmt.Printf("Hex Key: %s\n", hex.EncodeToString(key.Bytes()))
	fmt.Printf("Public Key: %s\n", hex.EncodeToString(key.PublicKey().Bytes()))

	if lockingScript, err := key.LockingScript(); err == nil {
		fmt.Printf("P2PKH Script\n")
		if err := printScript(lockingScript); err != nil {
			return errors.Wrap(err, "print script")
		}
	}

	if ra, err := key.RawAddress(); err == nil {
		fmt.Printf("P2PKH Raw Address\n")
		if err := printRawAddress(ra); err != nil {
			return errors.Wrap(err, "print raw address")
		}

		ad := bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet)
		fmt.Printf("P2PKH ")
		fmt.Printf("Address: %s\n", ad)
	}

	return nil
}

func printPublicKey(publicKey bitcoin.PublicKey) error {
	fmt.Printf("Hex Public Key: %s\n", hex.EncodeToString(publicKey.Bytes()))

	if lockingScript, err := publicKey.LockingScript(); err == nil {
		fmt.Printf("P2PKH Script\n")
		if err := printScript(lockingScript); err != nil {
			return errors.Wrap(err, "print script")
		}
	}

	if ra, err := publicKey.RawAddress(); err == nil {
		fmt.Printf("P2PKH Raw Address\n")
		if err := printRawAddress(ra); err != nil {
			return errors.Wrap(err, "print raw address")
		}

		ad := bitcoin.NewAddressFromRawAddress(ra, bitcoin.MainNet)
		fmt.Printf("P2PKH ")
		fmt.Printf("Address: %s\n", ad)
	}

	return nil
}

func printRawAddress(ra bitcoin.RawAddress) error {
	fmt.Printf("Raw Address Hex: %s\n", hex.EncodeToString(ra.Bytes()))
	fmt.Printf("Raw Address Base64: %s\n", base64.StdEncoding.EncodeToString(ra.Bytes()))

	if ra.Type() == bitcoin.ScriptTypePKH {
		pkh, _ := ra.Hash()
		if err := printPublicKeyHash(*pkh); err != nil {
			return errors.Wrap(err, "print public key hash")
		}
	}

	return nil
}

func printAddress(address bitcoin.Address) error {
	fmt.Printf("Address: %s\n", address)

	ra := bitcoin.NewRawAddressFromAddress(address)
	printRawAddress(ra)

	if lockingScript, err := ra.LockingScript(); err == nil {
		fmt.Printf("Locking Script\n")
		if err := printScript(lockingScript); err != nil {
			return errors.Wrap(err, "print script")
		}
	}

	return nil
}

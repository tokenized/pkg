package txbuilder

import (
	"bytes"
	"fmt"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

// InputIsSigned returns true if the input at the specified index already has a signature script.
func (tx *TxBuilder) InputIsSigned(index int) bool {
	if index >= len(tx.MsgTx.TxIn) {
		return false
	}

	return len(tx.MsgTx.TxIn[index].SignatureScript) > 0
}

// AllInputsAreSigned returns true if all inputs have a signature script.
func (tx *TxBuilder) AllInputsAreSigned() bool {
	for _, input := range tx.MsgTx.TxIn {
		if len(input.SignatureScript) == 0 {
			return false
		}
	}
	return true
}

// SignP2PKHInput sets the signature script on the specified PKH input.
// This should only be used when you aren't signing for all inputs and the fee is overestimated, so
//   it needs no adjustement.
func (tx *TxBuilder) SignP2PKHInput(index int, key bitcoin.Key, hashCache *SigHashCache) error {
	if index >= len(tx.Inputs) {
		return errors.New("Input index out of range")
	}

	address, err := bitcoin.RawAddressFromLockingScript(tx.Inputs[index].LockingScript)
	if err != nil {
		return err
	}

	if address.Type() != bitcoin.ScriptTypePKH {
		return errors.Wrap(ErrWrongScriptTemplate, "Not a P2PKH locking script")
	}

	hash, err := address.Hash()
	if err != nil {
		return err
	}
	if !bytes.Equal(hash.Bytes(), bitcoin.Hash160(key.PublicKey().Bytes())) {
		return errors.Wrap(ErrWrongPrivateKey, fmt.Sprintf("Required : %x", hash.Bytes()))
	}

	tx.MsgTx.TxIn[index].SignatureScript, err = P2PKHUnlockingScript(key, tx.MsgTx, index,
		tx.Inputs[index].LockingScript, tx.Inputs[index].Value, SigHashAll+SigHashForkID, hashCache)

	return err
}

// Sign estimates and updates the fee, signs all inputs, and corrects the fee if necessary.
//   keys is a slice of all keys required to sign all inputs. They do not have to be in any order.
// TODO Upgrade to sign more than just P2PKH inputs.
func (tx *TxBuilder) Sign(keys []bitcoin.Key) error {
	// Update fee to estimated amount
	estimatedFee := int64(float32(tx.EstimatedSize()) * tx.FeeRate)
	inputValue := tx.InputValue()
	outputValue := tx.OutputValue(true)
	shc := SigHashCache{}

	if inputValue < outputValue+uint64(estimatedFee) {
		return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
			outputValue+uint64(estimatedFee)))
	}

	var err error
	done := false

	currentFee := int64(inputValue) - int64(outputValue)
	done, err = tx.adjustFee(estimatedFee - currentFee)
	if err != nil {
		if errors.Cause(err) == ErrInsufficientValue {
			return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
				outputValue+uint64(estimatedFee)))
		}
		return err
	}

	attempt := 3 // Max of 3 fee adjustment attempts
	for {
		shc.ClearOutputs()

		// Sign all inputs
		for index, _ := range tx.Inputs {
			if err := tx.signInput(index, keys, shc); err != nil {
				return errors.Wrap(err, "sign input")
			}
		}

		if done || attempt == 0 {
			break
		}

		// Check fee and adjust if too low
		targetFee := int64(float32(tx.MsgTx.SerializeSize()) * tx.FeeRate)
		inputValue = tx.InputValue()
		outputValue = tx.OutputValue(false)
		changeValue := tx.changeSum()
		if inputValue < outputValue+uint64(targetFee) {
			return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
				outputValue+uint64(targetFee)))
		}

		currentFee = int64(inputValue) - int64(outputValue) - int64(changeValue)
		if currentFee >= targetFee && float32(currentFee-targetFee)/float32(targetFee) < 0.05 {
			break // Within 5% of target fee
		}

		done, err = tx.adjustFee(targetFee - currentFee)
		if err != nil {
			if errors.Cause(err) == ErrInsufficientValue {
				return errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
					outputValue+uint64(targetFee)))
			}
			return err
		}

		attempt--
	}

	return nil
}

// SignOnly signs any unsigned inputs in the tx.
// It does not adjust the fee or make any other modifications to the tx like Sign.
func (tx *TxBuilder) SignOnly(keys []bitcoin.Key) error {
	shc := SigHashCache{}

	for index, _ := range tx.Inputs {
		if len(tx.MsgTx.TxIn[index].SignatureScript) > 0 {
			continue // already signed
		}

		if err := tx.signInput(index, keys, shc); err != nil {
			return errors.Wrap(err, "sign input")
		}
	}

	return nil
}

// signInput signs an input of the tx.
func (tx *TxBuilder) signInput(index int, keys []bitcoin.Key, shc SigHashCache) error {
	address, err := bitcoin.RawAddressFromLockingScript(tx.Inputs[index].LockingScript)
	if err != nil {
		return errors.Wrap(err, "locking script")
	}

	switch address.Type() {
	case bitcoin.ScriptTypePKH:
		hash, err := address.Hash()
		if err != nil {
			return errors.Wrap(err, "address hash")
		}
		signed := false
		for _, key := range keys {
			pkh := bitcoin.Hash160(key.PublicKey().Bytes())
			if !bytes.Equal(pkh, hash.Bytes()) {
				continue
			}

			tx.MsgTx.TxIn[index].SignatureScript, err = P2PKHUnlockingScript(key, tx.MsgTx, index,
				tx.Inputs[index].LockingScript, tx.Inputs[index].Value, SigHashAll+SigHashForkID,
				&shc)

			if err != nil {
				return errors.Wrap(err, "unlock script")
			}
			signed = true
			break
		}

		if !signed {
			return ErrMissingPrivateKey
		}

	case bitcoin.ScriptTypePK:
		pubKey, err := address.GetPublicKey()
		if err != nil {
			return err
		}
		pkb := pubKey.Bytes()
		signed := false
		for _, key := range keys {
			pk := key.PublicKey().Bytes()
			if !bytes.Equal(pk, pkb) {
				continue
			}

			tx.MsgTx.TxIn[index].SignatureScript, err = P2PKUnlockingScript(key, tx.MsgTx, index,
				tx.Inputs[index].LockingScript, tx.Inputs[index].Value, SigHashAll+SigHashForkID,
				&shc)

			if err != nil {
				return errors.Wrap(err, "unlock script")
			}
			signed = true
			break
		}

		if !signed {
			return ErrMissingPrivateKey
		}

	default:
		return errors.Wrap(ErrWrongScriptTemplate, "Not a P2PKH or P2PK locking script")
	}

	return nil
}

func P2PKHUnlockingScript(key bitcoin.Key, tx *wire.MsgTx, index int,
	lockScript []byte, value uint64, hashType SigHashType, hashCache *SigHashCache) ([]byte, error) {
	// <Signature> <PublicKey>
	sig, err := InputSignature(key, tx, index, lockScript, value, hashType, hashCache)
	if err != nil {
		return nil, err
	}

	pubkey := key.PublicKey().Bytes()

	buf := bytes.NewBuffer(make([]byte, 0, len(sig)+len(pubkey)+2))
	err = bitcoin.WritePushDataScript(buf, sig)
	if err != nil {
		return nil, err
	}

	err = bitcoin.WritePushDataScript(buf, pubkey)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func P2PKUnlockingScript(key bitcoin.Key, tx *wire.MsgTx, index int,
	lockScript []byte, value uint64, hashType SigHashType, hashCache *SigHashCache) ([]byte, error) {
	// <Signature>
	sig, err := InputSignature(key, tx, index, lockScript, value, hashType, hashCache)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(sig)+1))
	err = bitcoin.WritePushDataScript(buf, sig)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func P2SHUnlockingScript(script []byte) ([]byte, error) {
	// <RedeemScript>...
	return nil, errors.New("SH Unlocking Script Not Implemented") // TODO Implement SH unlocking script
}

// P2MultiPKHUnlockingScript returns an unlocking script for a P2MultiPKH locking script.
// Provide all public keys in order. Signatures should be the same length as the public keys and
//   have empty entries when that key didn't sign.
func P2MultiPKHUnlockingScript(required uint16, pubKeys [][]byte, sigs [][]byte) ([]byte, error) {
	if len(pubKeys) != len(sigs) {
		return nil, errors.New("Same number of public keys and signatures required")
	}

	// For each signer : OP_TRUE + PublicKey + Signature
	// For each non-signer : OP_FALSE
	const persigentry int = 74 + 34 + 1
	buf := bytes.NewBuffer(make([]byte, 0, (int(required)*persigentry)+(len(pubKeys)-int(required))))

	// Add everything in reverse because it will be pushed into the stack (LIFO) and popped out in reverse.
	total := len(pubKeys)
	for i := total - 1; i >= 0; i-- {
		if len(sigs[i]) > 0 {
			if err := bitcoin.WritePushDataScript(buf, sigs[i]); err != nil {
				return nil, err
			}

			if err := bitcoin.WritePushDataScript(buf, pubKeys[i]); err != nil {
				return nil, err
			}

			if err := buf.WriteByte(bitcoin.OP_TRUE); err != nil {
				return nil, err
			}
		} else {
			if err := buf.WriteByte(bitcoin.OP_FALSE); err != nil {
				return nil, err
			}
		}
	}

	return buf.Bytes(), nil
}

func P2RPHUnlockingScript(k []byte) ([]byte, error) {
	// <PublicKey> <Signature(containing r)>
	// k is 256 bit number used to calculate sig with r
	return nil, errors.New("RPH Unlocking Script Not Implemented") // TODO Implement RPH unlocking script
}

// InputSignature returns the serialized ECDSA signature for the input index of the specified
//   transaction, with hashType appended to it.
func InputSignature(key bitcoin.Key, tx *wire.MsgTx, index int, lockScript []byte,
	value uint64, hashType SigHashType, hashCache *SigHashCache) ([]byte, error) {

	hash, err := signatureHash(tx, index, lockScript, value, hashType, hashCache)
	if err != nil {
		return nil, fmt.Errorf("create tx sig hash: %s", err)
	}

	sig, err := key.Sign(hash)
	if err != nil {
		return nil, fmt.Errorf("cannot sign tx input: %s", err)
	}

	return append(sig.Bytes(), byte(hashType)), nil
}

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

	return len(tx.MsgTx.TxIn[index].UnlockingScript) > 0
}

// AllInputsAreSigned returns true if all inputs have a signature script.
func (tx *TxBuilder) AllInputsAreSigned() bool {
	for _, input := range tx.MsgTx.TxIn {
		if len(input.UnlockingScript) == 0 {
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

	tx.MsgTx.TxIn[index].UnlockingScript, err = P2PKHUnlockingScript(key, tx.MsgTx, index,
		tx.Inputs[index].LockingScript, tx.Inputs[index].Value, SigHashAll+SigHashForkID, hashCache)

	return err
}

// Sign estimates and updates the fee, signs all inputs, and corrects the fee if necessary.
//   keys is a slice of all keys required to sign all inputs. They do not have to be in any order.
// TODO Upgrade to sign more than just P2PKH inputs.
func (tx *TxBuilder) Sign(keys []bitcoin.Key) ([]bitcoin.Key, error) {
	// Update fee to estimated amount
	estimatedFee := EstimatedFeeValue(uint64(tx.EstimatedSize()), float64(tx.FeeRate))
	inputValue := tx.InputValue()
	outputValue := tx.OutputValue(true)
	shc := SigHashCache{}

	if inputValue < outputValue+estimatedFee {
		return nil, errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
			outputValue+estimatedFee))
	}

	var err error
	done := false

	currentFee := int64(inputValue) - int64(outputValue)
	done, err = tx.adjustFee(int64(estimatedFee) - currentFee)
	if err != nil {
		if errors.Cause(err) == ErrInsufficientValue {
			return nil, errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
				outputValue+estimatedFee))
		}
		return nil, err
	}

	attempt := 3 // Max of 3 fee adjustment attempts
	for {
		shc.ClearOutputs()
		var result []bitcoin.Key

		// Sign all inputs
		for index, _ := range tx.Inputs {
			keys, err := tx.signInput(index, keys, shc)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("sign input %d", index))
			}

			result = appendKeys(result, keys...)
		}

		if done || attempt == 0 {
			return result, nil
		}

		// Check fee and adjust if too low
		targetFee := int64(EstimatedFeeValue(uint64(tx.MsgTx.SerializeSize()), float64(tx.FeeRate)))
		inputValue = tx.InputValue()
		outputValue = tx.OutputValue(false)
		changeValue := tx.changeSum()
		if inputValue < outputValue+uint64(targetFee) {
			return nil, errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
				outputValue+uint64(targetFee)))
		}

		currentFee = int64(inputValue) - int64(outputValue) - int64(changeValue)
		if currentFee >= targetFee && float32(currentFee-targetFee)/float32(targetFee) < 0.05 {
			// Within 5% of target fee
			return result, nil
		}

		done, err = tx.adjustFee(targetFee - currentFee)
		if err != nil {
			if errors.Cause(err) == ErrInsufficientValue {
				return nil, errors.Wrap(ErrInsufficientValue, fmt.Sprintf("%d/%d", inputValue,
					outputValue+uint64(targetFee)))
			}
			return nil, err
		}

		attempt--
	}
}

// SignOnly signs any unsigned inputs in the tx.
// It does not adjust the fee or make any other modifications to the tx like Sign.
func (tx *TxBuilder) SignOnly(keys []bitcoin.Key) ([]bitcoin.Key, error) {
	shc := SigHashCache{}
	var result []bitcoin.Key
	for index, _ := range tx.Inputs {
		if len(tx.MsgTx.TxIn[index].UnlockingScript) > 0 {
			continue // already signed
		}

		keys, err := tx.signInput(index, keys, shc)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("sign input %d", index))
		}

		result = appendKeys(result, keys...)
	}

	return result, nil
}

func appendKeys(list []bitcoin.Key, keys ...bitcoin.Key) []bitcoin.Key {
	result := list
	for _, key := range keys {
		found := false
		for _, k := range list {
			if k.Equal(key) {
				found = true
				break
			}
		}

		if !found {
			result = append(result, key)
		}
	}

	return result
}

// signInput signs an input of the tx and returns the keys used.
func (tx *TxBuilder) signInput(index int, keys []bitcoin.Key,
	shc SigHashCache) ([]bitcoin.Key, error) {

	lockingScript := tx.Inputs[index].LockingScript
	value := tx.Inputs[index].Value

	if lockingScript.IsP2PKH() {
		for _, key := range keys {
			keyLockingScript, err := key.LockingScript()
			if err != nil {
				return nil, errors.Wrap(err, "key locking script")
			}

			if !keyLockingScript.Equal(lockingScript) {
				continue
			}

			unlockingScript, err := P2PKHUnlockingScript(key, tx.MsgTx, index, lockingScript,
				value, SigHashAll+SigHashForkID, &shc)
			if err != nil {
				return nil, errors.Wrap(err, "unlock script")
			}
			tx.MsgTx.TxIn[index].UnlockingScript = unlockingScript

			return []bitcoin.Key{key}, nil
		}

		return nil, ErrMissingPrivateKey
	}

	if lockingScript.IsP2PK() {
		scriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(lockingScript), -1)
		if err != nil {
			return nil, errors.Wrap(err, "parse locking script")
		}

		if len(scriptItems) != 2 {
			return nil, bitcoin.ErrUnknownScriptTemplate
		}

		pubKeyItem := scriptItems[0]
		if pubKeyItem.Type != bitcoin.ScriptItemTypePushData {
			return nil, bitcoin.ErrUnknownScriptTemplate
		}

		for _, key := range keys {
			if !bytes.Equal(key.PublicKey().Bytes(), pubKeyItem.Data) {
				continue
			}

			unlockingScript, err := P2PKUnlockingScript(key, tx.MsgTx, index, lockingScript,
				value, SigHashAll+SigHashForkID, &shc)
			if err != nil {
				return nil, errors.Wrap(err, "unlock script")
			}
			tx.MsgTx.TxIn[index].UnlockingScript = unlockingScript

			return []bitcoin.Key{key}, nil
		}

		return nil, ErrMissingPrivateKey
	}

	return nil, errors.Wrap(ErrWrongScriptTemplate, "Not a P2PKH or P2PK locking script")
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

	hash, err := SignatureHash(tx, index, lockScript, value, hashType, hashCache)
	if err != nil {
		return nil, fmt.Errorf("create tx sig hash: %s", err)
	}

	sig, err := key.Sign(*hash)
	if err != nil {
		return nil, fmt.Errorf("cannot sign tx input: %s", err)
	}

	return append(sig.Bytes(), byte(hashType)), nil
}

package bitcoin

import (
	"bytes"

	"github.com/pkg/errors"
)

// AddressFromLockingScript returns the address associated with the specified locking script.
func AddressFromLockingScript(lockingScript Script, net Network) (Address, error) {
	ra, err := RawAddressFromLockingScript(lockingScript)
	if err != nil {
		return Address{}, err
	}
	return NewAddressFromRawAddress(ra, net), nil
}

// checkNonStandard returns a non-standard raw address if the locking script is possibly spendable.
func checkNonStandard(lockingScript Script) (RawAddress, error) {
	if LockingScriptIsUnspendable(lockingScript) {
		return RawAddress{}, ErrUnknownScriptTemplate
	}

	return NewRawAddressNonStandard(lockingScript)
}

// RawAddressFromLockingScript returns the script template associated with the specified locking
// script.
func RawAddressFromLockingScript(lockingScript Script) (RawAddress, error) {
	var result RawAddress
	if len(lockingScript) == 0 {
		return result, ErrUnknownScriptTemplate
	}
	script := lockingScript
	switch script[0] {
	case OP_DUP: // PKH or RPH
		if len(script) < 25 {
			return checkNonStandard(lockingScript)
		}
		script = script[1:]
		switch script[0] {
		case OP_HASH160: // PKH
			if len(script) != 24 {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_PUSH_DATA_20 {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			pkh := script[:ScriptHashLength]
			script = script[ScriptHashLength:]

			if script[0] != OP_EQUALVERIFY {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_CHECKSIG {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			err := result.SetPKH(pkh)
			return result, err

		case OP_3: // RPH
			if len(script) != 33 {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_SPLIT {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_NIP {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_1 {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_SPLIT {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_SWAP {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_SPLIT {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_DROP {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_HASH160 {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_PUSH_DATA_20 {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			rph := script[:ScriptHashLength]
			script = script[ScriptHashLength:]

			if script[0] != OP_EQUALVERIFY {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_SWAP {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			if script[0] != OP_CHECKSIG {
				return checkNonStandard(lockingScript)
			}
			script = script[1:]

			err := result.SetRPH(rph)
			return result, err

		}

	case OP_PUSH_DATA_33: // P2PK
		if len(script) != 35 {
			return checkNonStandard(lockingScript)
		}
		script = script[1:]

		pk := script[:PublicKeyCompressedLength]
		script = script[PublicKeyCompressedLength:]

		if script[0] != OP_CHECKSIG {
			return checkNonStandard(lockingScript)
		}
		script = script[1:]

		err := result.SetCompressedPublicKey(pk)
		return result, err

	case OP_HASH160: // P2SH
		if len(script) != 23 {
			return checkNonStandard(lockingScript)
		}
		script = script[1:]

		if script[0] != OP_PUSH_DATA_20 {
			return checkNonStandard(lockingScript)
		}
		script = script[1:]

		sh := script[:ScriptHashLength]
		script = script[ScriptHashLength:]

		if script[0] != OP_EQUAL {
			return checkNonStandard(lockingScript)
		}
		script = script[1:]

		err := result.SetSH(sh)
		return result, err

	case OP_0: // MultiPKH
		// 35 = 1 min number push + 4 op codes outside of pkh if statements + 30 per pkh
		if len(script) < 35 {
			return RawAddress{}, ErrUnknownScriptTemplate
		}
		script = script[1:]

		if script[0] != OP_TOALTSTACK {
			return RawAddress{}, ErrUnknownScriptTemplate
		}
		script = script[1:]

		// Loop through pkhs
		pkhs := make([][]byte, 0, len(script)/30)
		for script[0] == OP_IF {
			script = script[1:]

			if script[0] != OP_DUP {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_HASH160 {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_PUSH_DATA_20 {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			pkhs = append(pkhs, script[:ScriptHashLength])
			script = script[ScriptHashLength:]

			if script[0] != OP_EQUALVERIFY {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_CHECKSIGVERIFY {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_FROMALTSTACK {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_1ADD {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_TOALTSTACK {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if script[0] != OP_ENDIF {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
			script = script[1:]

			if len(script) == 0 {
				return RawAddress{}, ErrUnknownScriptTemplate
			}
		}

		if len(script) < 3 {
			return RawAddress{}, ErrUnknownScriptTemplate
		}

		// Parse required signature count
		required, length, err := ParsePushNumberScript(script)
		if err != nil {
			return RawAddress{}, ErrUnknownScriptTemplate
		}
		script = script[length:]

		if len(script) != 2 {
			return RawAddress{}, ErrUnknownScriptTemplate
		}

		if script[0] != OP_FROMALTSTACK {
			return RawAddress{}, ErrUnknownScriptTemplate
		}
		script = script[1:]

		if script[0] != OP_LESSTHANOREQUAL {
			return RawAddress{}, ErrUnknownScriptTemplate
		}
		script = script[1:]

		err = result.SetMultiPKH(int(required), pkhs)
		return result, err
	}

	return checkNonStandard(lockingScript)
}

func (ra RawAddress) LockingScript() (Script, error) {
	switch ra.scriptType {
	case ScriptTypePKH:
		result := make(Script, 0, 25)

		result = append(result, OP_DUP)
		result = append(result, OP_HASH160)

		// Push public key hash
		result = append(result, OP_PUSH_DATA_20) // Single byte push op code of 20 bytes
		result = append(result, ra.data...)

		result = append(result, OP_EQUALVERIFY)
		result = append(result, OP_CHECKSIG)
		return result, nil

	case ScriptTypePK:
		result := make(Script, 0, PublicKeyCompressedLength+2)

		// Push public key
		result = append(result, OP_PUSH_DATA_33) // Single byte push op code of 33 bytes
		result = append(result, ra.data...)

		result = append(result, OP_CHECKSIG)
		return result, nil

	case ScriptTypeSH:
		result := make(Script, 0, 23)

		result = append(result, OP_HASH160)

		// Push script hash
		result = append(result, OP_PUSH_DATA_20) // Single byte push op code of 20 bytes
		result = append(result, ra.data...)

		result = append(result, OP_EQUAL)
		return result, nil

	case ScriptTypeRPH:
		result := make(Script, 0, 34)

		result = append(result, OP_DUP)
		result = append(result, OP_3)
		result = append(result, OP_SPLIT)
		result = append(result, OP_NIP)
		result = append(result, OP_1)
		result = append(result, OP_SPLIT)
		result = append(result, OP_SWAP)
		result = append(result, OP_SPLIT)
		result = append(result, OP_DROP)
		result = append(result, OP_HASH160)

		// Push r hash
		result = append(result, OP_PUSH_DATA_20) // Single byte push op code of 20 bytes
		result = append(result, ra.data...)

		result = append(result, OP_EQUALVERIFY)
		result = append(result, OP_SWAP)
		result = append(result, OP_CHECKSIG)
		return result, nil

	case ScriptTypeMultiPKH:
		buf := bytes.NewReader(ra.data)

		required, err := ReadBase128VarInt(buf)
		if err != nil {
			return nil, errors.Wrap(err, "read required")
		}

		count, err := ReadBase128VarInt(buf)
		if err != nil {
			return nil, errors.Wrap(err, "read count")
		}

		pkh := make([]byte, ScriptHashLength)

		// 14 = 10 max number push + 4 op codes outside of pkh if statements
		// 30 = 10 op codes + 20 byte pkh per pkh
		result := make(Script, 0, 14+(count*30))

		result = append(result, OP_FALSE)
		result = append(result, OP_TOALTSTACK)

		for i := uint64(0); i < count; i++ {
			// Check if this pkh has a signature
			result = append(result, OP_IF)

			// Check signature against this pkh
			result = append(result, OP_DUP)
			result = append(result, OP_HASH160)

			// Push public key hash
			result = append(result, OP_PUSH_DATA_20) // Single byte push op code of 20 bytes
			if _, err := buf.Read(pkh); err != nil {
				return nil, errors.Wrap(err, "read pkh")
			}
			result = append(result, pkh...)

			result = append(result, OP_EQUALVERIFY)
			result = append(result, OP_CHECKSIGVERIFY)

			// Add 1 to count of valid signatures
			result = append(result, OP_FROMALTSTACK)
			result = append(result, OP_1ADD)
			result = append(result, OP_TOALTSTACK)

			result = append(result, OP_ENDIF)
		}

		// Check required signature count
		result = append(result, PushNumberScript(int64(required))...)
		result = append(result, OP_FROMALTSTACK)
		result = append(result, OP_LESSTHANOREQUAL)
		return result, nil

	case ScriptTypeNonStandard:
		return NewScript(ra.data), nil
	}

	return nil, ErrUnknownScriptTemplate
}

// PublicKeyFromLockingScript returns the serialized compressed public key from the locking script
// if there is one. It only works for P2PK locking scripts.
func PublicKeyFromLockingScript(lockingScript []byte) ([]byte, error) {
	if len(lockingScript) < 2 {
		return nil, ErrUnknownScriptTemplate
	}

	buf := bytes.NewReader(lockingScript)

	// First push
	_, firstPush, err := ParsePushDataScript(buf)
	if err != nil {
		return nil, err
	}

	if isPublicKey(firstPush) {
		return firstPush, nil
	}

	return nil, ErrUnknownScriptTemplate
}

package bitcoin

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	ScriptItemTypeOpCode   = ScriptItemType(0x01)
	ScriptItemTypePushData = ScriptItemType(0x02)

	PublicKeyHashSize   = 20
	SigHashAll          = 0x01
	SigHashAnyOneCanPay = 0x80

	OP_FALSE = byte(0x00)
	OP_TRUE  = byte(0x51)

	OP_1NEGATE = byte(0x4f)

	OP_0  = byte(0x00)
	OP_1  = byte(0x51)
	OP_2  = byte(0x52)
	OP_3  = byte(0x53)
	OP_4  = byte(0x54)
	OP_5  = byte(0x55)
	OP_6  = byte(0x56)
	OP_7  = byte(0x57)
	OP_8  = byte(0x58)
	OP_9  = byte(0x59)
	OP_10 = byte(0x5a)
	OP_11 = byte(0x5b)
	OP_12 = byte(0x5c)
	OP_13 = byte(0x5d)
	OP_14 = byte(0x5e)
	OP_15 = byte(0x5f)
	OP_16 = byte(0x60)

	OP_NOP          = byte(0x61)
	OP_VER          = byte(0x62)
	OP_IF           = byte(0x63)
	OP_NOTIF        = byte(0x64)
	OP_VERIF        = byte(0x65)
	OP_VERNOTIF     = byte(0x66)
	OP_ELSE         = byte(0x67)
	OP_ENDIF        = byte(0x68)
	OP_VERIFY       = byte(0x69)
	OP_RETURN       = byte(0x6a)
	OP_TOALTSTACK   = byte(0x6b)
	OP_FROMALTSTACK = byte(0x6c)
	OP_2DROP        = byte(0x6d)
	OP_2DUP         = byte(0x6e)
	OP_3DUP         = byte(0x6f)
	OP_2OVER        = byte(0x70)
	OP_2ROT         = byte(0x71)
	OP_2SWAP        = byte(0x72)
	OP_IFDUP        = byte(0x73)
	OP_DEPTH        = byte(0x74) // Push the number of items in the stack onto the stack
	OP_DROP         = byte(0x75) // Remove the top stack item
	OP_DUP          = byte(0x76) // Duplicate top item on stack
	OP_NIP          = byte(0x77) // Remove second item deep in stack
	OP_OVER         = byte(0x78) // Copy second stack item to the top
	OP_PICK         = byte(0x79) // Copy item that is n deep in stack and push onto stack
	OP_ROLL         = byte(0x7a) // Move item n deep in stack to the top
	OP_ROT          = byte(0x7b) // Move the top stack item to before the third stack item.
	OP_SWAP         = byte(0x7c) // Reverse the order of the top two stack items
	OP_TUCK         = byte(0x7d) // Copy the item at the top of the stack into before the second item on the stack
	OP_CAT          = byte(0x7e) // Concatenate the top to stack items
	OP_SPLIT        = byte(0x7f) // Split the second item at the index specified in the top item
	OP_NUM2BIN      = byte(0x80) // Convert second item on the stack into a byte sequence the length specified by the first stack item
	OP_BIN2NUM      = byte(0x81) // Convert the top stack item into a number value
	OP_SIZE         = byte(0x82) // Push the length of the top stack item onto the stack

	OP_EQUAL       = byte(0x87)
	OP_EQUALVERIFY = byte(0x88)

	OP_1ADD      = byte(0x8b)
	OP_1SUB      = byte(0x8c)
	OP_2MUL      = byte(0x8d)
	OP_2DIV      = byte(0x8e)
	OP_NEGATE    = byte(0x8f)
	OP_ABS       = byte(0x90)
	OP_NOT       = byte(0x91)
	OP_0NOTEQUAL = byte(0x92)

	OP_ADD    = byte(0x93)
	OP_SUB    = byte(0x94)
	OP_MUL    = byte(0x95)
	OP_DIV    = byte(0x96)
	OP_MOD    = byte(0x97) // Divide a by b and put the remainder on the stack
	OP_LSHIFT = byte(0x98)
	OP_RSHIFT = byte(0x99)

	OP_BOOLAND            = byte(0x9a)
	OP_BOOLOR             = byte(0x9b)
	OP_NUMEQUAL           = byte(0x9c)
	OP_NUMEQUALVERIFY     = byte(0x9d)
	OP_NUMNOTEQUAL        = byte(0x9e)
	OP_LESSTHAN           = byte(0x9f)
	OP_GREATERTHAN        = byte(0xa0)
	OP_LESSTHANOREQUAL    = byte(0xa1)
	OP_GREATERTHANOREQUAL = byte(0xa2)
	OP_MIN                = byte(0xa3)
	OP_MAX                = byte(0xa4)
	OP_WITHIN             = byte(0xa5)

	OP_RIPEMD160           = byte(0xa6)
	OP_SHA1                = byte(0xa7)
	OP_SHA256              = byte(0xa8)
	OP_HASH160             = byte(0xa9)
	OP_HASH256             = byte(0xaa)
	OP_CODESEPARATOR       = byte(0xab)
	OP_CHECKSIG            = byte(0xac)
	OP_CHECKSIGVERIFY      = byte(0xad)
	OP_CHECKMULTISIG       = byte(0xae)
	OP_CHECKMULTISIGVERIFY = byte(0xaf)

	// Bitwise logical operators
	OP_INVERT = byte(0x83)
	OP_AND    = byte(0x84)
	OP_OR     = byte(0x85)
	OP_XOR    = byte(0x86)

	OP_RESERVED  = byte(0x50)
	OP_RESERVED1 = byte(0x89)
	OP_RESERVED2 = byte(0x8a)

	OP_NOP1  = byte(0xb0)
	OP_NOP2  = byte(0xb1)
	OP_NOP3  = byte(0xb2)
	OP_NOP4  = byte(0xb3)
	OP_NOP5  = byte(0xb4)
	OP_NOP6  = byte(0xb5)
	OP_NOP7  = byte(0xb6)
	OP_NOP8  = byte(0xb7)
	OP_NOP9  = byte(0xb8)
	OP_NOP10 = byte(0xb9)

	// These values are place holders in the template for where the public key values should be
	// swapped in when instantiating the template.
	OP_HASH20     = byte(0xb6)
	OP_HASH32     = byte(0xb7)
	OP_PUBKEY     = byte(0xb8) // OP_NOP9 - Must be replaced by a public key
	OP_PUBKEYHASH = byte(0xb9) // OP_NOP10 - Must be replaced by a public key hash

	// Psuedo op codes used as place-holders in template scripts, but not valid in final scripts.
	OP_PUBKEYHASH_ACTUAL    = byte(0xfd)
	OP_PUBKEY_ACTUAL        = byte(0xfe)
	OP_INVALIDOPCODE_ACTUAL = byte(0xff)

	OP_PUSH_DATA_20 = byte(0x14)
	OP_PUSH_DATA_32 = byte(0x20)
	OP_PUSH_DATA_33 = byte(0x21)

	// OP_MAX_SINGLE_BYTE_PUSH_DATA represents the max length for a single byte push
	OP_MAX_SINGLE_BYTE_PUSH_DATA = byte(0x4b)

	// OP_PUSH_DATA_1 represent the OP_PUSHDATA1 opcode.
	OP_PUSH_DATA_1 = byte(0x4c)

	// OP_PUSH_DATA_2 represents the OP_PUSHDATA2 opcode.
	OP_PUSH_DATA_2 = byte(0x4d)

	// OP_PUSH_DATA_4 represents the OP_PUSHDATA4 opcode.
	OP_PUSH_DATA_4 = byte(0x4e)

	// OP_PUSH_DATA_1_MAX is the maximum number of bytes that can be used in the
	// OP_PUSHDATA1 opcode.
	OP_PUSH_DATA_1_MAX = uint64(255)

	// OP_PUSH_DATA_2_MAX is the maximum number of bytes that can be used in the
	// OP_PUSHDATA2 opcode.
	OP_PUSH_DATA_2_MAX = uint64(65535)
)

var (
	endian = binary.LittleEndian

	ErrEmptyScript           = errors.New("Empty Script")
	ErrInvalidScript         = errors.New("Invalid Script")
	ErrNotP2PKH              = errors.New("Not P2PKH")
	ErrWrongScriptTemplate   = errors.New("Wrong Script Template")
	ErrNotPushOp             = errors.New("Not Push Op")
	ErrUnknownScriptNumber   = errors.New("Unknown Script Number")
	ErrWrongOpCode           = errors.New("Wrong Op Code")
	ErrInvalidScriptItemType = errors.New("Invalid Script Item Type")
	ErrNotUnsigned           = errors.New("Not unsigned")
	ErrNumberOverRun         = errors.New("Number Overrun")

	// ErrNotVerifyScript means the script doesn't end with a verify so it isn't safe to embed in
	// some other scripts.
	ErrNotVerifyScript = errors.New("Not Verify Script")

	byteToNames = map[byte]string{
		OP_0:                    "OP_0",
		OP_1NEGATE:              "OP_1NEGATE",
		OP_1:                    "OP_1",
		OP_2:                    "OP_2",
		OP_3:                    "OP_3",
		OP_4:                    "OP_4",
		OP_5:                    "OP_5",
		OP_6:                    "OP_6",
		OP_7:                    "OP_7",
		OP_8:                    "OP_8",
		OP_9:                    "OP_9",
		OP_10:                   "OP_10",
		OP_11:                   "OP_11",
		OP_12:                   "OP_12",
		OP_13:                   "OP_13",
		OP_14:                   "OP_14",
		OP_15:                   "OP_15",
		OP_16:                   "OP_16",
		OP_VER:                  "OP_VER",
		OP_VERIF:                "OP_VERIF",
		OP_VERNOTIF:             "OP_VERNOTIF",
		OP_RETURN:               "OP_RETURN",
		OP_DUP:                  "OP_DUP",
		OP_RIPEMD160:            "OP_RIPEMD160",
		OP_SHA1:                 "OP_SHA1",
		OP_SHA256:               "OP_SHA256",
		OP_HASH160:              "OP_HASH160",
		OP_HASH256:              "OP_HASH256",
		OP_EQUAL:                "OP_EQUAL",
		OP_EQUALVERIFY:          "OP_EQUALVERIFY",
		OP_BOOLAND:              "OP_BOOLAND",
		OP_BOOLOR:               "OP_BOOLOR",
		OP_NUMEQUAL:             "OP_NUMEQUAL",
		OP_NUMEQUALVERIFY:       "OP_NUMEQUALVERIFY",
		OP_NUMNOTEQUAL:          "OP_NUMNOTEQUAL",
		OP_LESSTHAN:             "OP_LESSTHAN",
		OP_GREATERTHAN:          "OP_GREATERTHAN",
		OP_LESSTHANOREQUAL:      "OP_LESSTHANOREQUAL",
		OP_GREATERTHANOREQUAL:   "OP_GREATERTHANOREQUAL",
		OP_MIN:                  "OP_MIN",
		OP_MAX:                  "OP_MAX",
		OP_WITHIN:               "OP_WITHIN",
		OP_CODESEPARATOR:        "OP_CODESEPARATOR",
		OP_CHECKSIG:             "OP_CHECKSIG",
		OP_CHECKSIGVERIFY:       "OP_CHECKSIGVERIFY",
		OP_CHECKMULTISIG:        "OP_CHECKMULTISIG",
		OP_CHECKMULTISIGVERIFY:  "OP_CHECKMULTISIGVERIFY",
		OP_NOP:                  "OP_NOP",
		OP_IF:                   "OP_IF",
		OP_NOTIF:                "OP_NOTIF",
		OP_ELSE:                 "OP_ELSE",
		OP_ENDIF:                "OP_ENDIF",
		OP_VERIFY:               "OP_VERIFY",
		OP_INVERT:               "OP_INVERT",
		OP_AND:                  "OP_AND",
		OP_OR:                   "OP_OR",
		OP_XOR:                  "OP_XOR",
		OP_TOALTSTACK:           "OP_TOALTSTACK",
		OP_FROMALTSTACK:         "OP_FROMALTSTACK",
		OP_1ADD:                 "OP_1ADD",
		OP_1SUB:                 "OP_1SUB",
		OP_2MUL:                 "OP_2MUL",
		OP_2DIV:                 "OP_2DIV",
		OP_NEGATE:               "OP_NEGATE",
		OP_ABS:                  "OP_ABS",
		OP_NOT:                  "OP_NOT",
		OP_0NOTEQUAL:            "OP_0NOTEQUAL",
		OP_ADD:                  "OP_ADD",
		OP_SUB:                  "OP_SUB",
		OP_MUL:                  "OP_MUL",
		OP_DIV:                  "OP_DIV",
		OP_MOD:                  "OP_MOD",
		OP_LSHIFT:               "OP_LSHIFT",
		OP_RSHIFT:               "OP_RSHIFT",
		OP_2DROP:                "OP_2DROP",
		OP_2DUP:                 "OP_2DUP",
		OP_3DUP:                 "OP_3DUP",
		OP_2OVER:                "OP_2OVER",
		OP_2ROT:                 "OP_2ROT",
		OP_2SWAP:                "OP_2SWAP",
		OP_IFDUP:                "OP_IFDUP",
		OP_DEPTH:                "OP_DEPTH",
		OP_NIP:                  "OP_NIP",
		OP_OVER:                 "OP_OVER",
		OP_PICK:                 "OP_PICK",
		OP_ROLL:                 "OP_ROLL",
		OP_ROT:                  "OP_ROT",
		OP_SWAP:                 "OP_SWAP",
		OP_TUCK:                 "OP_TUCK",
		OP_CAT:                  "OP_CAT",
		OP_SPLIT:                "OP_SPLIT",
		OP_NUM2BIN:              "OP_NUM2BIN",
		OP_BIN2NUM:              "OP_BIN2NUM",
		OP_SIZE:                 "OP_SIZE",
		OP_DROP:                 "OP_DROP",
		OP_PUBKEY:               "OP_PUBKEY",
		OP_PUBKEYHASH:           "OP_PUBKEYHASH",
		OP_PUBKEYHASH_ACTUAL:    "OP_PUBKEYHASH_ACTUAL",
		OP_PUBKEY_ACTUAL:        "OP_PUBKEY_ACTUAL",
		OP_INVALIDOPCODE_ACTUAL: "OP_INVALIDOPCODE_ACTUAL",
		OP_RESERVED:             "OP_RESERVED",
		OP_RESERVED1:            "OP_RESERVED1",
		OP_RESERVED2:            "OP_RESERVED2",
		OP_NOP1:                 "OP_NOP1",
		OP_NOP2:                 "OP_NOP2",
		OP_NOP3:                 "OP_NOP3",
		OP_NOP4:                 "OP_NOP4",
		OP_NOP5:                 "OP_NOP5",
		OP_NOP6:                 "OP_NOP6",
		OP_NOP7:                 "OP_NOP7",
		OP_NOP8:                 "OP_NOP8",
	}

	byteFromNames = map[string]byte{
		"OP_FALSE":                OP_FALSE,
		"OP_TRUE":                 OP_TRUE,
		"OP_1NEGATE":              OP_1NEGATE,
		"OP_0":                    OP_0,
		"OP_1":                    OP_1,
		"OP_2":                    OP_2,
		"OP_3":                    OP_3,
		"OP_4":                    OP_4,
		"OP_5":                    OP_5,
		"OP_6":                    OP_6,
		"OP_7":                    OP_7,
		"OP_8":                    OP_8,
		"OP_9":                    OP_9,
		"OP_10":                   OP_10,
		"OP_11":                   OP_11,
		"OP_12":                   OP_12,
		"OP_13":                   OP_13,
		"OP_14":                   OP_14,
		"OP_15":                   OP_15,
		"OP_16":                   OP_16,
		"OP_VER":                  OP_VER,
		"OP_VERIF":                OP_VERIF,
		"OP_VERNOTIF":             OP_VERNOTIF,
		"OP_RETURN":               OP_RETURN,
		"OP_DUP":                  OP_DUP,
		"OP_RIPEMD160":            OP_RIPEMD160,
		"OP_SHA1":                 OP_SHA1,
		"OP_SHA256":               OP_SHA256,
		"OP_HASH160":              OP_HASH160,
		"OP_HASH256":              OP_HASH256,
		"OP_EQUAL":                OP_EQUAL,
		"OP_EQUALVERIFY":          OP_EQUALVERIFY,
		"OP_LESSTHAN":             OP_LESSTHAN,
		"OP_GREATERTHAN":          OP_GREATERTHAN,
		"OP_LESSTHANOREQUAL":      OP_LESSTHANOREQUAL,
		"OP_GREATERTHANOREQUAL":   OP_GREATERTHANOREQUAL,
		"OP_CODESEPARATOR":        OP_CODESEPARATOR,
		"OP_CHECKSIG":             OP_CHECKSIG,
		"OP_CHECKSIGVERIFY":       OP_CHECKSIGVERIFY,
		"OP_CHECKMULTISIG":        OP_CHECKMULTISIG,
		"OP_CHECKMULTISIGVERIFY":  OP_CHECKMULTISIGVERIFY,
		"OP_NOP":                  OP_NOP,
		"OP_IF":                   OP_IF,
		"OP_NOTIF":                OP_NOTIF,
		"OP_ELSE":                 OP_ELSE,
		"OP_ENDIF":                OP_ENDIF,
		"OP_VERIFY":               OP_VERIFY,
		"OP_INVERT":               OP_INVERT,
		"OP_AND":                  OP_AND,
		"OP_OR":                   OP_OR,
		"OP_XOR":                  OP_XOR,
		"OP_TOALTSTACK":           OP_TOALTSTACK,
		"OP_FROMALTSTACK":         OP_FROMALTSTACK,
		"OP_1ADD":                 OP_1ADD,
		"OP_1SUB":                 OP_1SUB,
		"OP_2MUL":                 OP_2MUL,
		"OP_2DIV":                 OP_2DIV,
		"OP_NEGATE":               OP_NEGATE,
		"OP_ABS":                  OP_ABS,
		"OP_NOT":                  OP_NOT,
		"OP_0NOTEQUAL":            OP_0NOTEQUAL,
		"OP_ADD":                  OP_ADD,
		"OP_SUB":                  OP_SUB,
		"OP_MUL":                  OP_MUL,
		"OP_DIV":                  OP_DIV,
		"OP_MOD":                  OP_MOD,
		"OP_LSHIFT":               OP_LSHIFT,
		"OP_RSHIFT":               OP_RSHIFT,
		"OP_DROP":                 OP_DROP,
		"OP_2DROP":                OP_2DROP,
		"OP_2DUP":                 OP_2DUP,
		"OP_3DUP":                 OP_3DUP,
		"OP_2OVER":                OP_2OVER,
		"OP_2ROT":                 OP_2ROT,
		"OP_2SWAP":                OP_2SWAP,
		"OP_IFDUP":                OP_IFDUP,
		"OP_DEPTH":                OP_DEPTH,
		"OP_NIP":                  OP_NIP,
		"OP_OVER":                 OP_OVER,
		"OP_PICK":                 OP_PICK,
		"OP_ROLL":                 OP_ROLL,
		"OP_ROT":                  OP_ROT,
		"OP_SWAP":                 OP_SWAP,
		"OP_TUCK":                 OP_TUCK,
		"OP_CAT":                  OP_CAT,
		"OP_SPLIT":                OP_SPLIT,
		"OP_NUM2BIN":              OP_NUM2BIN,
		"OP_BIN2NUM":              OP_BIN2NUM,
		"OP_SIZE":                 OP_SIZE,
		"OP_PUBKEY":               OP_PUBKEY,
		"OP_PUBKEYHASH":           OP_PUBKEYHASH,
		"OP_PUBKEYHASH_ACTUAL":    OP_PUBKEYHASH_ACTUAL,
		"OP_PUBKEY_ACTUAL":        OP_PUBKEY_ACTUAL,
		"OP_INVALIDOPCODE_ACTUAL": OP_INVALIDOPCODE_ACTUAL,
		"OP_RESERVED":             OP_RESERVED,
		"OP_RESERVED1":            OP_RESERVED1,
		"OP_RESERVED2":            OP_RESERVED2,
		"OP_NOP1":                 OP_NOP1,
		"OP_NOP2":                 OP_NOP2,
		"OP_NOP3":                 OP_NOP3,
		"OP_NOP4":                 OP_NOP4,
		"OP_NOP5":                 OP_NOP5,
		"OP_NOP6":                 OP_NOP6,
		"OP_NOP7":                 OP_NOP7,
		"OP_NOP8":                 OP_NOP8,
		"OP_NOP9":                 OP_NOP9,
		"OP_NOP10":                OP_NOP10,
	}
)

type ScriptItemType uint8

type ScriptItem struct {
	Type   ScriptItemType
	OpCode byte
	Data   Hex
}

type ScriptItems []*ScriptItem

type Script []byte

func (item ScriptItem) String() string {
	if item.Type == ScriptItemTypePushData {
		if isText(item.Data) {
			return fmt.Sprintf("\"%s\"", string(item.Data))
		}

		if value, err := ScriptNumberValue(&item); err == nil && value < 0xffff && value > -0xffff {
			return fmt.Sprintf("%d", value)
		}

		return fmt.Sprintf("0x%s", hex.EncodeToString(item.Data))
	}

	// Op Code
	name, exists := byteToNames[item.OpCode]
	if exists {
		return name
	}

	// Undefined op code
	return fmt.Sprintf("{0x%s}", hex.EncodeToString([]byte{item.OpCode}))
}

func (i ScriptItem) Equal(item ScriptItem) bool {
	if i.Type != item.Type {
		return false
	}

	if i.OpCode != item.OpCode {
		return false
	}

	if !bytes.Equal(i.Data, item.Data) {
		return false
	}

	return true
}

// isText returns true if the byte slice is more than one character, has a letter character, and
// only consists of letters, digits, and spaces.
func isText(bs []byte) bool {
	if len(bs) < 2 {
		return false
	}

	hasLetter := false
	for _, b := range bs {
		if isLetterChar(b) {
			hasLetter = true
		} else if !isNumberChar(b) && b != 0x20 { // space
			return false
		}
	}

	return hasLetter
}

func isLetterChar(b byte) bool {
	if b >= 0x41 && b <= 0x5a {
		return true // A to Z
	}

	if b >= 0x61 && b <= 0x7a {
		return true // a to z
	}

	return false
}

func isNumberChar(b byte) bool {
	return b >= 0x30 && b <= 0x39
}

func ParseScriptItems(buf *bytes.Reader, count int) (ScriptItems, error) {
	if count == -1 {
		// Read all
		var result ScriptItems
		i := 0
		for buf.Len() > 0 {
			item, err := ParseScript(buf)
			if err != nil {
				return nil, errors.Wrapf(err, "item %d", i)
			}

			result = append(result, item)
			i++
		}

		return result, nil
	}

	result := make(ScriptItems, count)
	for i := range result {
		item, err := ParseScript(buf)
		if err != nil {
			return nil, errors.Wrapf(err, "item %d", i)
		}

		result[i] = item
	}

	return result, nil
}

func NewOpCodeScriptItem(opCode byte) *ScriptItem {
	return &ScriptItem{
		Type:   ScriptItemTypeOpCode,
		OpCode: opCode,
	}
}

func NewPushDataScriptItem(b []byte) *ScriptItem {
	return &ScriptItem{
		Type: ScriptItemTypePushData,
		Data: b,
	}
}

func (item ScriptItem) Script() (Script, error) {
	buf := &bytes.Buffer{}
	switch item.Type {
	case ScriptItemTypeOpCode:
		if _, err := buf.Write([]byte{item.OpCode}); err != nil {
			return nil, errors.Wrap(err, "op code")
		}

	case ScriptItemTypePushData:
		if err := WritePushDataScript(buf, item.Data); err != nil {
			return nil, errors.Wrap(err, "data")
		}

	default:
		return nil, errors.Wrapf(ErrInvalidScriptItemType, "%d", item.Type)
	}

	return Script(buf.Bytes()), nil
}

func (item ScriptItem) Write(w io.Writer) error {
	switch item.Type {
	case ScriptItemTypeOpCode:
		if _, err := w.Write([]byte{item.OpCode}); err != nil {
			return errors.Wrap(err, "op code")
		}

	case ScriptItemTypePushData:
		if err := WritePushDataScript(w, item.Data); err != nil {
			return errors.Wrap(err, "data")
		}

	default:
		return errors.Wrapf(ErrInvalidScriptItemType, "%d", item.Type)
	}

	return nil
}

func (items ScriptItems) Script() (Script, error) {
	buf := &bytes.Buffer{}
	for i, item := range items {
		switch item.Type {
		case ScriptItemTypeOpCode:
			if _, err := buf.Write([]byte{item.OpCode}); err != nil {
				return nil, errors.Wrapf(err, "item %d: op code", i)
			}

		case ScriptItemTypePushData:
			if err := WritePushDataScript(buf, item.Data); err != nil {
				return nil, errors.Wrapf(err, "item %d: data", i)
			}

		default:
			return nil, errors.Wrapf(ErrInvalidScriptItemType, "item %d: data", i)
		}
	}

	return Script(buf.Bytes()), nil
}

func (items ScriptItems) Write(w io.Writer) error {
	for i, item := range items {
		switch item.Type {
		case ScriptItemTypeOpCode:
			if _, err := w.Write([]byte{item.OpCode}); err != nil {
				return errors.Wrapf(err, "item %d: op code", i)
			}

		case ScriptItemTypePushData:
			if err := WritePushDataScript(w, item.Data); err != nil {
				return errors.Wrapf(err, "item %d: data", i)
			}

		default:
			return errors.Wrapf(ErrInvalidScriptItemType, "item %d: data", i)
		}
	}

	return nil
}

func NewScript(b []byte) Script {
	return Script(b)
}

// Value returns a value that can be handled by a database driver to put values in the database.
func (s Script) Value() (driver.Value, error) {
	return []byte(s), nil
}

func (s Script) PubKeyCount() uint32 {
	buf := bytes.NewReader(s)
	result := uint32(0)
	for {
		item, err := ParseScript(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0
		}

		if item.Type == ScriptItemTypePushData {
			l := len(item.Data)
			if l == Hash20Size || l == PublicKeyCompressedLength {
				result++
			}

			continue
		}
	}

	return result
}

// RequiredSignatures is the number of signatures required to unlock the template.
// Note: Only supports P2PKH and MultiPKH.
func (s Script) RequiredSignatures() (uint32, error) {
	if s.MatchesTemplate(PKHTemplate) || s.MatchesTemplate(PKTemplate) {
		return 1, nil
	}

	if required, _, err := s.MultiPKHCounts(); err == nil {
		return required, nil
	} else {
		return 0, errors.Wrap(err, "multi-pkh counts")
	}

	return 0, ErrUnknownScriptTemplate
}

func (s Script) IsP2PKH() bool {
	return s.MatchesTemplate(PKHTemplate)
}

func (s Script) IsP2PK() bool {
	return s.MatchesTemplate(PKTemplate)
}

func (s Script) MatchesTemplate(template Template) bool {
	buf := bytes.NewReader(s)
	for {
		item, err := ParseScript(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return false
		}

		if len(template) == 0 {
			return false
		}

		expected := template[0]
		template = template[1:]

		if expected == OP_PUBKEYHASH {
			if item.Type != ScriptItemTypePushData || len(item.Data) != Hash20Size {
				return false
			}
		} else if expected == OP_PUBKEY {
			if item.Type != ScriptItemTypePushData || len(item.Data) != PublicKeyCompressedLength {
				return false
			}
		} else if item.OpCode != expected {
			return false
		}
	}

	return len(template) == 0
}

func CheckOpCode(r *bytes.Reader, opCode byte) error {
	item, err := ParseScript(r)
	if err != nil {
		return errors.Wrap(err, "parse script")
	}

	if item.Type == ScriptItemTypePushData {
		return errors.Wrap(ErrWrongOpCode, "is push op")
	}

	if item.OpCode == opCode {
		return nil
	}

	return errors.Wrapf(ErrWrongOpCode, "got %s, want %s", OpCodeToString(item.OpCode),
		OpCodeToString(opCode))
}

// MultiPKHCounts returns the number of require signatures and total signers for a multi-pkh locking
// script. It returns the error ErrWrongScriptTemplate if the locking script doesn't match the
// template.
//
// Returns:
// Required Signatures Count
// Total Signers Count
func (s Script) MultiPKHCounts() (uint32, uint32, error) {
	r := bytes.NewReader(s)

	// Initialize alt stack
	if err := CheckOpCode(r, OP_0); err != nil {
		return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
	}

	if err := CheckOpCode(r, OP_TOALTSTACK); err != nil {
		return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
	}

	// For each signer
	total := uint32(0)
	var requiredSigners int64
	for {
		item, err := ParseScript(r)
		if err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		if item.Type != ScriptItemTypeOpCode || item.OpCode != OP_IF {
			requiredSigners, err = ScriptNumberValue(item)
			if err != nil {
				return 0, 0, errors.Wrap(ErrWrongScriptTemplate,
					errors.Wrap(err, "required signers script number").Error())
			}

			break // this must be the required signers count
		}

		if item.OpCode != OP_IF {
			return 0, 0, errors.Wrapf(ErrWrongScriptTemplate, "not OP_IF: %s",
				OpCodeToString(item.OpCode))
		}

		if err := CheckOpCode(r, OP_DUP); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		if err := CheckOpCode(r, OP_HASH160); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		pubKeyItem, err := ParseScript(r)
		if err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate,
				errors.Wrap(err, "public key hash").Error())
		}

		if pubKeyItem.Type != ScriptItemTypePushData {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, "public key hash not push data")
		}

		if len(pubKeyItem.Data) != PublicKeyHashSize {
			return 0, 0, errors.Wrapf(ErrWrongScriptTemplate,
				"wrong public key hash size: got %d, want %d", len(pubKeyItem.Data),
				PublicKeyHashSize)
		}

		if err := CheckOpCode(r, OP_EQUALVERIFY); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		if err := CheckOpCode(r, OP_CHECKSIGVERIFY); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		// Increment alt stack for a valid signature
		if err := CheckOpCode(r, OP_FROMALTSTACK); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		if err := CheckOpCode(r, OP_1ADD); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		if err := CheckOpCode(r, OP_TOALTSTACK); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		if err := CheckOpCode(r, OP_ENDIF); err != nil {
			return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
		}

		total++
	}

	// Check alt stack
	// Already got {required signers count} in loop to break from the loop
	if requiredSigners < 1 || requiredSigners > 0xffffffff {
		return 0, 0, errors.Wrapf(ErrUnknownScriptTemplate, "require signer value %d",
			requiredSigners)
	}

	if err := CheckOpCode(r, OP_FROMALTSTACK); err != nil {
		return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
	}

	if err := CheckOpCode(r, OP_LESSTHANOREQUAL); err != nil {
		return 0, 0, errors.Wrap(ErrWrongScriptTemplate, err.Error())
	}

	if _, err := ParseScript(r); errors.Cause(err) != io.EOF {
		return 0, 0, errors.Wrap(ErrWrongScriptTemplate, "not end of script")
	}

	return uint32(requiredSigners), total, nil
}

// IsSigHashAll returns true if all valid signatures in this unlocking script have the sighashall
// flag set. It ignores invalid signatures as that would be caught by the transaction validation.
// This function is concerned with a valid transaction when an input is not signed sig hash all, so
// parts of the transaction were not signed by this unlocking script.
func (s Script) IsSigHashAll() (bool, error) {
	items, err := ParseScriptItems(bytes.NewReader(s), -1)
	if err != nil {
		return false, errors.Wrap(err, "parse")
	}

	for _, item := range items {
		if item.Type != ScriptItemTypePushData {
			continue
		}

		l := len(item.Data)
		if l < 2 {
			continue // not a valid signature, so ignore it
		}

		sigHashType := item.Data[l-1]
		sig := item.Data[:l-1]

		signature, err := SignatureFromBytes(sig)
		if err != nil {
			continue // not a valid signature, so ignore it
		}

		if err := signature.Validate(); err != nil {
			continue // not a valid signature, so ignore it
		}

		if sigHashType&SigHashAll == 0 {
			return false, nil // sig hash type does not have sig hash all flag set
		}
	}

	return true, nil
}

func (s Script) IsAnyoneCanPay() (bool, error) {
	items, err := ParseScriptItems(bytes.NewReader(s), -1)
	if err != nil {
		return false, errors.Wrap(err, "parse")
	}

	for _, item := range items {
		if item.Type != ScriptItemTypePushData {
			continue
		}

		l := len(item.Data)
		if l < 2 {
			continue // not a valid signature, so ignore it
		}

		sigHashType := item.Data[l-1]
		sig := item.Data[:l-1]

		signature, err := SignatureFromBytes(sig)
		if err != nil {
			continue // not a valid signature, so ignore it
		}

		if err := signature.Validate(); err != nil {
			continue // not a valid signature, so ignore it
		}

		if sigHashType&SigHashAnyOneCanPay == 0 {
			return false, nil // sig hash type does not have sig hash anyone can pay flag set
		}
	}

	return true, nil
}

func (s Script) IsFalseOpReturn() bool {
	l := len(s)
	if l < 2 {
		return false
	}

	return s[0] == OP_FALSE && s[1] == OP_RETURN
}

// AddHardVerify attempts to update a script to end with a verify so that it can be embedded within
// another script and still fail that script if it fails. This assumes the script it is to be
// embedded within expects sub-scripts to verify and provides if statements to skip over sub-scripts
// that don't need to be executed. If the sub-script doesn't end with a verify and it fails the
// containing script might still count it as valid.
// It returns an error if the script could not be modified to end with a verify and it didn't
// already end with a verify, meaning it may not be safe to embed in another script.
// For example a multi-pkh script provides if statements to only execute the pkh sub scripts that
// are provided by the unlocking script.
func (s *Script) AddHardVerify() error {
	items, err := ParseScriptItems(bytes.NewReader(*s), -1)
	if err != nil {
		return errors.Wrap(err, "parse")
	}

	itemCount := len(items)
	if itemCount == 0 {
		return ErrEmptyScript
	}
	lastItem := items[itemCount-1]

	if lastItem.Type != ScriptItemTypeOpCode {
		return errors.Wrap(ErrNotVerifyScript, "last item not op code")
	}

	switch lastItem.OpCode {
	case OP_EQUAL:
		// Convert to verify op code
		(*s)[len(*s)-1] = OP_EQUALVERIFY
		return nil

	case OP_CHECKSIG:
		// Convert to verify op code
		(*s)[len(*s)-1] = OP_CHECKSIGVERIFY
		return nil

	case OP_CHECKMULTISIG:
		// Convert to verify op code
		(*s)[len(*s)-1] = OP_CHECKMULTISIGVERIFY
		return nil

	case OP_EQUALVERIFY, OP_CHECKSIGVERIFY, OP_CHECKMULTISIGVERIFY:
		return nil // Already a verify op code

	case OP_LESSTHAN, OP_GREATERTHAN, OP_LESSTHANOREQUAL, OP_GREATERTHANOREQUAL:
		// Append verify op code
		*s = append(*s, OP_VERIFY)
		return nil
	}

	return errors.Wrapf(ErrNotVerifyScript, "unsupported last item: %s", lastItem.String())
}

func AddHardVerify(s Script) (Script, error) {
	copy := s.Copy()
	if err := copy.AddHardVerify(); err != nil {
		return nil, err
	}
	return copy, nil
}

func (s *Script) RemoveHardVerify() error {
	items, err := ParseScriptItems(bytes.NewReader(*s), -1)
	if err != nil {
		return errors.Wrap(err, "parse")
	}

	itemCount := len(items)
	if itemCount == 0 {
		return ErrEmptyScript
	}
	lastItem := items[itemCount-1]

	if lastItem.Type != ScriptItemTypeOpCode {
		return errors.Wrap(ErrNotVerifyScript, "last item not op code")
	}

	switch lastItem.OpCode {
	case OP_EQUALVERIFY:
		// Convert to non-verify op code
		(*s)[len(*s)-1] = OP_EQUAL
		return nil

	case OP_CHECKSIGVERIFY:
		// Convert to non-verify op code
		(*s)[len(*s)-1] = OP_CHECKSIG
		return nil

	case OP_CHECKMULTISIGVERIFY:
		// Convert to non-verify op code
		(*s)[len(*s)-1] = OP_CHECKMULTISIG
		return nil

	case OP_EQUAL, OP_CHECKSIG, OP_CHECKMULTISIG:
		return nil // Already a verify op code

	case OP_LESSTHAN, OP_GREATERTHAN, OP_LESSTHANOREQUAL, OP_GREATERTHANOREQUAL:
		return nil // Already a verify op code

	case OP_VERIFY:
		// Remove OP_VERIFY (last op-code)
		*s = (*s)[:len(*s)-1]
		return nil
	}

	return errors.Wrapf(ErrNotVerifyScript, "unsupported last item: %s", lastItem.String())
}

func RemoveHardVerify(s Script) (Script, error) {
	copy := s.Copy()
	if err := copy.RemoveHardVerify(); err != nil {
		return nil, err
	}
	return copy, nil
}

func (s Script) Equal(r Script) bool {
	return bytes.Equal(s, r)
}

func (s Script) Copy() Script {
	c := make(Script, len(s))
	copy(c, s)
	return c
}

func (s Script) String() string {
	return ScriptToString(s)
}

func (s Script) Bytes() []byte {
	return s
}

// MarshalText returns the text encoding of the raw address.
// Implements encoding.TextMarshaler interface.
func (s Script) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText parses a text encoded raw address and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (s *Script) UnmarshalText(text []byte) error {
	b, err := StringToScript(string(text))
	if err != nil {
		return errors.Wrap(err, "script to string")
	}

	return s.UnmarshalBinary(b)
}

// MarshalBinary returns the binary encoding of the raw address.
// Implements encoding.BinaryMarshaler interface.
func (s Script) MarshalBinary() ([]byte, error) {
	return s.Bytes(), nil
}

// UnmarshalBinary parses a binary encoded raw address and sets the value of this object.
// Implements encoding.BinaryUnmarshaler interface.
func (s *Script) UnmarshalBinary(data []byte) error {
	// Copy byte slice in case it is reused after this call.
	*s = make([]byte, len(data))
	copy(*s, data)
	return nil
}

// MarshalJSON converts to json.
func (s Script) MarshalJSON() ([]byte, error) {
	return []byte("\"" + hex.EncodeToString(s) + "\""), nil
}

// UnmarshalJSON converts from json.
func (s *Script) UnmarshalJSON(data []byte) error {
	l := len(data)
	if l < 2 {
		return fmt.Errorf("Too short for Script hex data : %d", l)
	}

	if l == 2 {
		*s = nil
		return nil
	}

	// Decode hex and remove double quotes.
	raw, err := hex.DecodeString(string(data[1 : l-1]))
	if err != nil {
		return err
	}
	*s = raw

	return nil
}

// Scan converts from a database column.
func (s *Script) Scan(data interface{}) error {
	if data == nil {
		*s = nil
		return nil
	}

	b, ok := data.([]byte)
	if !ok {
		return errors.New("Script db column not bytes")
	}

	if len(b) == 0 {
		*s = nil
		return nil
	}

	// Copy byte slice because it will be wiped out by the database after this call.
	*s = make([]byte, len(b))
	copy(*s, b)

	return nil
}

// PushDataSize returns the size of the script item needed to push data of a specified size.
func PushDataSize(size int) int {
	if size <= int(OP_MAX_SINGLE_BYTE_PUSH_DATA) {
		return 1 + size
	} else if uint64(size) < OP_PUSH_DATA_1_MAX {
		return 2 + size
	} else if uint64(size) < OP_PUSH_DATA_2_MAX {
		return 3 + size
	}

	return 4 + size
}

// PushDataScriptSize returns the encoded push data script size op codes.
func PushDataScriptSize(size uint64) []byte {
	if size <= uint64(OP_MAX_SINGLE_BYTE_PUSH_DATA) {
		return []byte{byte(size)} // Single byte push
	} else if size < OP_PUSH_DATA_1_MAX {
		return []byte{OP_PUSH_DATA_1, byte(size)}
	} else if size < OP_PUSH_DATA_2_MAX {
		var buf bytes.Buffer
		binary.Write(&buf, endian, OP_PUSH_DATA_2)
		binary.Write(&buf, endian, uint16(size))
		return buf.Bytes()
	}

	var buf bytes.Buffer
	binary.Write(&buf, endian, OP_PUSH_DATA_4)
	binary.Write(&buf, endian, uint32(size))
	return buf.Bytes()
}

// PushDataScript writes a push data bitcoin script including the encoded size preceding it.
func WritePushDataScript(w io.Writer, data []byte) error {
	size := len(data)
	var err error
	if size <= int(OP_MAX_SINGLE_BYTE_PUSH_DATA) {
		_, err = w.Write([]byte{byte(size)}) // Single byte push
	} else if size < int(OP_PUSH_DATA_1_MAX) {
		_, err = w.Write([]byte{OP_PUSH_DATA_1, byte(size)})
	} else if size < int(OP_PUSH_DATA_2_MAX) {
		_, err = w.Write([]byte{OP_PUSH_DATA_2})
		if err != nil {
			return err
		}
		err = binary.Write(w, endian, uint16(size))
	} else {
		_, err = w.Write([]byte{OP_PUSH_DATA_4})
		if err != nil {
			return err
		}
		err = binary.Write(w, endian, uint32(size))
	}
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// ParseScript will parse the next item of a bitcoin script.
// A bytes.Reader object is needed to check the size against the remaining length before allocating
// the memory to store the push.
func ParseScript(buf *bytes.Reader) (*ScriptItem, error) {
	var opCode byte
	if err := binary.Read(buf, endian, &opCode); err != nil {
		return &ScriptItem{
			Type:   ScriptItemTypeOpCode,
			OpCode: 0,
			Data:   nil,
		}, err
	}

	isPushOp := false
	dataSize := 0
	if opCode == OP_FALSE {
		return &ScriptItem{
			Type:   ScriptItemTypeOpCode,
			OpCode: opCode,
			Data:   nil,
		}, nil
	} else if opCode <= OP_MAX_SINGLE_BYTE_PUSH_DATA {
		isPushOp = true
		dataSize = int(opCode)
	} else if opCode >= OP_1 && opCode <= OP_16 {
		return &ScriptItem{
			Type:   ScriptItemTypeOpCode,
			OpCode: opCode,
			Data:   nil,
		}, nil
	} else if opCode == OP_1NEGATE {
		return &ScriptItem{
			Type:   ScriptItemTypeOpCode,
			OpCode: opCode,
			Data:   nil,
		}, nil
	} else {
		switch opCode {
		case OP_PUSH_DATA_1:
			var size uint8
			if err := binary.Read(buf, endian, &size); err != nil {
				return nil, err
			}
			isPushOp = true
			dataSize = int(size)
		case OP_PUSH_DATA_2:
			var size uint16
			if err := binary.Read(buf, endian, &size); err != nil {
				return nil, err
			}
			isPushOp = true
			dataSize = int(size)
		case OP_PUSH_DATA_4:
			var size uint32
			if err := binary.Read(buf, endian, &size); err != nil {
				return nil, err
			}
			isPushOp = true
			dataSize = int(size)
		}
	}

	if !isPushOp {
		return &ScriptItem{
			Type:   ScriptItemTypeOpCode,
			OpCode: opCode,
			Data:   nil,
		}, nil
	}
	if dataSize == 0 {
		return &ScriptItem{
			Type:   ScriptItemTypePushData,
			OpCode: opCode,
			Data:   nil,
		}, nil
	}

	if dataSize > buf.Len() { // Check this to prevent trying to allocate a large amount.
		return nil, errors.Wrap(ErrInvalidScript,
			fmt.Sprintf("Push data size past end of script : %d/%d", dataSize, buf.Len()))
	}

	data := make([]byte, dataSize)
	if _, err := buf.Read(data); err != nil {
		return nil, err
	}

	return &ScriptItem{
		Type:   ScriptItemTypePushData,
		OpCode: opCode,
		Data:   data,
	}, nil
}

// ParsePushDataScriptSize will parse a push data script and return its size.
func ParsePushDataScriptSize(buf io.Reader) (uint64, error) {
	var opCode byte
	err := binary.Read(buf, endian, &opCode)
	if err != nil {
		return 0, err
	}

	if opCode <= OP_MAX_SINGLE_BYTE_PUSH_DATA {
		return uint64(opCode), nil
	}

	switch opCode {
	case OP_PUSH_DATA_1:
		var size uint8
		err := binary.Read(buf, endian, &size)
		if err != nil {
			return 0, err
		}
		return uint64(size), nil
	case OP_PUSH_DATA_2:
		var size uint16
		err := binary.Read(buf, endian, &size)
		if err != nil {
			return 0, err
		}
		return uint64(size), nil
	case OP_PUSH_DATA_4:
		var size uint32
		err := binary.Read(buf, endian, &size)
		if err != nil {
			return 0, err
		}
		return uint64(size), nil
	default:
		return 0, errors.Wrap(ErrNotPushOp, fmt.Sprintf("Invalid push data op code : 0x%02x", opCode))
	}
}

// ParsePushDataScript will parse a bitcoin script for the next "object". It will return the next
// op code, and if that op code is a push data op code, it will return the data.
// A bytes.Reader object is needed to check the size against the remaining length before allocating
// the memory to store the push.
func ParsePushDataScript(buf *bytes.Reader) (uint8, []byte, error) {
	var opCode byte
	err := binary.Read(buf, endian, &opCode)
	if err != nil {
		return 0, nil, err
	}

	isPushOp := false
	dataSize := 0
	if opCode == OP_FALSE {
		return opCode, nil, nil
	} else if opCode <= OP_MAX_SINGLE_BYTE_PUSH_DATA {
		isPushOp = true
		dataSize = int(opCode)
	} else if opCode >= OP_1 && opCode <= OP_16 {
		return opCode, []byte{opCode - OP_1 + 1}, nil
	} else if opCode == OP_1NEGATE {
		return opCode, []byte{0xff}, nil
	} else {
		switch opCode {
		case OP_PUSH_DATA_1:
			var size uint8
			err := binary.Read(buf, endian, &size)
			if err != nil {
				return 0, nil, err
			}
			isPushOp = true
			dataSize = int(size)
		case OP_PUSH_DATA_2:
			var size uint16
			err := binary.Read(buf, endian, &size)
			if err != nil {
				return 0, nil, err
			}
			isPushOp = true
			dataSize = int(size)
		case OP_PUSH_DATA_4:
			var size uint32
			err := binary.Read(buf, endian, &size)
			if err != nil {
				return 0, nil, err
			}
			isPushOp = true
			dataSize = int(size)
		}
	}

	if !isPushOp {
		return opCode, nil, ErrNotPushOp
	}
	if dataSize == 0 {
		return opCode, nil, nil
	}

	if dataSize > buf.Len() { // Check this to prevent trying to allocate a large amount.
		return 0, nil, fmt.Errorf("Push data size past end of script : %d/%d", dataSize, buf.Len())
	}

	data := make([]byte, dataSize)
	_, err = buf.Read(data)
	if err != nil {
		return 0, nil, err
	}
	return opCode, data, nil
}

// PushNumberScript returns a section of script that will push the specified number onto the stack.
// Example encodings:
//
//	   127 -> [0x7f]
//	  -127 -> [0xff]
//	   128 -> [0x80 0x00]
//	  -128 -> [0x80 0x80]
//	   129 -> [0x81 0x00]
//	  -129 -> [0x81 0x80]
//	   256 -> [0x00 0x01]
//	  -256 -> [0x00 0x81]
//	 32767 -> [0xff 0x7f]
//	-32767 -> [0xff 0xff]
//	 32768 -> [0x00 0x80 0x00]
//	-32768 -> [0x00 0x80 0x80]
func PushNumberScriptItem(n int64) *ScriptItem {
	// OP_FALSE, OP_0
	if n == 0 {
		return NewOpCodeScriptItem(OP_0)
	}

	// OP_1NEGATE
	if n == -1 {
		return NewOpCodeScriptItem(OP_1NEGATE)
	}

	// Single byte number push op codes
	if n > 0 && n <= 16 {
		return NewOpCodeScriptItem(0x50 + byte(n))
	}

	// Take the absolute value and keep track of whether it was originally
	// negative.
	isNegative := n < 0
	if isNegative {
		n = -n
	}

	// Encode to little endian.  The maximum number of encoded bytes is 9
	// (8 bytes for max int64 plus a potential byte for sign extension).
	result := make(Script, 0, 10)
	for n > 0 {
		result = append(result, byte(n&0xff))
		n >>= 8
	}

	// When the most significant byte already has the high bit set, an
	// additional high byte is required to indicate whether the number is
	// negative or positive.  The additional byte is removed when converting
	// back to an integral and its high bit is used to denote the sign.
	//
	// Otherwise, when the most significant byte does not already have the
	// high bit set, use it to indicate the value is negative, if needed.
	if result[len(result)-1]&0x80 != 0 {
		extraByte := byte(0x00)
		if isNegative {
			extraByte = 0x80
		}
		result = append(result, extraByte)

	} else if isNegative {
		result[len(result)-1] |= 0x80
	}

	return NewPushDataScriptItem(result)
}

func PushNumberScriptItemUnsigned(n uint64) *ScriptItem {
	// OP_FALSE, OP_0
	if n == 0 {
		return NewOpCodeScriptItem(OP_0)
	}

	// Single byte number push op codes
	if n > 0 && n <= 16 {
		return NewOpCodeScriptItem(0x50 + byte(n))
	}

	// Encode to little endian.  The maximum number of encoded bytes is 9
	// (8 bytes for max int64 plus a potential byte for sign extension).
	result := make(Script, 0, 10)
	for n > 0 {
		result = append(result, byte(n&0xff))
		n >>= 8
	}

	// When the most significant byte already has the high bit set, an
	// additional high byte is required to indicate whether the number is
	// negative or positive.  The additional byte is removed when converting
	// back to an integral and its high bit is used to denote the sign.
	//
	// Otherwise, when the most significant byte does not already have the
	// high bit set, use it to indicate the value is negative, if needed.
	if result[len(result)-1]&0x80 != 0 {
		extraByte := byte(0x00)
		result = append(result, extraByte)
	}

	return NewPushDataScriptItem(result)
}

func PushNumberScript(n int64) Script {
	item := PushNumberScriptItem(n)
	script, _ := item.Script()
	return script
}

// ScriptNumberValue returns the number value given the op code or push data returned from
// ParseScript.
func ScriptNumberValue(item *ScriptItem) (int64, error) {
	if item.Type == ScriptItemTypePushData {
		return DecodeScriptLittleEndian(item.Data)
	}

	if item.OpCode >= OP_1 && item.OpCode <= OP_16 {
		return int64(item.OpCode - 0x50), nil
	}

	switch item.OpCode {
	case OP_FALSE:
		return 0, nil
	case OP_1NEGATE:
		return -1, nil
	}

	return 0, errors.Wrapf(ErrUnknownScriptNumber, "op code : %s, data : %x",
		OpCodeToString(item.OpCode), item.Data)
}

func ScriptNumberValueUnsigned(item *ScriptItem) (uint64, error) {
	if item.Type == ScriptItemTypePushData {
		return DecodeScriptLittleEndianUnsigned(item.Data)
	}

	if item.OpCode >= OP_1 && item.OpCode <= OP_16 {
		return uint64(item.OpCode - 0x50), nil
	}

	switch item.OpCode {
	case OP_FALSE:
		return 0, nil
	case OP_1NEGATE:
		return 0, ErrNotUnsigned
	}

	return 0, errors.Wrapf(ErrUnknownScriptNumber, "op code : %s, data : %x",
		OpCodeToString(item.OpCode), item.Data)
}

// ParsePushNumberScript reads a number out of script and returns the value, the bytes of script it
// used, and an error if one occured.
func ParsePushNumberScript(b []byte) (int64, int, error) {
	if len(b) == 0 {
		return 0, 0, errors.New("Script empty")
	}

	// Zero
	if b[0] == OP_FALSE {
		return 0, 1, nil
	}

	// Negative one
	if b[0] == 0x4f {
		return -1, 1, nil
	}

	// 1 - 16 (single byte number op codes)
	if b[0] >= 0x51 && b[0] <= 0x60 {
		return int64(b[0] - 0x50), 1, nil
	}

	if b[0] > 10 {
		return 0, 0, errors.New("Invalid push op for number")
	}

	length := int(b[0])
	b = b[1:]

	// Decode from little endian.
	v, err := DecodeScriptLittleEndian(b)
	if err != nil {
		return 0, 0, errors.Wrap(err, "decode")
	}

	return v, length, nil
}

func DecodeScriptLittleEndian(b []byte) (int64, error) {
	if len(b) > 8 {
		return 0, errors.Wrapf(ErrNumberOverRun, "%d bytes doesn't fit in int64", len(b))
	}

	var result int64
	for i, val := range b {
		result |= int64(val) << uint8(8*i)
	}

	// When the most significant byte of the input bytes has the sign bit set, the result is
	// negative.  So, remove the sign bit from the result and make it negative.
	if b[len(b)-1]&0x80 != 0 {
		// The maximum length of v has already been determined to be 4 above, so uint8 is enough to
		// cover the max possible shift value of 24.
		result &= ^(int64(0x80) << uint8(8*(len(b)-1)))
		result = -result
	}

	return result, nil
}

func DecodeScriptLittleEndianUnsigned(b []byte) (uint64, error) {
	if len(b) > 8 {
		return 0, errors.Wrapf(ErrNumberOverRun, "%d bytes doesn't fit in uint64", len(b))
	}

	var result uint64
	for i, val := range b {
		result |= uint64(val) << uint8(8*i)
	}

	// When the most significant byte of the input bytes has the sign bit set, the result is
	// negative.  So, remove the sign bit from the result and make it negative.
	if b[len(b)-1]&0x80 != 0 {
		return 0, ErrNotUnsigned
	}

	return result, nil
}

func PubKeyFromP2PKHSigScript(script []byte) ([]byte, error) {
	buf := bytes.NewReader(script)

	// Signature
	_, signature, err := ParsePushDataScript(buf)
	if err != nil {
		return nil, err
	}
	if len(signature) == 0 {
		return nil, ErrNotP2PKH
	}

	// Public Key
	_, publicKey, err := ParsePushDataScript(buf)
	if err != nil {
		return nil, err
	}
	if len(publicKey) == 0 {
		return nil, ErrNotP2PKH
	}

	return publicKey, nil
}

func PubKeyHashFromP2PKHSigScript(script []byte) ([]byte, error) {
	publicKey, err := PubKeyFromP2PKHSigScript(script)
	if err != nil {
		return nil, err
	}

	// Hash public key
	return Hash160(publicKey), nil
}

func PubKeysFromSigScript(script []byte) ([][]byte, error) {
	buf := bytes.NewReader(script)
	result := make([][]byte, 0)

	for {
		_, pushdata, err := ParsePushDataScript(buf)

		if err == nil {
			if isPublicKey(pushdata) {
				result = append(result, pushdata)
			}
			continue
		}

		if err == io.EOF { // finished parsing script
			break
		}
		if err != ErrNotPushOp { // ignore non push op codes
			return nil, err
		}
	}

	return result, nil
}

func PKHsFromLockingScript(script []byte) ([]Hash20, error) {
	buf := bytes.NewReader(script)
	result := make([]Hash20, 0)

	for {
		_, pushdata, err := ParsePushDataScript(buf)

		if err == nil {
			// Check for public key hash
			if len(pushdata) == Hash20Size {
				hash, _ := NewHash20(pushdata)
				result = append(result, *hash)
			}

			// Check for public key, then hash it
			if len(pushdata) == PublicKeyCompressedLength &&
				(pushdata[0] == 0x02 || pushdata[0] == 0x03) {
				hash, _ := NewHash20(Hash160(pushdata))
				result = append(result, *hash)
			}
			continue
		}

		if err == io.EOF { // finished parsing script
			break
		}
		if err != ErrNotPushOp { // ignore non push op codes
			return nil, err
		}
	}

	return result, nil
}

// LockingScriptIsUnspendable returns true if the locking script is known to be unspendable.
func LockingScriptIsUnspendable(script []byte) bool {
	l := len(script)
	if l == 0 {
		return false
	}

	if script[0] == OP_RETURN {
		return true
	}

	if l > 1 && script[0] == OP_FALSE && script[1] == OP_RETURN {
		return true
	}

	return false
}

func OpCodeToString(opCode byte) string {
	name, exists := byteToNames[opCode]
	if exists {
		return name
	}

	return fmt.Sprintf("{0x%s}", hex.EncodeToString([]byte{opCode}))
}

// ScriptToString converts a bitcoin script into a text representation.
func ScriptToString(script Script) string {
	var result []string
	buf := bytes.NewReader(script)

	for {
		item, err := ParseScript(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		result = append(result, item.String())
	}

	return strings.Join(result, " ")
}

// StringToScript converts a text representation of a bitcoin script to a string.
func StringToScript(text string) (Script, error) {
	buf := &bytes.Buffer{}

	var previousParts string
	parts := strings.Fields(text)
	for _, part := range parts {
		firstChar := part[0]
		lastChar := part[len(part)-1]

		if len(previousParts) > 0 {
			part = previousParts + " " + part
			if lastChar != '"' {
				previousParts = part
				continue
			}
		}

		if firstChar == '"' && lastChar != '"' {
			previousParts = part
			continue
		}

		firstChar = part[0]
		lastChar = part[len(part)-1]

		if len(part) > 3 && part[:3] == "OP_" {
			opCode, exists := byteFromNames[part]
			if exists {
				buf.WriteByte(opCode)
				continue
			}
		}

		if len(part) >= 2 && part[:2] == "0x" {
			b, err := hex.DecodeString(part[2:]) // skip leading "0x"
			if err != nil {
				return nil, errors.Wrapf(err, "decode push data hex: %s", part[2:])
			}

			if err := WritePushDataScript(buf, b); err != nil {
				return nil, errors.Wrap(err, "write push data")
			}
			continue
		}

		if int, err := strconv.ParseInt(part, 10, 64); err == nil {
			item := PushNumberScriptItem(int)
			if err := item.Write(buf); err != nil {
				return nil, errors.Wrap(err, "write number")
			}
			continue
		}

		if len(part) < 2 {
			return nil, fmt.Errorf("Invalid part : \"%s\"", part)
		}

		if firstChar == '"' && lastChar == '"' {
			// Text
			if err := WritePushDataScript(buf, []byte(part[1:len(part)-1])); err != nil {
				return nil, errors.Wrap(err, "write push data")
			}

			continue
		}

		if firstChar == '{' && lastChar == '}' {
			// Undefined op code
			b, err := hex.DecodeString(part[1 : len(part)-1])
			if err != nil {
				return nil, errors.Wrapf(err, "decode undefined op code hex: %s", part)
			}

			buf.Write(b)
			continue
		}

		return nil, errors.Wrap(errors.New("Unknown Script Item"), part)
	}

	return Script(buf.Bytes()), nil
}

func CleanScriptParts(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ") // Remove new lines
	text = strings.ReplaceAll(text, "\r", " ") // Remove carriage returns
	text = strings.ReplaceAll(text, "\t", " ") // Remove tabs

	// Remove extra spaces
	parts := strings.Fields(text)
	var remaining []string
	for _, part := range parts {
		if len(part) > 0 {
			remaining = append(remaining, part)
		}
	}

	return remaining
}

func CleanScriptText(text string) string {
	return strings.Join(CleanScriptParts(text), " ")
}

// BytePushData returns the push op to push a single byte value to the stack.
func BytePushData(b byte) Script {
	if b == 0 {
		return Script{OP_0}
	}

	if b <= 16 {
		return Script{OP_1 + b - 1}
	}

	return Script{0x01, b} // one byte push data
}

// PushData
func PushData(b []byte) Script {
	script, err := NewPushDataScriptItem(b).Script()
	if err != nil {
		panic(fmt.Sprintf("Failed to create push data script : %s", err))
	}

	return script
}

type byteOrScript interface{} // or byte slice, which is the same as script

// ConcatScript concatenates op codes and sections of script into one script.
func ConcatScript(bs ...byteOrScript) Script {
	l := 0
	for _, v := range bs {
		switch b := v.(type) {
		case []byte:
			l += len(b)
		case Script:
			l += len(b)
		case byte:
			l += 1
		default:
			panic(fmt.Sprintf("Unknown concat script value type : %s : %v",
				typeName(reflect.TypeOf(v)), v))
		}
	}

	result := make(Script, l)
	offset := 0
	for _, v := range bs {
		switch b := v.(type) {
		case []byte:
			copy(result[offset:], b)
			offset += len(b)
		case Script:
			copy(result[offset:], b)
			offset += len(b)
		case byte:
			result[offset] = b
			offset += 1

		default:
			panic(fmt.Sprintf("Unknown concat script value type : %s", typeName(reflect.TypeOf(v))))
		}
	}

	return result
}

func typeName(typ reflect.Type) string {
	kind := typ.Kind()
	switch kind {
	case reflect.Ptr, reflect.Slice, reflect.Array:
		return fmt.Sprintf("%s:%s", kind, typeName(typ.Elem()))
	case reflect.Struct:
		return typ.Name()
	default:
		return kind.String()
	}
}

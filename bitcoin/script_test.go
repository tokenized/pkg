package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

var tests = []struct {
	size   uint64
	script []byte
}{
	// Single byte pushes (op code is 8 bit integer representing size to push)
	{0, []byte{0}},
	{10, []byte{10}},
	{0x4b, []byte{0x4b}},

	// OP_PUSHDATA1 (push code 0x4c followed by 1 byte for size)
	{0x4c, []byte{0x4c, 0x4c}},
	{0x50, []byte{0x4c, 0x50}},

	// OP_PUSHDATA2 (push code 0x4d followed by 2 bytes for size)
	{0x1050, []byte{0x4d, 0x50, 0x10}},
	{0xff50, []byte{0x4d, 0x50, 0xff}},

	// OP_PUSHDATA4 (push code 0x4e followed by 4 bytes for size)
	{0x0010ff50, []byte{0x4e, 0x50, 0xff, 0x10, 0x00}},
	{0x00ffff50, []byte{0x4e, 0x50, 0xff, 0xff, 0x00}},
}

func TestPushDataScriptSize(t *testing.T) {
	for i, test := range tests {
		result := PushDataScriptSize(test.size)
		if !bytes.Equal(result, test.script) {
			t.Fatalf("Failed test %d :\nResult : %+v\nCorrect : %+v\n", i, result, test.script)
		}
	}
}

func TestWritePushDataScript(t *testing.T) {
	for i, test := range tests {
		data := make([]byte, test.size)
		var buf bytes.Buffer
		err := WritePushDataScript(&buf, data)
		if err != nil {
			t.Fatalf("Failed to write %d push data script : %s", test.size, err)
		}
		result := buf.Bytes()
		result = result[:len(test.script)]
		if !bytes.Equal(result, test.script) {
			t.Fatalf("Failed test %d :\nResult : %+v\nCorrect : %+v\n", i, result, test.script)
		}
	}
}

func TestParsePushDataScriptSize(t *testing.T) {
	for i, test := range tests {
		buf := bytes.NewReader(test.script)
		result, err := ParsePushDataScriptSize(buf)
		if err != nil {
			t.Fatalf("Failed test %d : %s", i, err)
		}
		if result != test.size {
			t.Fatalf("Failed test %d :\nResult : %d\nCorrect : %d\n", i, result, test.size)
		}
	}
}

func TestParsePushDataScript(t *testing.T) {
	for i, test := range tests {
		buf := bytes.NewBuffer(test.script)
		data := make([]byte, test.size)
		_, err := buf.Write(data)
		if err != nil {
			t.Fatalf("Failed to write push data for test %d : %s", i, err)
		}
		r := bytes.NewReader(buf.Bytes())
		_, result, err := ParsePushDataScript(r)
		if err != nil && err != ErrNotPushOp {
			t.Fatalf("Failed test %d : %s", i, err)
		}
		if uint64(len(result)) != test.size {
			t.Fatalf("Failed test %d :\nResult : %d\nCorrect : %d\n", i, result, test.size)
		}
	}
}

func TestScriptToString(t *testing.T) {
	tests := []struct {
		name string
		text string
		hex  string
	}{
		{
			name: "PKH",
			text: "OP_DUP OP_HASH160 0x999ac355257736dfa1ad9652fcb51c7136fc27f9 OP_EQUALVERIFY OP_CHECKSIG",
			hex:  "76a914999ac355257736dfa1ad9652fcb51c7136fc27f988ac",
		},
		{
			name: "Text",
			text: "OP_0 OP_RETURN \"test text\"",
			hex:  "006a09746573742074657874",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatalf("Failed to decode hex : %s", err)
			}

			str := ScriptToString(script)
			if str != tt.text {
				t.Fatalf("Wrong text : \ngot  : %s\nwant : %s", str, tt.text)
			}
			t.Logf("String : %s", str)

			scr, err := StringToScript(tt.text)
			if err != nil {
				t.Fatalf("Failed to convert string to script : %s", err)
			}

			if !bytes.Equal(scr, script) {
				t.Fatalf("Wrong bytes : \ngot  : %x\nwant : %x", scr, script)
			}
		})
	}
}

func Test_MatchesTemplate(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		template string
	}{
		{
			name:     "PKH",
			text:     "OP_DUP OP_HASH160 0x999ac355257736dfa1ad9652fcb51c7136fc27f9 OP_EQUALVERIFY OP_CHECKSIG",
			template: "OP_DUP OP_HASH160 OP_PUBKEYHASH OP_EQUALVERIFY OP_CHECKSIG",
		},
		{
			name:     "PK",
			text:     "0x029ac355257736dfa1ad9652fcb51c7136fc27f9ad9652fcb51c7136fc27f95257 OP_CHECKSIG",
			template: "OP_PUBKEY OP_CHECKSIG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := StringToScript(tt.text)
			if err != nil {
				t.Fatalf("Failed to decode script : %s", err)
			}
			template, err := StringToScript(tt.template)
			if err != nil {
				t.Fatalf("Failed to decode template : %s", err)
			}

			if !script.MatchesTemplate(Template(template)) {
				t.Fatalf("Failed to match template")
			}
		})
	}
}

func Test_MatchesTemplate_MultiPKH(t *testing.T) {
	tests := []struct {
		required uint32
		count    uint32
	}{
		{
			required: 1,
			count:    1,
		},
		{
			required: 1,
			count:    2,
		},
		{
			required: 2,
			count:    3,
		},
		{
			required: 3,
			count:    3,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d of %d", tt.required, tt.count), func(t *testing.T) {
			template, err := NewMultiPKHTemplate(tt.required, tt.count)
			if err != nil {
				t.Fatalf("Failed to create template : %s", err)
			}

			pubKeys := make([]PublicKey, tt.count)
			for i := range pubKeys {
				key, _ := GenerateKey(MainNet)
				pubKeys[i] = key.PublicKey()
			}

			script, err := template.LockingScript(pubKeys)
			if err != nil {
				t.Fatalf("Failed to create script : %s", err)
			}

			if !script.MatchesTemplate(template) {
				t.Fatalf("Failed to match template")
			}

			count := script.PubKeyCount()
			if count != tt.count {
				t.Fatalf("Wrong script pub key count : got %d, want %d", count, tt.count)
			}

			required, err := script.RequiredSignatures()
			if err != nil {
				t.Fatalf("Failed to get script required signature count : %s", err)
			}

			if required != tt.required {
				t.Fatalf("Wrong script required signature count : got %d, want %d", required,
					tt.required)
			}

			count = template.PubKeyCount()
			if count != tt.count {
				t.Fatalf("Wrong template pub key count : got %d, want %d", count, tt.count)
			}

			required, err = template.RequiredSignatures()
			if err != nil {
				t.Fatalf("Failed to get template required signature count : %s", err)
			}

			if required != tt.required {
				t.Fatalf("Wrong template required signature count : got %d, want %d", required,
					tt.required)
			}
		})
	}
}

func Test_PubKeyCount_RequiredSignatures(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		count    uint32
		required uint32
	}{
		{
			name:     "PKH",
			text:     "OP_DUP OP_HASH160 0x999ac355257736dfa1ad9652fcb51c7136fc27f9 OP_EQUALVERIFY OP_CHECKSIG",
			count:    1,
			required: 1,
		},
		{
			name:     "PK",
			text:     "0x029ac355257736dfa1ad9652fcb51c7136fc27f9ad9652fcb51c7136fc27f95257 OP_CHECKSIG",
			count:    1,
			required: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := StringToScript(tt.text)
			if err != nil {
				t.Fatalf("Failed to decode script : %s", err)
			}

			count := script.PubKeyCount()
			if count != tt.count {
				t.Fatalf("Wrong pub key count : got %d, want %d", count, tt.count)
			}

			required, err := script.RequiredSignatures()
			if err != nil {
				t.Fatalf("Failed to get required signature count : %s", err)
			}

			if required != tt.required {
				t.Fatalf("Wrong required signature count : got %d, want %d", required, tt.required)
			}
		})
	}
}

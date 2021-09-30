package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func Test_TemplateEncoding(t *testing.T) {
	tests := []struct {
		name string
		text string
		hex  string
	}{
		{
			name: "PKH",
			text: "OP_DUP OP_HASH160 OP_PUBKEYHASH OP_EQUALVERIFY OP_CHECKSIG",
			hex:  "76a9b988ac",
		},
		{
			name: "MultiPKH_1_2",
			text: `OP_FALSE OP_TOALTSTACK OP_IF OP_DUP OP_HASH160 OP_PUBKEYHASH OP_EQUALVERIFY
				OP_CHECKSIGVERIFY OP_FROMALTSTACK OP_1ADD OP_TOALTSTACK OP_ENDIF OP_IF OP_DUP
				OP_HASH160 OP_PUBKEYHASH OP_EQUALVERIFY OP_CHECKSIGVERIFY OP_FROMALTSTACK OP_1ADD
				OP_TOALTSTACK OP_ENDIF OP_FROMALTSTACK OP_1 OP_GREATERTHANOREQUAL`,
			hex: "006b6376a9b988ad6c8b6b686376a9b988ad6c8b6b686c51a2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var template Template

			if err := template.UnmarshalText([]byte(tt.text)); err != nil {
				t.Fatalf("Failed to unmarshal text : %s", err)
			}

			b, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatalf("Failed to decode hex : %s", err)
			}

			t.Logf("Script Hex : %x", template.Bytes())

			if !bytes.Equal(b, template.Bytes()) {
				t.Fatalf("Wrong bytes : \ngot  : %x\nwant : %x", b, template.Bytes())
			}

			t.Logf("Script : %s", template.String())

			text := CleanScriptText(tt.text)

			if template.String() != text {
				t.Fatalf("Wrong text : \ngot  : %s\nwant : %s", template.String(), text)
			}
		})
	}
}

func Test_TemplatePKH(t *testing.T) {
	key, err := GenerateKey(TestNet)
	if err != nil {
		t.Fatalf("Failed to generate key 1 : %s", err)
	}

	ra, err := key.RawAddress()
	if err != nil {
		t.Fatalf("Failed to generate raw address : %s", err)
	}

	script, err := ra.LockingScript()
	if err != nil {
		t.Fatalf("Failed to generate script : %s", err)
	}

	t.Logf("Script : %s", ScriptToString(script))

	template := PKHTemplate
	t.Logf("Template : %x", template.Bytes())

	templateScript, err := template.LockingScript([]PublicKey{key.PublicKey()})
	if err != nil {
		t.Fatalf("Failed to create template script : %s", err)
	}

	t.Logf("Template Script : %s", templateScript)

	if !bytes.Equal(script, templateScript) {
		t.Fatalf("Wrong script : \ngot  : %x\nwant : %x", script, templateScript.Bytes())
	}
}

func Test_TemplateMultiPKH(t *testing.T) {
	key1, err := GenerateKey(TestNet)
	if err != nil {
		t.Fatalf("Failed to generate key 1 : %s", err)
	}

	key2, err := GenerateKey(TestNet)
	if err != nil {
		t.Fatalf("Failed to generate key 2 : %s", err)
	}

	ra, err := NewRawAddressMultiPKH(1, [][]byte{Hash160(key1.PublicKey().Bytes()),
		Hash160(key2.PublicKey().Bytes())})
	if err != nil {
		t.Fatalf("Failed to generate raw address : %s", err)
	}

	script, err := ra.LockingScript()
	if err != nil {
		t.Fatalf("Failed to generate script : %s", err)
	}

	t.Logf("Script : %s", ScriptToString(script))

	template, err := NewMultiPKHTemplate(1, 2)
	if err != nil {
		t.Fatalf("Failed to create template : %s", err)
	}

	t.Logf("Template : %x", template.Bytes())

	templateScript, err := template.LockingScript([]PublicKey{key1.PublicKey(), key2.PublicKey()})
	if err != nil {
		t.Fatalf("Failed to create template script : %s", err)
	}

	t.Logf("Template Script : %s", templateScript)

	if !bytes.Equal(script, templateScript) {
		t.Fatalf("Wrong script : \ngot  : %x\nwant : %x", script, templateScript.Bytes())
	}
}

func Test_TemplateLockingScript(t *testing.T) {
	tests := []struct {
		name      string
		publicKey string
		hex       string
	}{
		{
			name:      "PKH",
			publicKey: "0313545ddbd2a185c7ac71c7d0e458e4739fee73923ab067e4d87bde7156756032",
			hex:       "76a914999ac355257736dfa1ad9652fcb51c7136fc27f988ac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicKey, err := PublicKeyFromStr(tt.publicKey)
			if err != nil {
				t.Fatalf("Failed to parse public key : %s", err)
			}

			template := PKHTemplate

			script, err := template.LockingScript([]PublicKey{publicKey})
			if err != nil {
				t.Fatalf("Failed to create locking script : %s", err)
			}

			b, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatalf("Failed to decode hex : %s", err)
			}

			if !bytes.Equal(b, script.Bytes()) {
				t.Fatalf("Wrong bytes : \ngot  : %x\nwant : %x", b, script.Bytes())
			}
		})
	}
}

func Test_PKH_RequiredSignatures(t *testing.T) {
	result, err := PKHTemplate.RequiredSignatures()
	if err != nil {
		t.Fatalf("Failed to get required signatures : %s", err)
	}

	if result != 1 {
		t.Fatalf("Wrong required signatures : got %d, want %d", result, 1)
	}

	total := PKHTemplate.PubKeyCount()

	if total != 1 {
		t.Fatalf("Wrong total : got %d, want %d", total, 1)
	}
}

func Test_MultiPKH_RequiredSignatures(t *testing.T) {
	tests := []struct {
		required uint32
		total    uint32
	}{
		{
			required: 1,
			total:    3,
		},
		{
			required: 2,
			total:    3,
		},
		{
			required: 1,
			total:    2,
		},
		{
			required: 2,
			total:    2,
		},
		{
			required: 3,
			total:    4,
		},
		{
			required: 150,
			total:    160,
		},
		{
			required: 300,
			total:    350,
		},
		{
			required: 0xff,
			total:    350,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d of %d", tt.required, tt.total), func(t *testing.T) {
			template, err := NewMultiPKHTemplate(tt.required, tt.total)
			if err != nil {
				t.Fatalf("Failed to create template : %s", err)
			}

			result, err := template.RequiredSignatures()
			if err != nil {
				t.Fatalf("Failed to get required signatures : %s", err)
			}

			if result != tt.required {
				t.Fatalf("Wrong required signatures : got %d, want %d", result, tt.required)
			}

			total := template.PubKeyCount()

			if total != tt.total {
				t.Fatalf("Wrong total : got %d, want %d", total, tt.total)
			}

			t.Logf("Required %d of total %d", result, total)
		})
	}
}

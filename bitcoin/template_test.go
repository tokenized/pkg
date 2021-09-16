package bitcoin

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestTemplateEncoding(t *testing.T) {
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

func TestTemplatePKH(t *testing.T) {
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

	template := NewPKHTemplate()

	t.Logf("Template : %x", template.Bytes())

	templateScript, err := template.LockingScript([]PublicKey{key.PublicKey()})
	if err != nil {
		t.Fatalf("Failed to create template script : %s", err)
	}

	t.Logf("Template Script : %s", ScriptToString(templateScript))

	if !bytes.Equal(script, templateScript) {
		t.Fatalf("Wrong script : \ngot  : %x\nwant : %x", script, templateScript)
	}
}

func TestTemplateMultiPKH(t *testing.T) {
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

	t.Logf("Template Script : %s", ScriptToString(templateScript))

	if !bytes.Equal(script, templateScript) {
		t.Fatalf("Wrong script : \ngot  : %x\nwant : %x", script, templateScript)
	}
}

func TestTemplateLockingScript(t *testing.T) {
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

			template := NewPKHTemplate()

			script, err := template.LockingScript([]PublicKey{publicKey})
			if err != nil {
				t.Fatalf("Failed to create locking script : %s", err)
			}

			b, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatalf("Failed to decode hex : %s", err)
			}

			if !bytes.Equal(b, script) {
				t.Fatalf("Wrong bytes : \ngot  : %x\nwant : %x", b, script)
			}
		})
	}
}

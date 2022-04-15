package bsor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
)

func Test_ParseElement(t *testing.T) {
	tests := []struct {
		script bitcoin.Script
		id     uint64
		value  []byte
	}{
		{
			script: bitcoin.Script{bitcoin.OP_1, 4, 't', 'e', 's', 't'},
			id:     1,
			value:  []byte("test"),
		},
		{
			script: bitcoin.Script{2, 0xD2, 0x04, 4, 't', 'e', 's', 't'},
			id:     1234,
			value:  []byte("test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.script.String(), func(t *testing.T) {
			element, err := ParseElement(bytes.NewReader(tt.script))
			if err != nil {
				t.Fatalf("Failed to parse element : %s", err)
			}

			if element.ID != tt.id {
				t.Errorf("Wrong element ID : got %d, want %d", element.ID, tt.id)
			}

			if element.Value.Type == bitcoin.ScriptItemTypeOpCode {

			} else if !bytes.Equal(element.Value.Data, tt.value) {
				t.Errorf("Wrong element value : got %x, want %x", element.Value.Data, tt.value)
			}

			js, _ := json.MarshalIndent(element, "", "  ")
			t.Logf("Element : %s", js)
		})
	}
}

type TestStruct1 struct {
	Field1 int    `bsor:"1"`
	Field2 string `bsor:"2"`
}

func Test_Marshal_TestStruct1(t *testing.T) {
	tests := []struct {
		value TestStruct1
	}{
		{
			value: TestStruct1{
				Field1: 10,
				Field2: "test",
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			script, err := Marshal(tt.value)
			if err != nil {
				t.Fatalf("Failed to marshal struct : %s", err)
			}

			js, _ := json.MarshalIndent(tt.value, "", "  ")
			t.Logf("Element : %s", js)

			t.Logf("Script : %s", script)
		})
	}
}

package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestBase128(t *testing.T) {
	tests := []struct {
		hex   string
		value uint64
	}{
		{
			hex:   "b0b8d0a2a4b081a316",
			value: 1604976374254410800,
		},
		{
			hex:   "00",
			value: 0,
		},
		{
			hex:   "01",
			value: 1,
		},
		{
			hex:   "0f",
			value: 15,
		},
		{
			hex:   "ff01",
			value: 255,
		},
		{
			hex:   "AC02",
			value: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			b, err := hex.DecodeString(tt.hex)
			if err != nil {
				t.Fatal(err)
			}

			value, err := ReadBase128VarInt(bytes.NewReader(b))
			if err != nil {
				t.Fatalf("Failed to read : %s", err)
			}

			if value != tt.value {
				t.Errorf("Wrong value : got %d, want %d", value, tt.value)
			}

			var w bytes.Buffer
			if err := WriteBase128VarInt(&w, value); err != nil {
				t.Fatalf("Failed to write : %s", err)
			}

			if !bytes.Equal(w.Bytes(), b) {
				t.Errorf("Wrong hex : got %x, want %x", w.Bytes(), b)
			}
		})
	}
}

func TestBase128Signed(t *testing.T) {
	tests := []struct {
		hex   string
		value int64
	}{
		{
			hex:   "b0b8d0a2a4b081a316",
			value: 1604976374254410800,
		},
		{
			hex:   "00",
			value: 0,
		},
		{
			hex:   "01",
			value: 1,
		},
		{
			hex:   "0f",
			value: 15,
		},
		{
			hex:   "ff01",
			value: 255,
		},
		{
			hex:   "ac02",
			value: 300,
		},
		{
			hex:   "ac02",
			value: -300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			var w bytes.Buffer
			if err := WriteBase128VarSignedInt(&w, tt.value); err != nil {
				t.Fatalf("Failed to write : %s", err)
			}

			fmt.Printf("Bytes : %x\n", w.Bytes())

			value, err := ReadBase128VarSignedInt(&w)
			if err != nil {
				t.Fatalf("Failed to read : %s", err)
			}

			if value != tt.value {
				t.Errorf("Wrong value : got %d, want %d", value, tt.value)
			}

			t.Logf("Value : %d", value)
		})
	}
}

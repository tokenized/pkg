package bitcoin

import (
	"bytes"
	"encoding/hex"
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

			t.Logf("Bytes : %x\n", w.Bytes())

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

func Test_BIP0276(t *testing.T) {
	tests := []struct {
		name         string
		encodedValue string
		prefix       string
		net          Network
		decodedHex   string
	}{
		{
			name:         "multi-sig script",
			encodedValue: "bitcoin-script:0101006b6376a91415c037003f74c0080d95bb6b1a002b61e32fe43d88ad6c8b6b686376a914dee127ba7d439774733fb155f4db2a6b28b349e888ad6c8b6b686376a914ce361108670c9a1e9b16a4e64fbccc2fd1278ac388ad6c8b6b686376a9146ad74c3d9ea6eb296a71dc8f4bc8c3c3282f5eab88ad6c8b6b686376a9141b59add493d60b1f06a161f3434bac492be6b0bc88ad6c8b6b686376a9142b4746ec32bcafcc869e67959d8aa6c6e533527d88ad6c8b6b68526ca1978c7655",
			prefix:       "bitcoin-script",
			net:          MainNet,
			decodedHex:   "006b6376a91415c037003f74c0080d95bb6b1a002b61e32fe43d88ad6c8b6b686376a914dee127ba7d439774733fb155f4db2a6b28b349e888ad6c8b6b686376a914ce361108670c9a1e9b16a4e64fbccc2fd1278ac388ad6c8b6b686376a9146ad74c3d9ea6eb296a71dc8f4bc8c3c3282f5eab88ad6c8b6b686376a9141b59add493d60b1f06a161f3434bac492be6b0bc88ad6c8b6b686376a9142b4746ec32bcafcc869e67959d8aa6c6e533527d88ad6c8b6b68526ca1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			net, prefix, data, err := BIP0276Decode(tt.encodedValue)
			if err != nil {
				t.Errorf("Failed to decode value : %s", err)
				return
			}

			if net != tt.net {
				t.Errorf("Wrong net value : got %s, want %s", net, tt.net)
				return
			}

			if prefix != tt.prefix {
				t.Errorf("Wrong prefix value : got %s, want %s", prefix, tt.prefix)
				return
			}

			h := hex.EncodeToString(data)
			if h != tt.decodedHex {
				t.Errorf("Wrong decoded value : \n    got  %s\n    want %s", h, tt.decodedHex)
			}
		})
	}
}

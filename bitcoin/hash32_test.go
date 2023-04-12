package bitcoin

import (
	"bytes"
	"testing"
)

func Test_Hash32_String_HardCode(t *testing.T) {
	tests := []struct {
		text string
		hash Hash32
	}{
		{
			text: "84e806b4c902d8ad7696ec89d2a6222872cfaa5fad7ef9d21f6279159a74e775",
			hash: Hash32{0x75, 0xe7, 0x74, 0x9a, 0x15, 0x79, 0x62, 0x1f, 0xd2, 0xf9, 0x7e, 0xad,
				0x5f, 0xaa, 0xcf, 0x72, 0x28, 0x22, 0xa6, 0xd2, 0x89, 0xec, 0x96, 0x76, 0xad, 0xd8,
				0x02, 0xc9, 0xb4, 0x06, 0xe8, 0x84},
		},
		{
			text: "0e88b0b19202b75599bad07b735acc93d5688c2b87859e70b67c7c171d0e1955",
			hash: Hash32{0x55, 0x19, 0x0e, 0x1d, 0x17, 0x7c, 0x7c, 0xb6, 0x70, 0x9e, 0x85, 0x87,
				0x2b, 0x8c, 0x68, 0xd5, 0x93, 0xcc, 0x5a, 0x73, 0x7b, 0xd0, 0xba, 0x99, 0x55, 0xb7,
				0x02, 0x92, 0xb1, 0xb0, 0x88, 0x0e},
		},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			hash, err := NewHash32FromStr(test.text)
			if err != nil {
				t.Fatalf("Failed to convert from string : %s", err)
			}
			t.Logf("Bytes: %x", hash[:])

			if !bytes.Equal(hash[:], test.hash[:]) {
				t.Errorf("Wrong bytes : \n  got  : %x\n  want : %x", hash[:], test.hash[:])
			}

			text := hash.String()
			t.Logf("String: %s", text)
			if text != test.text {
				t.Errorf("Wrong text : \n  got  : %s\n  want : %s", text, test.text)
			}
		})
	}
}

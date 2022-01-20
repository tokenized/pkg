package bitcoin

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"
)

type hexTestStruct struct {
	Value Hex `json:"value"`
}

func Test_Hex_Marshal(t *testing.T) {
	tests := []struct {
		h  string
		js string
	}{
		{
			h:  "619c335025c7f4012e556c2a58b2506e30b8511b53ade95ea316fd8c3286feb9",
			js: `{"value":"619c335025c7f4012e556c2a58b2506e30b8511b53ade95ea316fd8c3286feb9"}`,
		},
		{
			h:  "e30b8511b53ade95ea316fd8c328",
			js: `{"value":"e30b8511b53ade95ea316fd8c328"}`,
		},
		{
			h:  "",
			js: `{"value":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.h, func(t *testing.T) {
			b, err := hex.DecodeString(tt.h)
			if err != nil {
				t.Fatalf("Failed to decode hex : %s", err)
			}

			s := hexTestStruct{
				Value: b,
			}

			js, err := json.Marshal(s)
			if err != nil {
				t.Fatalf("Failed to marshal json : %s", err)
			}

			if string(js) != tt.js {
				t.Errorf("Wrong json : got %s, want %s", string(js), tt.js)
			}

			s2 := hexTestStruct{}
			if err := json.Unmarshal([]byte(tt.js), &s2); err != nil {
				t.Errorf("Failed to unmarshal json : %s", err)
			}

			if !bytes.Equal(b, s2.Value) {
				t.Errorf("Wrong value : got %x, want %x", s2.Value, b)
			}
		})
	}
}

func Test_Hex_ErrMissingQuotes(t *testing.T) {
	js := []byte(`{"value":12}`)
	s := hexTestStruct{}
	if err := json.Unmarshal(js, &s); err != ErrMissingQuotes {
		t.Errorf("Wrong error : got %s, want %s", err, ErrMissingQuotes)
	}
}

func Test_Hex_InvalidHex(t *testing.T) {
	js := []byte(`{"value":123}`)
	s := hexTestStruct{}
	if err := json.Unmarshal(js, &s); err == nil {
		t.Errorf("Did not get error")
	}

	js = []byte(`{"value":123t}`)
	if err := json.Unmarshal(js, &s); err == nil {
		t.Errorf("Did not get error")
	}
}

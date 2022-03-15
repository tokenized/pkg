package merkle_proof

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/tokenized/pkg/json"
)

func TestSerialize(t *testing.T) {
	h := "000cef65a4611570303539143dabd6aa64dbd0f41ed89074406dc0e7cd251cf1efff69f17b44cfe9" +
		"c2a23285168fe05084e1254daa5305311ed8cd95b19ea6b0ed7505008e66d81026ddb2dae0bd8808" +
		"2632790fc6921b299ca798088bef5325a607efb9004d104f378654a25e35dbd6a539505a1e3ddbba" +
		"7f92420414387bb5b12fc1c10f00472581a20a043cee55edee1c65dd6677e09903f22992062d8fd4" +
		"b8d55de7b060006fcc978b3f999a3dbb85a6ae55edc06dd9a30855a030b450206c3646dadbd8c000" +
		"423ab0273c2572880cdc0030034c72ec300ec9dd7bbc7d3f948a9d41b3621e39"

	js := `{
	    "index": 12,
	    "txOrId": "ffeff11c25cde7c06d407490d81ef4d0db64aad6ab3d14393530701561a465ef",
	    "target": "75edb0a69eb195cdd81e310553aa4d25e18450e08f168532a2c2e9cf447bf169",
	    "nodes": [
	      "b9ef07a62553ef8b0898a79c291b92c60f7932260888bde0dab2dd2610d8668e",
	      "0fc1c12fb1b57b38140442927fbadb3d1e5a5039a5d6db355ea25486374f104d",
	      "60b0e75dd5b8d48f2d069229f20399e07766dd651ceeed55ee3c040aa2812547",
	      "c0d8dbda46366c2050b430a05508a3d96dc0ed55aea685bb3d9a993f8b97cc6f",
	      "391e62b3419d8a943f7dbc7bddc90e30ec724c033000dc0c8872253c27b03a42"
	    ]
	}`

	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("Failed to decode hex : %s", err)
	}

	var binaryProof MerkleProof
	if err := binaryProof.Deserialize(bytes.NewReader(b)); err != nil {
		t.Fatalf("Failed to deserialize : %s", err)
	}

	var jsonProof MerkleProof
	if err := json.Unmarshal([]byte(js), &jsonProof); err != nil {
		t.Fatalf("Failed to unmarshal : %s", err)
	}

	var buf bytes.Buffer
	if err := binaryProof.Serialize(&buf); err != nil {
		t.Fatalf("Failed to serialize : %s", err)
	}

	if !bytes.Equal(buf.Bytes(), b) {
		t.Errorf("Wrong serialized bytes : \n got  %x\n want %x", buf.Bytes(), b)
	}

	outJS, err := json.Marshal(jsonProof)
	if err != nil {
		t.Fatalf("Failed to marshal : %s", err)
	}

	t.Logf("Output JSON : %s", string(outJS))

	cleanJS := strings.ReplaceAll(js, " ", "")
	cleanJS = strings.ReplaceAll(cleanJS, "\n", "")
	cleanJS = strings.ReplaceAll(cleanJS, "\t", "")

	t.Logf("Clean JSON : %s", string(cleanJS))

	if cleanJS != string(outJS) {
		t.Errorf("Wrong JSON : \n got  %s\n want %s", string(outJS), cleanJS)
	}

	binaryJS, err := json.Marshal(binaryProof)
	if err != nil {
		t.Fatalf("Failed to marshal : %s", err)
	}

	if cleanJS != string(binaryJS) {
		t.Errorf("Wrong binary JSON : \n got  %s\n want %s", string(binaryJS), cleanJS)
	}

}

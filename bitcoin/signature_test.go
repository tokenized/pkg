package bitcoin

import (
	"bytes"
	"testing"
)

func TestSignatureCompact(t *testing.T) {
	sigCompact := "IChdjWiBBd85xYoJegm4C0Gg/7HIH+XFsfz1xXIPtX+fDXyuF2lykeAcKmsKtJuPnCMbcCgX2olXRsGHjRZtsoM="

	sig, err := SignatureFromCompact(sigCompact)
	if err != nil {
		t.Fatalf("Failed to decode compact signature : %s", err)
	}

	reencode := sig.ToCompact()
	if reencode != sigCompact {
		t.Fatalf("Wrong encoding : \ngot  %s\nwant %s", reencode, sigCompact)
	}
}

func TestSignatureSerialize(t *testing.T) {
	sigCompact := "IChdjWiBBd85xYoJegm4C0Gg/7HIH+XFsfz1xXIPtX+fDXyuF2lykeAcKmsKtJuPnCMbcCgX2olXRsGHjRZtsoM="
	sig, err := SignatureFromCompact(sigCompact)
	if err != nil {
		t.Fatalf("Failed to decode compact signature : %s", err)
	}

	var buf bytes.Buffer
	if err := sig.Serialize(&buf); err != nil {
		t.Fatalf("Failed to serialize signature : %s", err)
	}

	var setSig Signature
	if err := setSig.SetBytes(buf.Bytes()); err != nil {
		t.Fatalf("Failed to set bytes on signature : %s", err)
	}

	var readSig Signature
	if err := readSig.Deserialize(&buf); err != nil {
		t.Fatalf("Failed to deserialize signature : %s", err)
	}

	if !sig.Equal(readSig) {
		t.Fatalf("Signatures don't match")
	}
}

func TestSig(t *testing.T) {
	key, err := KeyFromStr("5Kff9dBYuPTbrkRDavBLf9RU5VLQ2d4XZBcBdGviP7ERNPfosc1")
	// pubKey, err := PublicKeyFromStr("028122ef3567384cbd620b518b8538b7bd15b807778f4995285862e5fe96d2bc32")
	if err != nil {
		t.Fatalf("Bad key : %s", err)
	}
	pubKey := key.PublicKey()

	t.Logf("pubKey : %s", pubKey)

	sigDer := "3045022100c990c00648737339359824f1005e64726e452a27a4962c5c706db05dbefdf521022028872720954fe84fa9761810ea5c2916f43fc0013ec5f7232b4e4e1653c22dc0"
	sig, err := SignatureFromStr(sigDer)
	if err != nil {
		t.Fatalf("Bad signature : %s", err)
	}

	t.Logf("sig : %s", sig)

	sigHash, err := NewHash32FromStr("1ebf70af194358997fe868d341975045cd81c48819c3210a75cda4ccd520f952")
	if err != nil {
		t.Fatalf("Bad hash : %s", err)
	}

	if !sig.Verify(sigHash[:], pubKey) {
		t.Fatalf("Invalid signature")
	}
}

// func TestSigs(t *testing.T) {
// 	count := 20
// 	keys := make([]Key, count)
// 	var err error
// 	for i := range keys {
// 		keys[i], err = GenerateKey(MainNet)
// 		if err != nil {
// 			t.Fatalf("Failed to generate key : %s", err)
// 		}
// 		fmt.Printf("\"%s\",\n", keys[i])
// 	}

// 	hashes := make([]Hash32, count)
// 	for i := range hashes {
// 		rand.Read(hashes[i][:])
// 		fmt.Printf("\"%s\",\n", hashes[i])
// 	}

// 	signatures := make([]Signature, count)
// 	for i := range signatures {
// 		signatures[i], err = keys[i].Sign(hashes[i][:])
// 		if err != nil {
// 			t.Fatalf("Failed to sign hash : %s", err)
// 		}
// 		fmt.Printf("\"%s\",\n", signatures[i])
// 	}
// }

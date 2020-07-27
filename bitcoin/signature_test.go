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

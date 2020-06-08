package bitcoin

import (
	"testing"
)

func TestSignature(t *testing.T) {
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

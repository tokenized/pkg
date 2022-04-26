package threshold

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
)

func TestSerializeMath(t *testing.T) {
	i, err := RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate ordinal : %s", err)
	}

	var ibuf bytes.Buffer
	err = WriteBigInt(i, &ibuf)
	if err != nil {
		t.Fatalf("Failed to write big int : %s", err)
	}

	var j big.Int
	iread := bytes.NewReader(ibuf.Bytes())
	err = ReadBigInt(&j, iread)
	if err != nil {
		t.Fatalf("Failed to read big int : %s", err)
	}

	if i.Cmp(&j) != 0 {
		t.Fatalf("BigInt not equal : \n  got  %s\n  want %s", j.String(), i.String())
	}

	ipair := BigPair{
		X: i,
	}

	ipair.Y, err = RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate ordinal : %s", err)
	}

	var pbuf bytes.Buffer
	err = WriteBigPair(ipair, &pbuf)
	if err != nil {
		t.Fatalf("Failed to write big pair : %s", err)
	}

	var jpair BigPair
	pread := bytes.NewReader(pbuf.Bytes())
	err = ReadBigPair(&jpair, pread)
	if err != nil {
		t.Fatalf("Failed to read big pair : %s", err)
	}

	if !ipair.Equal(jpair) {
		t.Fatalf("BigPair not equal : \n  got  %s\n  want %s", jpair.String(), ipair.String())
	}

	istring := "test1"

	var sbuf bytes.Buffer
	err = WriteString(istring, &sbuf)
	if err != nil {
		t.Fatalf("Failed to write string : %s", err)
	}

	var jstring string
	sread := bytes.NewReader(sbuf.Bytes())
	err = ReadString(&jstring, sread)
	if err != nil {
		t.Fatalf("Failed to read string : %s", err)
	}

	if istring != jstring {
		t.Fatalf("String not equal : \n  got  %s\n  want %s", jstring, istring)
	}
}

func TestSerializeSecretShare(t *testing.T) {
	ordinals := make([]big.Int, 0)
	for i := 0; i < 3; i++ {
		ord, err := RandomOrdinal()
		if err != nil {
			t.Fatalf("Failed to generate ordinal : %s", err)
		}

		ordinals = append(ordinals, ord)
	}

	secrets := make([]*SecretShare, 0, 3)
	shares := make([][]BigPair, 0, 6)
	evalShares := make([][]big.Int, 0, 3)
	for i := 0; i < 3; i++ {
		secret, err := NewSecretShare(432, 1, 1, i, ordinals)
		if err != nil {
			t.Fatalf("Failed to generate secret %d : %s", i, err)
		}
		secrets = append(secrets, secret)

		poly, evals := secrets[i].GetShare()
		shares = append(shares, poly)
		shares = append(shares, evals)

		evalSubShares := make([]big.Int, 0, 3)
		for j := 0; j < 3; j++ {
			if i == j {
				evalSubShares = append(evalSubShares, big.Int{})
				continue
			}
			evalShare := secrets[i].GetEvalShare(j)
			evalSubShares = append(evalSubShares, evalShare)
		}
		evalShares = append(evalShares, evalSubShares)
	}

	for i := 0; i < 3; i++ {
		k := 0
		for j := 0; j < 3; j++ {
			if i == j {
				k += 2
				continue
			}

			secrets[i].AddShare(j, shares[k], shares[k+1])
			secrets[i].AddEvalShare(j, evalShares[j][i])
			k += 2
		}
	}

	values := make([]big.Int, 0, 3)
	publicKeys := make([]*bitcoin.PublicKey, 0, 3)
	for i := 0; i < 3; i++ {
		if !secrets[i].SharesComplete() {
			t.Fatalf("Shares not complete %d", i)
		}

		value, err := secrets[i].CreateSecret(i, ordinals)
		if err != nil {
			t.Fatalf("Failed to create secret %d : %s", i, err)
		}
		values = append(values, value)

		publicKey := secrets[i].CreatePublicKey()
		publicKeys = append(publicKeys, publicKey)
	}

	for i := 1; i < 3; i++ {
		if !publicKeys[0].Equal(*publicKeys[i]) {
			t.Fatalf("Public keys don't match")
		}
	}

	for i := 0; i < 3; i++ {
		var buf bytes.Buffer
		err := secrets[i].Serialize(&buf)
		if err != nil {
			t.Fatalf("Failed to serialize SecretShare %d : %s", i, err)
		}

		read := bytes.NewReader(buf.Bytes())
		var newSecret SecretShare
		err = newSecret.Deserialize(read)
		if err != nil {
			t.Fatalf("Failed to deserialize SecretShare %d : %s", i, err)
		}

		// Compare
		if secrets[i].ID != newSecret.ID {
			t.Fatalf("SecretShare %d ID not equal", i)
		}

		if secrets[i].Type != newSecret.Type {
			t.Fatalf("SecretShare %d Type not equal", i)
		}

		// Evals            []big.Int // Polynomial evaluated for ordinals
		if len(secrets[i].Evals) != len(newSecret.Evals) {
			t.Fatalf("SecretShare %d Evals different length", i)
		}

		for j, eval := range secrets[i].Evals {
			if eval.Cmp(&newSecret.Evals[j]) != 0 {
				t.Fatalf("SecretShare %d Evals[%d] not equal", i, j)
			}
		}

		// HiddenEvals      []BigPair // Polynomial evaluations multiplied by generator point
		if len(secrets[i].HiddenEvals) != len(newSecret.HiddenEvals) {
			t.Fatalf("SecretShare %d HiddenEvals different length", i)
		}

		for j, eval := range secrets[i].HiddenEvals {
			if !eval.Equal(newSecret.HiddenEvals[j]) {
				t.Fatalf("SecretShare %d HiddenEvals[%d] not equal", i, j)
			}
		}

		// HiddenPolynomial []BigPair // Coefficients multiplied by generator point
		if len(secrets[i].HiddenPolynomial) != len(newSecret.HiddenPolynomial) {
			t.Fatalf("SecretShare %d HiddenPolynomial different length", i)
		}

		for j, coeff := range secrets[i].HiddenPolynomial {
			if !coeff.Equal(newSecret.HiddenPolynomial[j]) {
				t.Fatalf("SecretShare %d HiddenPolynomial[%d] not equal", i, j)
			}
		}

		// Shared            []bool
		if len(secrets[i].Shared) != len(newSecret.Shared) {
			t.Fatalf("SecretShare %d Shared different length", i)
		}

		for j, shared := range secrets[i].Shared {
			if shared != newSecret.Shared[j] {
				t.Fatalf("SecretShare %d Shared[%d] not equal", i, j)
			}
		}

		// SharedEvals       [][]BigPair // Hidden evaluations of all ordinals on other party's polynomial
		if len(secrets[i].SharedEvals) != len(newSecret.SharedEvals) {
			t.Fatalf("SecretShare %d SharedEvals different length", i)
		}

		for j, sharedEvals := range secrets[i].SharedEvals {
			if len(sharedEvals) != len(newSecret.SharedEvals[j]) {
				t.Fatalf("SecretShare %d SharedEvals[%d] different length", i, j)
			}

			for k, eval := range sharedEvals {
				if !eval.Equal(newSecret.SharedEvals[j][k]) {
					t.Fatalf("SecretShare %d SharedEvals[%d][%d] not equal", i, j, k)
				}
			}
		}

		// SharedPolynomials [][]BigPair // Hidden coefficients of other party's polynomial
		if len(secrets[i].SharedPolynomials) != len(newSecret.SharedPolynomials) {
			t.Fatalf("SecretShare %d SharedPolynomials different length", i)
		}

		for j, sharedPoly := range secrets[i].SharedPolynomials {
			if len(sharedPoly) != len(newSecret.SharedPolynomials[j]) {
				t.Fatalf("SecretShare %d SharedPolynomials[%d] different length", i, j)
			}

			for k, coeff := range sharedPoly {
				if !coeff.Equal(newSecret.SharedPolynomials[j][k]) {
					t.Fatalf("SecretShare %d SharedPolynomials[%d][%d] not equal", i, j, k)
				}
			}
		}

		// ActualEvalShared []bool
		if len(secrets[i].ActualEvalShared) != len(newSecret.ActualEvalShared) {
			t.Fatalf("SecretShare %d ActualEvalShared different length", i)
		}

		for j, shared := range secrets[i].ActualEvalShared {
			if shared != newSecret.ActualEvalShared[j] {
				t.Fatalf("SecretShare %d ActualEvalShared[%d] not equal", i, j)
			}
		}

		// ActualEvals      []big.Int // Non-hidden evaluation of this party's ordinal on other party's polynomial
		if len(secrets[i].ActualEvals) != len(newSecret.ActualEvals) {
			t.Fatalf("SecretShare %d ActualEvals different length", i)
		}

		for j, eval := range secrets[i].ActualEvals {
			if eval.Cmp(&newSecret.ActualEvals[j]) != 0 {
				t.Fatalf("SecretShare %d ActualEvals[%d] not equal", i, j)
			}
		}
	}
}

func TestSerializeEphemeralKey(t *testing.T) {
	value := EphemeralKey{
		ID:         123,
		Degree:     9,
		IsComplete: true,
		IsUsed:     false,
	}

	var err error
	value.LittleK, err = RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}

	value.Alpha, err = RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}

	value.VWShares = make([][]big.Int, 2)
	for i := 0; i < 2; i++ {
		vw := make([]big.Int, 3)
		for j := 0; j < 3; j++ {
			vw[j], err = RandomOrdinal()
			if err != nil {
				t.Fatalf("Failed to generate random : %s", err)
			}
		}
		value.VWShares[i] = vw
	}

	value.Key, err = RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}

	value.SigHash, err = RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}

	value.SignatureShares = make([]BigPair, 4) // Shares used to construct a signature
	for i := 0; i < 4; i++ {
		ssX, err := RandomOrdinal()
		if err != nil {
			t.Fatalf("Failed to generate random : %s", err)
		}
		ssY, err := RandomOrdinal()
		if err != nil {
			t.Fatalf("Failed to generate random : %s", err)
		}
		value.SignatureShares[i] = BigPair{X: ssX, Y: ssY}
	}

	var buf bytes.Buffer
	err = value.Serialize(&buf)
	if err != nil {
		t.Fatalf("Failed to serialize EphemeralKey : %s", err)
	}

	read := bytes.NewReader(buf.Bytes())
	var newValue EphemeralKey
	err = newValue.Deserialize(read)
	if err != nil {
		t.Fatalf("Failed to deserialize EphemeralKey : %s", err)
	}

	// ID              uint64 // to identify this key
	if value.ID != newValue.ID {
		t.Fatalf("EphemeralKey ID not equal")
	}

	// Degree          int
	if value.Degree != newValue.Degree {
		t.Fatalf("EphemeralKey Degree not equal")
	}

	// LittleK         big.Int     // little k
	if value.LittleK.Cmp(&newValue.LittleK) != 0 {
		t.Fatalf("EphemeralKey LittleK not equal")
	}

	// Alpha           big.Int     // blinding value
	if value.Alpha.Cmp(&newValue.Alpha) != 0 {
		t.Fatalf("EphemeralKey Alpha not equal")
	}

	// VWShares        [][]big.Int // V W shares from all members (for signatures)
	if len(value.VWShares) != len(newValue.VWShares) {
		t.Fatalf("EphemeralKey VWShares length not equal")
	}

	for i, vwShare := range value.VWShares {
		for j, val := range vwShare {
			if val.Cmp(&newValue.VWShares[i][j]) != 0 {
				t.Fatalf("EphemeralKey VWShares[%d][%d] not equal", i, j)
			}
		}
	}

	// IsComplete      bool        // The key has been calculated
	if value.IsComplete != newValue.IsComplete {
		t.Fatalf("EphemeralKey IsComplete not equal")
	}

	// Key             big.Int
	if value.Key.Cmp(&newValue.Key) != 0 {
		t.Fatalf("EphemeralKey Key not equal")
	}

	// IsUsed          bool // A signature share has been given out for this key
	if value.IsUsed != newValue.IsUsed {
		t.Fatalf("EphemeralKey IsUsed not equal")
	}

	// SigHash         big.Int
	if value.SigHash.Cmp(&newValue.SigHash) != 0 {
		t.Fatalf("EphemeralKey SigHash not equal")
	}

	// SignatureShares []BigPair // Shares used to construct a signature
	if len(value.SignatureShares) != len(newValue.SignatureShares) {
		t.Fatalf("EphemeralKey SignatureShares length not equal")
	}

	for i, sigShare := range value.SignatureShares {
		if !sigShare.Equal(newValue.SignatureShares[i]) {
			t.Fatalf("EphemeralKey SignatureShares[%d] not equal", i)
		}
	}
}

func TestSerializeGroup(t *testing.T) {
	value := Group{
		ID:       "test id",
		Ordinals: make(map[string]big.Int), // To accumulate until all are received
		Creator:  "name",
		IsReady:  true,
	}

	randSecret, err := RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}
	value.Secret = randSecret.Bytes()

	randOrd1, err := RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}
	value.Ordinals["test1"] = randOrd1

	randOrd2, err := RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}
	value.Ordinals["test2"] = randOrd2

	randOrd3, err := RandomOrdinal()
	if err != nil {
		t.Fatalf("Failed to generate random : %s", err)
	}
	value.Ordinals["test3"] = randOrd3

	var buf bytes.Buffer
	err = value.Serialize(&buf)
	if err != nil {
		t.Fatalf("Failed to serialize Group : %s", err)
	}

	read := bytes.NewReader(buf.Bytes())
	var newValue Group
	err = newValue.Deserialize(read)
	if err != nil {
		t.Fatalf("Failed to deserialize Group : %s", err)
	}

	// ID       string
	if value.ID != newValue.ID {
		t.Fatalf("Group ID not equal")
	}

	// Ordinals map[string]big.Int // To accumulate until all are received
	if len(value.Ordinals) != len(newValue.Ordinals) {
		t.Fatalf("Group Ordinals length not equal")
	}

	for name, ordinal := range value.Ordinals {
		val, exists := newValue.Ordinals[name]
		if !exists {
			t.Fatalf("Group Ordinal[%s] missing", name)
		}
		if ordinal.Cmp(&val) != 0 {
			t.Fatalf("Group Ordinals[%s] not equal", name)
		}
	}

	// Creator  string
	if value.Creator != newValue.Creator {
		t.Fatalf("Group Creator not equal")
	}

	// Secret   []byte // For encrypting messages to the group
	if !bytes.Equal(value.Secret, newValue.Secret) {
		t.Fatalf("Group Secret not equal")
	}

	// IsReady  bool
	if value.IsReady != newValue.IsReady {
		t.Fatalf("Group IsReady not equal")
	}
}

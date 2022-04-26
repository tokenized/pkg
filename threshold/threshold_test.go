package threshold

import (
	"math/big"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
)

func TestShareProcessRandom(t *testing.T) {
	// Calculate values
	memberCount := 3
	signingThreshold := 3
	degree := (signingThreshold - 1) / 2

	// *********************************************************************************************
	// Group Setup
	// *********************************************************************************************

	// Create ordinals
	var err error
	ordinals := make([]big.Int, memberCount)
	for i := 0; i < memberCount; i++ {
		ordinals[i], err = RandomOrdinal()
		if err != nil {
			t.Fatalf("Failed to generate random ordinal %d : %s", i, err)
		}
	}

	// Create members
	members := make([]Member, memberCount)
	for i := 0; i < memberCount; i++ {
		members[i], err = NewMember(ordinals[i], ordinals, degree)
		if err != nil {
			t.Fatalf("Failed to create member %d : %s", i, err)
		}
	}

	// Private Key Shares
	for i, _ := range members {
		iSecret := members[i].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
		if iSecret == nil {
			continue // Probably complete
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
			if jSecret == nil {
				continue // Probably complete
			}

			poly, evals := jSecret.GetShare()
			iSecret.AddShare(jMember.OrdinalIndex, poly, evals)

			evalShare := jSecret.GetEvalShare(i)
			iSecret.AddEvalShare(jMember.OrdinalIndex, evalShare)
		}
	}

	for i, _ := range members {
		iSecret := members[i].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
		if iSecret == nil {
			continue // Probably complete
		}

		if !iSecret.SharesComplete() {
			t.Fatalf("Private secret share is not complete")
		}

		err = members[i].FinishSecretShare(iSecret)
		if err != nil {
			t.Fatalf("Failed to finish share %d : %s", i, err)
		}

		t.Logf("Private Key Share %d : %s", i, members[i].PrivateKeyShare.String())
		t.Logf("Public Key %d : %s", i, members[i].PublicKey.String())
	}

	// *********************************************************************************************
	// Ephemeral Key Generation
	// *********************************************************************************************

	var id uint64
	for i, _ := range members {
		secretShares, err := members[i].StartEphemeralKey()
		if err != nil {
			t.Fatalf("Failed to start ephemeral key %d : %s", i, err)
		}
		id = secretShares[0].ID
	}

	// LittleK Shares
	for i, _ := range members {
		iSecret := members[i].GetSecretShare(id, LittleK)
		if iSecret == nil {
			continue // Probably complete
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(id, LittleK)
			if jSecret == nil {
				continue // Probably complete
			}

			poly, evals := jSecret.GetShare()
			iSecret.AddShare(jMember.OrdinalIndex, poly, evals)

			evalShare := jSecret.GetEvalShare(i)
			iSecret.AddEvalShare(jMember.OrdinalIndex, evalShare)
		}
	}

	// Alpha Shares
	for i, _ := range members {
		iSecret := members[i].GetSecretShare(id, Alpha)
		if iSecret == nil {
			continue // Probably complete
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(id, Alpha)
			if jSecret == nil {
				continue // Probably complete
			}

			poly, evals := jSecret.GetShare()
			iSecret.AddShare(jMember.OrdinalIndex, poly, evals)

			evalShare := jSecret.GetEvalShare(i)
			iSecret.AddEvalShare(jMember.OrdinalIndex, evalShare)
		}
	}

	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			t.Fatalf("Failed to find ephemeral key %d", id)
		}

		secret := members[i].GetSecretShare(id, LittleK)
		if secret == nil {
			t.Fatalf("Failed to find littlek secret share")
		}

		if !secret.SharesComplete() {
			t.Fatalf("LittleK secret share is not complete")
		}

		err = members[i].FinishSecretShare(secret)
		if err != nil {
			t.Fatalf("Failed to finish littlek secret share %d : %s", i, err)
		}

		secret = members[i].GetSecretShare(id, Alpha)
		if secret == nil {
			t.Fatalf("Failed to find alpha secret share")
		}

		if !secret.SharesComplete() {
			t.Fatalf("Alpha secret share is not complete")
		}

		err = members[i].FinishSecretShare(secret)
		if err != nil {
			t.Fatalf("Failed to finish alpha secret share %d : %s", i, err)
		}
	}

	// VW Shares
	for i, _ := range members {
		iKey := members[i].FindEphemeralKey(id)
		if iKey == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		vw := iKey.GetVWShare(members[i].Ordinal())

		for j, _ := range members {
			if i == j {
				continue
			}

			jKey := members[j].FindEphemeralKey(id)
			if jKey == nil {
				t.Fatalf("Failed to find ephemeral key %d %d", j, id)
			}

			jKey.AddVWShare(vw)
		}
	}

	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		if err := key.CalculateKey(ordinals); err != nil {
			t.Fatalf("Failed to calculate ephemeral key %d : %s", i, err)
		}

		t.Logf("LittleK %d : %s", i, key.LittleK.String())
		t.Logf("Alpha %d : %s", i, key.Alpha.String())
		t.Logf("Ephemeral Key %d : %s", i, key.Key.String())
	}

	// *********************************************************************************************
	// Signing
	// *********************************************************************************************

	// Signature Hash
	sigHash, err := RandomOrdinal() // Generate random big int to use as sig hash.
	if err != nil {
		t.Fatalf("Failed to generate random sig hash : %s", err)
	}

	t.Logf("Signature Hash : %s", sigHash.String())

	key := members[0].FindUnusedEphemeralKey()
	if key == nil {
		t.Fatalf("Failed to find unused ephemeral key")
	}

	// Collect Signature Data from first "signingThreshold" members.
	for i, _ := range members[:signingThreshold] {
		iKey := members[i].FindEphemeralKey(id)
		if iKey == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		sigShare, err := iKey.GetSignatureShare(members[i].Ordinal(),
			members[i].PrivateKeyShare, sigHash)
		if err != nil {
			t.Fatalf("Failed to get signature share %d : %s", i, err)
		}

		t.Logf("Signature share %d : %s", i, sigShare.String())

		for j, _ := range members {
			if i == j {
				continue
			}

			jKey := members[j].FindEphemeralKey(id)
			if jKey == nil {
				t.Fatalf("Failed to find ephemeral key %d %d", j, id)
			}

			if err := jKey.AddSignatureShare(sigHash, members[i].Ordinal(), sigShare); err != nil {
				t.Fatalf("Failed to add signature share %d to %d : %s", i, j, err)
			}
		}
	}

	// Create Signature
	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		sig, err := key.CreateSignature(sigHash, *members[i].PublicKey)
		if err != nil {
			t.Fatalf("Failed to create signature %d : %s", i, err)
		}

		t.Logf("Signature R %d : %s", i, sig.R.String())
		t.Logf("Signature S %d : %s", i, sig.S.String())
		t.Logf("Signature %d : %s", i, sig.String())

		hash, err := bitcoin.NewHash32(sigHash.Bytes())
		if err != nil {
			t.Fatalf("Failed to create hash : %s", err)
		}

		if !sig.Verify(*hash, *members[i].PublicKey) {
			t.Fatalf("Signature Invalid")
		}
	}
}

func TestShareProcessFixed(t *testing.T) {

	var err error
	degree := 1 // t
	// keyThreshold := degree + 1 // t + 1
	signingThreshold := (2 * degree) + 1 // = 2t + 1

	// *********************************************************************************************
	// Group Setup
	// *********************************************************************************************

	// Ordinals
	ordinals := make([]big.Int, 4)
	ordinals[0].SetString("42237370567629114626362799506549046146555615292347125328546981847563710986648", 10)
	ordinals[1].SetString("101555145903449373395127190943202572984907076005555351482125433151620352233241", 10)
	ordinals[2].SetString("6872919929692078669980834183826411235739740157393246621629789477854793451742", 10)
	ordinals[3].SetString("108914315810233723849408491570118647332077253061471676278456424552570090875457", 10)

	for i, ord := range ordinals {
		t.Logf("Member %d Ordinal : %s", i, ord.String())
	}

	// Members
	members := make([]Member, 4)
	for i, ord := range ordinals {
		members[i], err = NewMember(ord, ordinals, degree)
		if err != nil {
			t.Fatalf("Failed to create member %d : %s", i, err)
		}
		t.Logf("Member %d Ordinal Index : %d", i, members[i].OrdinalIndex)
	}

	poly := Polynomial{
		Coefficients: make([]big.Int, degree+1),
	}

	secret := members[0].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
	poly.Coefficients[0].SetString("28196039294460439574788498489768470454724646610151765079577284619165749225148", 10)
	poly.Coefficients[1].SetString("115367808483568177490296983667302060884798066444852446307690977903582922150450", 10)
	secret.ManualSetup(poly, members[0].OrdinalIndex, members[0].Ordinals)

	secret = members[1].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
	poly.Coefficients[0].SetString("21553219618872199689238017930595852852490774776218662816333226967840310058125", 10)
	poly.Coefficients[1].SetString("38699431172060512958590374067855409848798831874389459933923435815189231340913", 10)
	secret.ManualSetup(poly, members[1].OrdinalIndex, members[1].Ordinals)

	secret = members[2].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
	poly.Coefficients[0].SetString("23366092712362790337415529029875290170864677687182420606969196855467532723927", 10)
	poly.Coefficients[1].SetString("77806634044654090293245939004751668457211983083887899755722884958619274085770", 10)
	secret.ManualSetup(poly, members[2].OrdinalIndex, members[2].Ordinals)

	secret = members[3].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
	poly.Coefficients[0].SetString("49377962248683436976844801054543328681628422334577173522174084768533348924864", 10)
	poly.Coefficients[1].SetString("63784732412042280971893506542534396423021838002078033516086289492112856698651", 10)
	secret.ManualSetup(poly, members[3].OrdinalIndex, members[3].Ordinals)

	for i, member := range members {
		t.Logf("Member %d", i)
		secret = members[i].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)

		t.Logf("  Ordinal Index : %d", member.OrdinalIndex)
		// t.Logf("  Polynomial")
		// for j, coef := range member.PrivatePolynomial.Coefficients {
		// 	t.Logf("    Coef %d : %s", j, coef.String())
		// }
		t.Logf("  Hidden Polynomial")
		for j, point := range secret.HiddenPolynomial {
			t.Logf("    Point %d : %s", j, point.String())
		}
	}

	// Shares
	for i, _ := range members {
		iSecret := members[i].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
		if iSecret == nil {
			continue // Probably complete
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
			if jSecret == nil {
				continue // Probably complete
			}

			poly, evals := jSecret.GetShare()
			iSecret.AddShare(jMember.OrdinalIndex, poly, evals)

			evalShare := jSecret.GetEvalShare(i)
			iSecret.AddEvalShare(jMember.OrdinalIndex, evalShare)
		}
	}

	for i, _ := range members {
		iSecret := members[i].GetSecretShare(PrivateKeyShareID, PrivateKeyShare)
		if iSecret == nil {
			continue // Probably complete
		}

		if !iSecret.SharesComplete() {
			t.Fatalf("Private secret share is not complete")
		}

		err = members[i].FinishSecretShare(iSecret)
		if err != nil {
			t.Fatalf("Failed to finish private secret share %d : %s", i, err)
		}

		t.Logf("Private Key Share %d : %s", i, members[i].PrivateKeyShare.String())
		t.Logf("Public Key %d : %s", i, members[i].PublicKey.String())
	}

	for _, member := range members[1:] {
		if !members[0].PublicKey.Equal(*member.PublicKey) {
			t.Fatalf("Public keys don't match : \n%s \n!= \n%s", members[0].PublicKey.String(),
				member.PublicKey.String())
		}
	}

	// *********************************************************************************************
	// Full Private Key Generation
	// *********************************************************************************************

	// Generate full private key using t + 1 private key shares
	for i, _ := range []int{1, 3} {
		secretShare := members[i].GetPrivateKeyShare()

		for j, _ := range members {
			if i == j {
				continue
			}

			if err := members[j].AddPrivateKeyShare(ordinals[i], secretShare); err != nil {
				t.Fatalf("Failed to add private key share %d to %d : %s", i, j, err)
			}
		}
	}

	// Check full private key generation
	for i, member := range members {
		key, err := member.GeneratePrivateKey(bitcoin.MainNet)
		if err != nil {
			t.Fatalf("Failed to generate private key %d : %s", i, err)
		}
		t.Logf("Full Private Key %d : %s", i, key.String())

		pubkey := key.PublicKey()
		if !pubkey.Equal(*member.PublicKey) {
			t.Fatalf("Private key's public key does not match %d : \n  got %s\n  want %s",
				i, pubkey.String(), member.PublicKey.String())
		}
	}

	// *********************************************************************************************
	// Ephemeral Key Generation
	// *********************************************************************************************

	var id uint64
	for i, _ := range members {
		secretShares, err := members[i].StartEphemeralKey()
		if err != nil {
			t.Fatalf("Failed to start ephemeral key %d : %s", i, err)
		}
		id = secretShares[0].ID
	}

	// Manually setup polynomials
	// Member 0
	littleK := members[0].GetSecretShare(id, LittleK)
	if littleK == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("10621720366847886261725677885360667148834045476789747167052058225183181725290", 10)
	poly.Coefficients[1].SetString("16718658108760292219253325634486431341618214746263808516879441297939198891465", 10)

	err = littleK.ManualSetup(poly, members[0].OrdinalIndex, members[0].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key little k %d : %s", 0, err)
	}

	alpha := members[0].GetSecretShare(id, Alpha)
	if alpha == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("2804098739123503233814047245685961332981429981646380323602423988591855377522", 10)
	poly.Coefficients[1].SetString("55731064280476568723011187500774403485796786177036395431413673437412630023446", 10)

	err = alpha.ManualSetup(poly, members[0].OrdinalIndex, members[0].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key alpha %d : %s", 0, err)
	}

	// Member 1
	littleK = members[1].GetSecretShare(id, LittleK)
	if littleK == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("30451836532661185218060663792515836885768512827060484664280593425890427162895", 10)
	poly.Coefficients[1].SetString("72101565115092594902249759961462016725931843678998566934190926420545820699909", 10)

	err = littleK.ManualSetup(poly, members[1].OrdinalIndex, members[1].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key little k %d : %s", 1, err)
	}

	alpha = members[1].GetSecretShare(id, Alpha)
	if alpha == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[1].SetString("64991479286540809873205112957121385137228919088694731813601081618003964143895", 10)
	poly.Coefficients[1].SetString("102190233642089843597073594283723866569556336685318674534500292277610202829462", 10)

	err = alpha.ManualSetup(poly, members[1].OrdinalIndex, members[1].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key alpha %d : %s", 1, err)
	}

	// Member 2
	littleK = members[2].GetSecretShare(id, LittleK)
	if littleK == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("42028780986229472765742951407662143787058244404351992973526225797244010701727", 10)
	poly.Coefficients[1].SetString("41017931708864390467578392423580678558433017667763889522747447619326447094207", 10)

	err = littleK.ManualSetup(poly, members[2].OrdinalIndex, members[2].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key little k %d : %s", 2, err)
	}

	alpha = members[2].GetSecretShare(id, Alpha)
	if alpha == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("9330632440698736248754059991307985944098181954465016277507288881790668106186", 10)
	poly.Coefficients[1].SetString("113107765084476964735368238145721831576302579226789690880043591726080060520298", 10)

	err = alpha.ManualSetup(poly, members[2].OrdinalIndex, members[2].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key alpha %d : %s", 2, err)
	}

	// Member 3
	littleK = members[3].GetSecretShare(id, LittleK)
	if littleK == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("35301108025830905741706072775173229452573474872909148357612735117813461133744", 10)
	poly.Coefficients[1].SetString("33052612711314268319692273931153545926897606320496056915648293915742450988511", 10)

	err = littleK.ManualSetup(poly, members[3].OrdinalIndex, members[3].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key little k %d : %s", 3, err)
	}

	alpha = members[3].GetSecretShare(id, Alpha)
	if alpha == nil {
		t.Fatalf("littlek secret share not found")
	}
	poly.Coefficients[0].SetString("105920431759951359438009727623218303592809123100338002829669287005207410532512", 10)
	poly.Coefficients[1].SetString("54493318992387954495231644734300031941739938279052173448119112368197349953766", 10)

	err = alpha.ManualSetup(poly, members[3].OrdinalIndex, members[3].Ordinals)
	if err != nil {
		t.Fatalf("Failed to manually setup ephemeral key alpha %d : %s", 3, err)
	}

	// LittleK Shares
	for i, _ := range members {
		iSecret := members[i].GetSecretShare(id, LittleK)
		if iSecret == nil {
			continue // Probably complete
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(id, LittleK)
			if jSecret == nil {
				continue // Probably complete
			}

			poly, evals := jSecret.GetShare()
			iSecret.AddShare(jMember.OrdinalIndex, poly, evals)

			evalShare := jSecret.GetEvalShare(i)
			iSecret.AddEvalShare(jMember.OrdinalIndex, evalShare)
		}
	}

	// Alpha Shares
	for i, _ := range members {
		iSecret := members[i].GetSecretShare(id, Alpha)
		if iSecret == nil {
			continue // Probably complete
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(id, Alpha)
			if jSecret == nil {
				continue // Probably complete
			}

			poly, evals := jSecret.GetShare()
			iSecret.AddShare(jMember.OrdinalIndex, poly, evals)

			evalShare := jSecret.GetEvalShare(i)
			iSecret.AddEvalShare(jMember.OrdinalIndex, evalShare)
		}
	}

	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			t.Fatalf("Failed to find ephemeral key %d", id)
		}

		secret := members[i].GetSecretShare(id, LittleK)
		if secret == nil {
			t.Fatalf("Failed to find littlek secret share")
		}

		if !secret.SharesComplete() {
			t.Fatalf("Littlek secret share is not complete")
		}

		err = members[i].FinishSecretShare(secret)
		if err != nil {
			t.Fatalf("Failed to finish littlek secret share %d : %s", i, err)
		}

		secret = members[i].GetSecretShare(id, Alpha)
		if secret == nil {
			t.Fatalf("Failed to find alpha secret share")
		}

		if !secret.SharesComplete() {
			t.Fatalf("Alpha secret share is not complete")
		}

		err = members[i].FinishSecretShare(secret)
		if err != nil {
			t.Fatalf("Failed to finish alpha secret share %d : %s", i, err)
		}
	}

	// VW Shares
	for i, _ := range members {
		iKey := members[i].FindEphemeralKey(id)
		if iKey == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		vw := iKey.GetVWShare(members[i].Ordinal())

		for j, _ := range members {
			if i == j {
				continue
			}

			jKey := members[j].FindEphemeralKey(id)
			if jKey == nil {
				t.Fatalf("Failed to find ephemeral key %d %d", j, id)
			}

			jKey.AddVWShare(vw)
		}
	}

	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		if err := key.CalculateKey(ordinals); err != nil {
			t.Fatalf("Failed to calculate ephemeral key %d : %s", i, err)
		}

		t.Logf("LittleK %d : %s", i, key.LittleK.String())
		t.Logf("Alpha %d : %s", i, key.Alpha.String())
		t.Logf("Ephemeral Key %d : %s", i, key.Key.String())
	}

	// Signature Hash
	var sigHash big.Int
	sigHash.SetString("33158541553975438394626036062115727724998868907128174559200076154031217289129", 10)

	t.Logf("Signature Hash : %s", sigHash.String())

	key := members[0].FindUnusedEphemeralKey()
	if key == nil {
		t.Fatalf("Failed to find unused ephemeral key")
	}

	// Collect Signature Data from first "signingThreshold" members.
	for i, _ := range members[:signingThreshold] {
		iKey := members[i].FindEphemeralKey(id)
		if iKey == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		sigShare, err := iKey.GetSignatureShare(members[i].Ordinal(),
			members[i].PrivateKeyShare, sigHash)
		if err != nil {
			t.Fatalf("Failed to get signature share %d : %s", i, err)
		}

		t.Logf("Signature share %d : %s", i, sigShare.String())

		for j, _ := range members {
			if i == j {
				continue
			}

			jKey := members[j].FindEphemeralKey(id)
			if jKey == nil {
				t.Fatalf("Failed to find ephemeral key %d %d", j, id)
			}

			if err := jKey.AddSignatureShare(sigHash, members[i].Ordinal(), sigShare); err != nil {
				t.Fatalf("Failed to add signature share %d to %d : %s", i, j, err)
			}
		}
	}

	// Create Signature
	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			t.Fatalf("Failed to find ephemeral key %d %d", i, id)
		}

		sig, err := key.CreateSignature(sigHash, *members[i].PublicKey)
		if err != nil {
			t.Fatalf("Failed to create signature %d : %s", i, err)
		}

		t.Logf("Signature R %d : %s", i, sig.R.String())
		t.Logf("Signature S %d : %s", i, sig.S.String())
		t.Logf("Signature %d : %s", i, sig.String())

		hash, err := bitcoin.NewHash32(sigHash.Bytes())
		if err != nil {
			t.Fatalf("Failed to create hash : %s", err)
		}

		if !sig.Verify(*hash, *members[i].PublicKey) {
			t.Fatalf("Signature Invalid")
		}
	}
}

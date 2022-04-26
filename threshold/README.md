
This package supports creating shared secrets within a group and using those to create signatures and reconstruct private keys. It uses a method called Joint Verifiable Random Secret Sharing (JVRSS). It is ported from nChain's Nakasendo library under the NCHAIN OPEN BITCOIN SV LICENSE.

## Definitions

Member
: An individual that is part of a group that is creating a shared secret.

Group
: A set of members that are creating a shared secret.

Hidden
: Specifies the numbers are multiplied by the EC generator (base) point. (just like with a private key to public key)

Shared Polynomial Data
: The data shared between members about their polynomial to allow creation of shared secrets.
Note: (the last two are the same to all members)
   - evaluation of the other member's ordinal on this member's private polynomial (not hidden, sent only to owner of ordinal)
   - hidden polynomial coefficients (sent to all members)
   - hidden evaluation of every member's ordinal on private polynomial (sent to all members)

Ephemeral Key
: A generated key used to create a signature. It is generated from 2 rounds of private polynomial generation and data sharing, followed by vw data sharing. It does not need to be retained after the signature is created.

## Calculations

The degree of the private polynomials for each member is represented by **t**. **t** must be 1 or greater `t >= 1`. Note: A polynomial has `t + 1` coefficients.

The required number of members to create a valid signature is `m >= 2t + 1`.

The required number of members to reconstruct a private key is `m >= t + 1`.

With the private key, signatures can be generated independently. So effectively `t + 1` if members are compromised, then the group is compromised, because they can collude to generate the private key and create signatures independent of the remaining group members.

All elliptic point calculations within this package use the secp256k1 curve.

All big number arithmetic within this package uses modular arithmetic with the modulo `FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141`

## Processes

### Generate Private Key Share

1. Any member collects/generates an ordinal for each member in the group and initiates the group with all the members by notifying them of all the ordinals.
2. Each member generates a random private polynomial and shares their data about that polynomial with all other members. The unhidden evaluation of the other member's ordinal on the polynomial is only shared with the other member.
3. Each member generates a private key share and public key from this shared data.

### Generate Ephemeral Keys

For each ephemeral key (littlek/Alpha pair), one signature can be created. They should not be reused.

For each ephemeral key desired the following steps are required.

1. Each member generates a new random private polynomial and shares their data about it.
2. Each member generates a littlek value from that round of data.
3. Each member generates another new random private polynomial and shares their data about it.
4. Each member generates an alpha value from that round of data.
5. Each member calculates a vw share from their littlk and alpha values and shares that with the group.
6. With at least `2t + 1` vw shares each member generates an ephemeral key from the vw shares.

### Create Signature

1. Any member initiates by sending a message to be signed to all members along with information about which ephemeral key to use.
2. Each member that approves of the message generates the signature share and provides it to the other members.
3. Any member with at least `2t + 1` signature shares can produce a valid signature.

### Full Private Key Generation

1. Any member collects `t` private key shares from other members.
2. From this plus the member's own private key share, totaling `t + 1` private key shares, the full private key can be calculated.

Note: This effectively ends the threshold system because then that member can generate signatures by themselves.


## Usage

Decide the number of members in the group `n` and the signing threshold desired `m`. The signing threshold must be odd because it is `2t + 1`. This means "2 of 3" systems are not possible. There must also be at least 3 members in the group because `t >= 1`, so the minimum signing threshold is 3.

Then calculate the polynomial degree you need with the equation `(m - 1) / 2 = t`. So for a signing threshold of 3 use a polynomial degree of 1 because `(3 - 1) / 2 = 1`.

	memberCount := 3
	signingThreshold := 3
	degree := (signingThreshold - 1) / 2

### Group Setup

Create a set of ordinals. One for each member of the group.

	var err error
	ordinals := make([]big.Int, memberCount)
	for i := 0; i < memberCount; i++ {
		ordinals[i], err = RandomOrdinal()
		if err != nil {
			return errors.Wrap(err, "create ordinal")
		}
	}

Create members of the group.

	members := make([]Member, memberCount)
	for i := 0; i < memberCount; i++ {
		members[i], err = NewMember(ordinals[i], ordinals, degree)
		if err != nil {
			return errors.Wrap(err, "create member")
		}
	}

### Private Key Share Generation

The first shared secret that must be generated is the private key share. It is automatically started when the member is created. But members must give each other data to be able to calculate the shares.

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

			share, err := jSecret.GetShare(jMember.Ordinal(), ordinals)
			if err != nil {
				return errors.Wrap(err, "generate private key share data")
			}

			if err := iSecret.AddShare(share, ordinals); err != nil {
				return errors.Wrap(err, "add private key share data")
			}

			evalShare, err := jSecret.GetEvalShare(jMember.Ordinal(), ordinals[i], ordinals)
			if err != nil {
				return errors.Wrap(err, "generate private key eval share data")
			}

			if err := iSecret.AddEvalShare(evalShare[0], evalShare[1], ordinals); err != nil {
				return errors.Wrap(err, "add private key eval share data")
			}

			if iSecret.SharesComplete() {
				members[i].PrivateKeyShare, err = iSecret.CreateSecret(members[i].OrdinalIndex, ordinals)
				if err != nil {
					return errors.Wrap(err, "create private key share")
				}

				members[i].PublicKey = iSecret.CreatePublicKey()
				if err != nil {
					return errors.Wrap(err, "create public key")
				}
			}
		}
	}


### Ephemeral Key Generation

After a private key share is established a bunch of ephemeral keys will need to be generated to be used for creating signatures.

First the process is started by creating a UUID for the key, then initializing the key by calling `StartEphemeralKey`.

	var id uint64
	for i, _ := range members {
		secretShares, err := members[i].StartEphemeralKey()
		if err != nil {
			return errors.Wrap(err, "start ephemeral key")
		}
		id = secretShares[0].ID
	}

That creates two secret shares, littlek and alpha, that must be generated by sharing data between members.

For littlek:

	for i, _ := range members {
		iSecret := members[i].GetSecretShare(id, LittleK)
		if iSecret == nil {
			return errors.New("Secret share not found")
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(id, LittleK)
			if jSecret == nil {
				return errors.New("Secret share not found")
			}

			share, err := jSecret.GetShare(jMember.Ordinal(), ordinals)
			if err != nil {
				return errors.Wrap(err, "get share")
			}

			if err := iSecret.AddShare(share, ordinals); err != nil {
				return errors.Wrap(err, "add share")
			}

			evalShare, err := jSecret.GetEvalShare(jMember.Ordinal(), ordinals[i], ordinals)
			if err != nil {
				return errors.Wrap(err, "get eval share")
			}

			if err := iSecret.AddEvalShare(evalShare[0], evalShare[1], ordinals); err != nil {
				return errors.Wrap(err, "add eval share")
			}
		}
	}

For alpha:

	for i, _ := range members {
		iSecret := members[i].GetSecretShare(id, Alpha)
		if iSecret == nil {
			return errors.New("Secret share not found")
		}

		for j, jMember := range members {
			if i == j {
				continue
			}

			jSecret := members[j].GetSecretShare(id, Alpha)
			if jSecret == nil {
				return errors.New("Secret share not found")
			}

			share, err := jSecret.GetShare(jMember.Ordinal(), ordinals[i], ordinals)
			if err != nil {
				return errors.Wrap(err, "get share")
			}

			if err := iSecret.AddShare(share, ordinals); err != nil {
				return errors.Wrap(err, "add share")
			}

			evalShare, err := jSecret.GetEvalShare(jMember.Ordinal(), ordinals[i], ordinals)
			if err != nil {
				return errors.Wrap(err, "get eval share")
			}

			if err := iSecret.AddEvalShare(evalShare[0], evalShare[1], ordinals); err != nil {
				return errors.Wrap(err, "add eval share")
			}
		}
	}

Then the shared secrets littlek and alpha can be generated and added to the ephemeral key like this.

	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			return errors.New("Ephemeral key not found")
		}

		secret := members[i].GetSecretShare(id, LittleK)
		if secret == nil {
			return errors.New("Secret share not found")
		}

		key.LittleK, err = secret.CreateSecret(members[i].OrdinalIndex, ordinals)
		if err != nil {
			return errors.Wrap(err, "create littlek")
		}

		secret = members[i].GetSecretShare(id, Alpha)
		if secret == nil {
			return errors.New("Secret share not found")
		}

		key.Alpha, err = secret.CreateSecret(members[i].OrdinalIndex, ordinals)
		if err != nil {
			return errors.Wrap(err, "create alpha")
		}

		members[i].RemoveSecretShare(id, LittleK)
		members[i].RemoveSecretShare(id, Alpha)
	}

Then to generate the actual ephemeral key values the members must share VW data.

	for i, _ := range members {
		iKey := members[i].FindEphemeralKey(id)
		if iKey == nil {
			return errors.New("Ephemeral key not found")
		}

		vw := iKey.GetVWShare(members[i].Ordinal())

		for j, _ := range members {
			if i == j {
				continue
			}

			jKey := members[j].FindEphemeralKey(id)
			if jKey == nil {
				return errors.New("Ephemeral key not found")
			}

			jKey.AddVWShare(vw)
		}
	}

Then, finally, the ephemeral key can be calculated.

	for i, _ := range members {
		key := members[i].FindEphemeralKey(id)
		if key == nil {
			return errors.New("Ephemeral key not found")
		}

		if err := key.CalculateKey(ordinals); err != nil {
			return errors.Wrap(err, "calculate ephemeral")
		}
	}

This can be done many times to preload the group with ephemeral keys for signature generation. Each ephemeral key can only be used to create one signature. Otherwise it will reveal information about the private key.

### Signing

After there are available ephemeral keys, signatures can be created. The message should be shared with all members, and they should generate their own signature hash to ensure they are signing the correct data. For this example we will just use a random signature hash.

	sigHash, err := RandomOrdinal() // Generate random big int to use as sig hash.
	if err != nil {
		return errors.Wrap(err, "generate random sig hash")
	}

The initiating member must find an available ephemeral key.

	key := members[0].FindUnusedEphemeralKey()
	if key == nil {
		return errors.New("Available ephemeral key not found")
	}

Then signature shares must be collected from members who agree and want to sign the message.

	for i, _ := range members[:signingThreshold] {
		iKey := members[i].FindEphemeralKey(id)
		if iKey == nil {
			return errors.New("Ephemeral key not found")
		}

		sigShare, err := iKey.GetSignatureShare(members[i].Ordinal(),
			members[i].PrivateKeyShare, sigHash)
		if err != nil {
			return errors.Wrap(err, "get sig share")
		}

		for j, _ := range members {
			if i == j {
				continue
			}

			jKey := members[j].FindEphemeralKey(id)
			if jKey == nil {
				return errors.New("Ephemeral key not found")
			}

			if err := jKey.AddSignatureShare(sigHash, sigShare); err != nil {
				return errors.Wrap(err, "add sig share")
			}
		}
	}

When a member has at least the threshold of signature shares, they can create a valid signature.

	sig, err := key.CreateSignature(sigHash, members[0].PublicKey)
	if err != nil {
		return errors.Wrap(err, "create signature")
	}

	if !sig.Verify(sigHash.Bytes(), members[0].PublicKey) {
		return errors.New("Invalid signature")
	}

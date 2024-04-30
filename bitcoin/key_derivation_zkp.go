package bitcoin

import (
	"crypto/sha256"
	"fmt"

	"github.com/pkg/errors"
)

// Regarding identity:

// I think we need to define our goals. This is my best current summary.

// * Declare a public identity on chain that is basically just a key and maybe randomized identifier.
// * Prove private information related to an identity via oracle certificates.
// * Prove keys (locking scripts) belong to an identity without an oracle.
//     * When the owner is involved they can simply sign a message with a known key saying the other keys belong to them.
//     * Don't reveal any derivation information about the keys. If the derivation is revealed in the proof then other keys from the owner can be derived by other parties and that is a privacy issue.
// * Allow a third party without private keys, but with private information, to prove keys (locking scripts) belong to an identity.
//     * Don't reveal any derivation information about the keys. If the derivation is revealed in the proof then other keys from the owner can be derived by other parties and that is a privacy issue.
//     * Don't allow the third party to be able to generate keys that may not be known to the owner. For example, don't allow them to generate and prove for randomized keys (owner's private key plus random value) where the random value could be withheld from the owner. This requires predefined derivation that is not revealed, but is proven.
// * Prove the other direction. Like for non-transferable assets. Anyone could sign saying they own the sender keys and the receiver keys. When they are signing to receive instruments to those keys they have no reason to be dishonest, but when the ownership is tied to being given privileges, like moving tokens between keys, we need some outside proof that they actually do. Like a record of an entity change that was signed by an oracle or just an oracle certificate directly saying both keys belong to the same identity.

// Showing multisig because it’s a little more involved, but not by much.
// The entity administered by Alice, Bob, and Charlie agree on a shared secret extended key pair Sx and sign a credential linking their 3 keys A,B,C to Sx along with the control arrangement (2 of 3). Optionally, a paymail host can sign a credential that affirms this along with the paymail handle. These would be the self-signed and 3rd-party signed identity credentials.
// The private key for Sx would be securely shared or cooperatively generated with the paymail host so that it can trustlessly derive locking scripts and proofs without active interaction from Alice, Bob, or Charlie. From Sx, un-hardened child private keys mi can be used to generate masked single-use public keys Ai, Bi, Ci used in multisig locking scripts. This can all be derived by the payment host without knowing the admin private keys a, b, c, but each of the admins have what they need to unlock the script because they also know the masking key mi.
// The full proof to link the multisig locking script to the credentials only requires the signed credential itself (includes A, B, C, Sx) the child key derivation path i, and two parts from two zero-knowledge proofs V and r. The standardized protocol should allow the verifier to independently compute the challenge hash t and the proof P.  If P==V then the ZKP is valid. The validator can then know that the locking script is linked the the paymail handle and the admins have self-certified that they have control of keys necessary to unlock the script. To the public, even those with the credentials that include the entity’s extended pubkey, there’s no way to correlate the entity’s locking scripts apart from the basic control arrangement, e.g. 2 of 3. (edited)

// sx - Provide third party with private extended key and derivation method.
// A - Provide third party with public key.

// mi - derive private key from sx with derivation path i
// Mi - public key from mi
// Ai - derive public key from A * mi

// k - generate random value
// V = kG + kA
// t = H(A||Mi||Ai)
// r = k + (t * mi)
// P = r(G + A) - t(Mi + Ai)

// Valid if V == P

// DerivationZKP is a zero knowledge proof of the source of the derivation of a key.
// It proves that a specified key is derived from another key.
type DerivationZKP struct {
	PublicKey  PublicKey
	Path       uint32
	Commitment Hash32
	Response   Hash32
}

// DeriveKeyZKP derives a key and a ZKP to prove the source of the derivation.
func DeriveKeyZKP(shared *ExtendedKey, source PublicKey, path uint32) (*DerivationZKP, error) {
	sharedDerivedExtended, err := shared.ChildKey(path)
	if err != nil {
		return nil, errors.Wrap(err, "derive shared")
	}

	sharedDerived := sharedDerivedExtended.Key(MainNet)

	sourceDerived, err := sharedDerived.AddPublicKey(source)
	if err != nil {
		return nil, errors.Wrap(err, "derive source")
	}

	k, err := GenerateKey(MainNet)
	if err != nil {
		return nil, errors.Wrap(err, "random key")
	}

	kG := k.PublicKey()
	kDerived, err := k.MultiplyPublicKey(sourceDerived)
	if err != nil {
		return nil, errors.Wrap(err, "multiply source derived")
	}

	// V = kG + source
	v, err := kG.Add(kDerived)
	if err != nil {
		return nil, errors.Wrap(err, "add source derived")
	}

	// t = H(source_public||shared_derived_public||source_derived_public)
	h := sha256.New()
	if _, err := h.Write(source.Bytes()); err != nil {
		return nil, errors.Wrap(err, "hash source")
	}

	if _, err := h.Write(sharedDerived.Bytes()); err != nil {
		return nil, errors.Wrap(err, "hash shared")
	}

	if _, err := h.Write(sourceDerived.Bytes()); err != nil {
		return nil, errors.Wrap(err, "hash source derived")
	}

	t := h.Sum(nil)
	tk, err := KeyFromNumber(t[:], MainNet)
	if err != nil {
		return nil, errors.Wrap(err, "t key from number")
	}

	// r = k + (t * shared_derived)
	r, err := sharedDerived.Multiply(tk)
	if err != nil {
		return nil, errors.Wrap(err, "multiply t")
	}

	r, err = r.Add(k)
	if err != nil {
		return nil, errors.Wrap(err, "add k")
	}

	// P = r(G + source_public) - t(shared_derived_public + source_derived_public)
	gPlusSource, err := G().Add(source)
	if err != nil {
		return nil, errors.Wrap(err, "G + source")
	}

	pr, err := r.MultiplyPublicKey(gPlusSource)
	if err != nil {
		return nil, errors.Wrap(err, "r * (G + source)")
	}

	sharedDerivedPlusSourceDerived, err := sharedDerived.PublicKey().Add(sourceDerived)
	if err != nil {
		return nil, errors.Wrap(err, "G + source")
	}

	pt, err := tk.MultiplyPublicKey(sharedDerivedPlusSourceDerived)
	if err != nil {
		return nil, errors.Wrap(err, "t * (shared_derived + source_derived)")
	}

	p, err := pr.Subtract(pt)
	if err != nil {
		return nil, errors.Wrap(err, "subtract")
	}

	if !p.Equal(v) {
		return nil, fmt.Errorf("ZKP Check failed : V != P")
	}

	return &DerivationZKP{
		PublicKey: sourceDerived,
		Path:      path,
		// Commitment: v.HashValue(),
		Response: r.HashValue(),
	}, nil
}

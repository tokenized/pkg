package txbuilder

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

func TestSigHash(t *testing.T) {
	tests := []struct {
		hex_transaction string
		script          string
		input_index     int
		value           uint64
		hashType        int
		signature_hash  string
	}{
		{"907c2bc503ade11cc3b04eb2918b6f547b0630ab569273824748c87ea14b0696526c66ba740200000004ab65ababfd1f9bdd4ef073c7afc4ae00da8a66f429c917a0081ad1e1dabce28d373eab81d8628de802000000096aab5253ab52000052ad042b5f25efb33beec9f3364e8a9139e8439d9d7e26529c3c30b6c3fd89f8684cfd68ea0200000009ab53526500636a52ab599ac2fe02a526ed040000000008535300516352515164370e010000000003006300ab2ec229", "", 2, 123, 1864164639, "ac37112171153d11b4ef51bde62599a5c4636cbe6b22afeb469de80a3d3c346a"},
		{"a0aa3126041621a6dea5b800141aa696daf28408959dfb2df96095db9fa425ad3f427f2f6103000000015360290e9c6063fa26912c2e7fb6a0ad80f1c5fea1771d42f12976092e7a85a4229fdb6e890000000001abc109f6e47688ac0e4682988785744602b8c87228fcef0695085edf19088af1a9db126e93000000000665516aac536affffffff8fe53e0806e12dfd05d67ac68f4768fdbe23fc48ace22a5aa8ba04c96d58e2750300000009ac51abac63ab5153650524aa680455ce7b000000000000499e50030000000008636a00ac526563ac5051ee030000000003abacabd2b6fe000000000003516563910fb6b5", "65", 0, 84626, -1391424484, "c2e5d07178ae69868741fb99710a9356009dbb7b280afb0ac0b0c8c957f5c0f0"},
		{"6e7e9d4b04ce17afa1e8546b627bb8d89a6a7fefd9d892ec8a192d79c2ceafc01694a6a7e7030000000953ac6a51006353636a33bced1544f797f08ceed02f108da22cd24c9e7809a446c61eb3895914508ac91f07053a01000000055163ab516affffffff11dc54eee8f9e4ff0bcf6b1a1a35b1cd10d63389571375501af7444073bcec3c02000000046aab53514a821f0ce3956e235f71e4c69d91abe1e93fb703bd33039ac567249ed339bf0ba0883ef300000000090063ab65000065ac654bec3cc504bcf499020000000005ab6a52abac64eb060100000000076a6a5351650053bbbc130100000000056a6aab53abd6e1380100000000026a51c4e509b8", "acab655151", 0, 943837271, 479279909, "cbfd22524e78bb72007600e8e5a5264f3bbebb3ced8eda07440b8434e42baeda"},
		{"73107cbd025c22ebc8c3e0a47b2a760739216a528de8d4dab5d45cbeb3051cebae73b01ca10200000007ab6353656a636affffffffe26816dffc670841e6a6c8c61c586da401df1261a330a6c6b3dd9f9a0789bc9e000000000800ac6552ac6aac51ffffffff0174a8f0010000000004ac52515100000000", "5163ac63635151ac", 1, 9351684, 1190874345, "76ea6155aad3a53b3a4b00aac404ec596185b6f199e605b4ca242f16aab28af7"},
		{"e93bbf6902be872933cb987fc26ba0f914fcfc2f6ce555258554dd9939d12032a8536c8802030000000453ac5353eabb6451e074e6fef9de211347d6a45900ea5aaf2636ef7967f565dce66fa451805c5cd10000000003525253ffffffff047dc3e6020000000007516565ac656aabec9eea010000000001633e46e600000000000015080a030000000001ab00000000", "5300ac6a53ab6a", 1, 1, -886562767, "bb5754484700a4ea35e9e09b58256fe6ac09d582f0cf222afe1bceb0c5531458"},
		{"50818f4c01b464538b1e7e7f5ae4ed96ad23c68c830e78da9a845bc19b5c3b0b20bb82e5e9030000000763526a63655352ffffffff023b3f9c040000000008630051516a6a5163a83caf01000000000553ab65510000000000", "6aac", 0, 756, 946795545, "097126c0342f2513255ebf77b4948ebaab25aa37b926b9f78fe70db55e5410ce"},
	}

	for _, tt := range tests {
		t.Run(tt.signature_hash, func(t *testing.T) {
			txData, err := hex.DecodeString(tt.hex_transaction)
			if err != nil {
				t.Fatalf("Failed to decode tx hex : %s\n", err)
			}

			var tx wire.MsgTx
			txBuf := bytes.NewBuffer(txData)
			if err := tx.Deserialize(txBuf); err != nil {
				t.Fatalf("Failed to deserialize tx : %s\n", err)
			}

			script, err := hex.DecodeString(tt.script)
			if err != nil {
				t.Fatalf("Failed to decode script hex : %s\n", err)
			}

			wantHash, err := hex.DecodeString(tt.signature_hash)
			if err != nil {
				t.Fatalf("Failed to decode signature hash hex : %s\n", err)
			}

			var hashCache SigHashCache
			gotHash, err := signatureHash(&tx, tt.input_index, script, tt.value,
				SigHashType(tt.hashType), &hashCache)
			if err != nil {
				t.Fatalf("Failed to generate signature hash : %s\n", err)
			}

			if !bytes.Equal(wantHash, gotHash) {
				t.Fatalf("Invalid Sig Hash\ngot:%x\nwant:%x", gotHash, wantHash)
			}
		})
	}
}

func TestSigHashJS(t *testing.T) {
	b, err := hex.DecodeString("01000000019418223c6d8d9e4d3fc5acd4d2641a7021362906bddb14d4ed9bd33b7d5e0cd10000000000ffffffff0258020000000000001976a914f01354f339b033474c4f607c03036224b5d9c0bd88acc3230000000000001976a914047ee96e142bb9763e0c4de2688821b4e8c5708888ac00000000")
	if err != nil {
		t.Fatalf("Failed to decode tx hex : %s", err)
	}

	msgTx := wire.NewMsgTx(1)
	if err := msgTx.Deserialize(bytes.NewReader(b)); err != nil {
		t.Fatalf("Failed to decode tx : %s", err)
	}

	lockingScript, err := hex.DecodeString("76a9145315bffb33ab27eac7c4113299ccb020ce4344ee88ac")
	if err != nil {
		t.Fatalf("Failed to decode locking script hex : %s", err)
	}

	tx := NewTxBuilder(1.0, 1.0)
	tx.MsgTx = msgTx
	tx.Inputs = []*InputSupplement{
		&InputSupplement{
			LockingScript: lockingScript,
			Value:         10000,
		},
	}

	hash, err := signatureHash(msgTx, 0, lockingScript, 10000, SigHashAll+SigHashForkID,
		&SigHashCache{})
	if err != nil {
		t.Fatalf("Failed to create sig hash : %s", err)
	}

	h, err := bitcoin.NewHash32(hash)

	t.Logf("Tx : %s", msgTx)

	t.Logf("Sig hash : %s\n", h)
}

func TestTxSig(t *testing.T) {
	t.Skip() // Not sure what is wrong with this test --ce

	h := "0100000003e2c28e2d82fe4d70a1440e421f36a2c22d63d92e3236fb101bca001055f0866c010000006a4730440220435690c216261bdba7faa0ce1bab20029ae3d5401aa864df484c0cfd8e4bed3b022069a89448714e3f114cec42aeca387591a911533ee4e94721ae4e387a884fbf0e4121035872eb69ccb4537d9ab825253042ab5ec050c769a1c8cd761833266e2dcafa1fffffffffd77152da599947d0462c4f698996752934b65ee7e5e8758925eaaaa0e9f6986a010000006a47304402200e1ac2e59438acb35e45b03fa20336f853daf0e2bb9baa8a2aef27cbe6d868420220117708898b906f9a3873b9dbd34fbded15ff2f5d3d94ebe25a55acac3001de444121034ed0ea77ce2e57dd1040654b6aceec413b249c5d78006d65304090004e95fcfcffffffffe2ff4a51a41057d0a4aba810a4f6dbb3b48a3d6ce7e7c63e0df08a064aa66e94030000006b4830450221009314a8b43e7c666be97cfe1d2cf89eef039d940584687bc10a10696e1a8e77d002201fa88385f8e8b8523e7916fa0f0c11d303cd9ff61a129f3deffd2c93b2ad8251412103f59fa994c5d6f8fa302e93d6771bb00cb2d01ac06be1ba5ec797e991c9efe4efffffffff03470b0000000000001976a914448744dfc5194b515908c58cebdd9aa2aadfb71088ac0000000000000000fd2101006a02bd0008746573742e544b4e041a0254314d0b010a880212034343591a140d3c8e864bd937f69aa7657495c1143eed677972220310f4032a720a15204912a0273590fa136510b05afad65bddd27d5e4610900318012a46304402204029f9c23d7ec1f2930c33a3f45e1843c24d866b92fc4b9fb18323d0f8323d4402206cc6acb79f8d4c0a9beed71c1aa14447538a24f5fe644d97cea162564d2473003097df2938d483b58f9fe98dbd162a720a152052b7d8ec766df65e6c845abb35ee617483725384106418012a4730450221008afbf8a38b0156ba051026d0e7538e129c413b9e545babb96109b698440bac47022018e8ff3a2d928207b68ae66018a4ec09e3a0858da4cb325cff564d4947f7d5423097df2938a8c0bc989fe98dbd16bf080000000000001976a9140a7f185d1ed9d3120dab77476e1e71305cc907ce88ac00000000"
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("Failed to decode hex : %s", err)
	}

	msgTx := wire.NewMsgTx(1)
	if err := msgTx.Deserialize(bytes.NewReader(b)); err != nil {
		t.Fatalf("Failed to deserialize tx : %s", err)
	}

	utxos := []bitcoin.UTXO{
		bitcoin.UTXO{
			Index: 1,
			Value: 136,
		},
		bitcoin.UTXO{
			Index: 1,
			Value: 1507,
		},
		bitcoin.UTXO{
			Index: 3,
			Value: 3894,
		},
	}

	hash, err := bitcoin.NewHash32FromStr("6c86f0551000ca1b10fb36322ed9632dc2a2361f420e44a1704dfe822d8ec2e2")
	if err != nil {
		t.Fatalf("Failed to decode hash : %s", err)
	}
	utxos[0].Hash = *hash

	utxos[0].LockingScript, err = hex.DecodeString("76a91481218b4f1e8b0b0e509b67ae21cf956222f5488088ac")
	if err != nil {
		t.Fatalf("Failed to decode locking script : %s", err)
	}

	hash, err = bitcoin.NewHash32FromStr("6a98f6e9a0aaea258975e8e5e75eb63429759689694f2c46d0479959da5271d7")
	if err != nil {
		t.Fatalf("Failed to decode hash : %s", err)
	}
	utxos[1].Hash = *hash

	utxos[1].LockingScript, err = hex.DecodeString("76a914b6753eaf42d1e2920831d4c6f04c42b9ec82744688ac")
	if err != nil {
		t.Fatalf("Failed to decode locking script : %s", err)
	}

	hash, err = bitcoin.NewHash32FromStr("946ea64a068af00d3ec6e7e76c3d8ab4b3dbf6a410a8aba4d05710a4514affe2")
	if err != nil {
		t.Fatalf("Failed to decode hash : %s", err)
	}
	utxos[2].Hash = *hash

	utxos[2].LockingScript, err = hex.DecodeString("76a91454de557ca96abc0604c0b9fa0ecc59a4b03b9ce988ac")
	if err != nil {
		t.Fatalf("Failed to decode locking script : %s", err)
	}

	tx, err := NewTxBuilderFromWireUTXOs(1.0, 1.0, msgTx, utxos)
	if err != nil {
		t.Fatalf("Failed to create tx : %s", err)
	}

	t.Logf("Tx : %s\n", tx.String(bitcoin.MainNet))

	pubKey1, err := bitcoin.PublicKeyFromStr("035872eb69ccb4537d9ab825253042ab5ec050c769a1c8cd761833266e2dcafa1f")
	if err != nil {
		t.Fatalf("Failed to decode pub key : %s", err)
	}

	sig1, err := bitcoin.SignatureFromStr("30440220435690c216261bdba7faa0ce1bab20029ae3d5401aa864df484c0cfd8e4bed3b022069a89448714e3f114cec42aeca387591a911533ee4e94721ae4e387a884fbf0e")
	if err != nil {
		t.Fatalf("Failed to decode sig : %s", err)
	}

	hashCache := &SigHashCache{}

	sigHashBytes, err := signatureHash(msgTx, 0, utxos[0].LockingScript, utxos[0].Value,
		SigHashAll|SigHashForkID, hashCache)

	rSigHashBytes := make([]byte, 32)
	reverse32(sigHashBytes, rSigHashBytes)

	if !sig1.Verify(rSigHashBytes, pubKey1) {
		t.Fatalf("Failed to verify sig 1")
	}

	// pubKey2, err := bitcoin.PublicKeyFromStr("034ed0ea77ce2e57dd1040654b6aceec413b249c5d78006d65304090004e95fcfc")
	// if err != nil {
	// 	t.Fatalf("Failed to decode pub key : %s", err)
	// }

	// pubKey3, err := bitcoin.PublicKeyFromStr("03f59fa994c5d6f8fa302e93d6771bb00cb2d01ac06be1ba5ec797e991c9efe4ef")
	// if err != nil {
	// 	t.Fatalf("Failed to decode pub key : %s", err)
	// }

}

func reverse32(h, rh []byte) {
	i := 32 - 1
	for _, b := range rh[:] {
		h[i] = b
		i--
	}
}

func TestSigHashBytes(t *testing.T) {
	tx := NewTxBuilder(0.5, 0.25)

	ad, err := bitcoin.DecodeAddress("12QTzHPj6vLPzsfcjZVtrPad5wiQ6TDvFV")
	if err != nil {
		t.Fatalf("Failed to decode address : %s", err)
	}
	ra := bitcoin.NewRawAddressFromAddress(ad)

	utxo := bitcoin.UTXO{
		Index:         1,
		Value:         10000,
		LockingScript: []byte{0},
	}

	rand.Read(utxo.Hash[:])

	tx.AddPaymentOutput(ra, 2000, false)
	tx.AddInputUTXO(utxo)

	buf := &bytes.Buffer{}
	if err := tx.MsgTx.Serialize(buf); err != nil {
		t.Fatalf("Failed to serialize tx : %s", err)
	}

	t.Logf("Tx : %x", buf.Bytes())

	buf = &bytes.Buffer{}
	if err := tx.MsgTx.TxOut[0].Serialize(buf, 0, 0); err != nil {
		t.Fatalf("Failed to serialize tx output : %s", err)
	}

	t.Logf("Outputs : %x", buf.Bytes())

	b, err := SignatureHashPreimageBytes(tx.MsgTx, 0, []byte{0}, 10000, SigHashAll|SigHashForkID,
		&SigHashCache{})
	if err != nil {
		t.Fatalf("Failed to get sig hash preimage bytes : %s", err)
	}

	t.Logf("Sig Hash Preimage : %x", b)
}

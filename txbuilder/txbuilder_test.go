package txbuilder

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

func Test_DustLimit(t *testing.T) {
	tests := []struct {
		dustFeeRateString string
		dustFeeRate       float32
		dust              uint64
	}{
		{
			dustFeeRateString: "0.0",
			dustFeeRate:       0,
			dust:              1,
		},
		{
			dustFeeRateString: "0",
			dustFeeRate:       0,
			dust:              1,
		},
		{
			dustFeeRateString: "0.25",
			dustFeeRate:       0.25,
			dust:              136,
		},
		{
			dustFeeRateString: "0.5",
			dustFeeRate:       0.5,
			dust:              273,
		},
		{
			dustFeeRateString: "1.0",
			dustFeeRate:       1.0,
			dust:              546,
		},
		{
			dustFeeRateString: "1",
			dustFeeRate:       1,
			dust:              546,
		},
	}

	for _, tt := range tests {
		t.Run(tt.dustFeeRateString, func(t *testing.T) {
			dustFeeRate64, err := strconv.ParseFloat(tt.dustFeeRateString, 32)
			if err != nil {
				return
			}

			dustFeeRate := float32(dustFeeRate64)

			if dustFeeRate != tt.dustFeeRate {
				t.Errorf("Wrong dust fee rate : got %f, want %f", dustFeeRate, tt.dustFeeRate)
			}

			dust := DustLimit(P2PKHOutputSize, dustFeeRate)
			if dust != tt.dust {
				t.Errorf("Wrong dust : got %d, want %d", dust, tt.dust)
			}
		})
	}
}

func TestBasic(t *testing.T) {
	key, err := bitcoin.GenerateKey(bitcoin.TestNet)
	if err != nil {
		t.Errorf("Failed to create private key : %s", err)
	}

	pkh := bitcoin.Hash160(key.PublicKey().Bytes())
	address, err := bitcoin.NewRawAddressPKH(pkh)
	if err != nil {
		t.Errorf("Failed to create pkh address : %s", err)
	}

	key2, err := bitcoin.GenerateKey(bitcoin.TestNet)
	if err != nil {
		t.Errorf("Failed to create private key 2 : %s", err)
	}

	pkh2 := bitcoin.Hash160(key2.PublicKey().Bytes())
	address2, err := bitcoin.NewRawAddressPKH(pkh2)
	if err != nil {
		t.Errorf("Failed to create pkh address 2 : %s", err)
	}

	inputTx := NewTxBuilder(1.1, 1.0)
	inputTx.SetChangeAddress(address2, "")
	err = inputTx.AddPaymentOutput(address, 10000, true)
	if err != nil {
		t.Errorf("Failed to add output : %s", err)
	}

	tx := NewTxBuilder(1.0, 1.0)
	tx.SetChangeAddress(address, "")

	err = tx.AddInput(wire.OutPoint{Hash: *inputTx.MsgTx.TxHash(), Index: 0},
		inputTx.MsgTx.TxOut[0].LockingScript, uint64(inputTx.MsgTx.TxOut[0].Value))
	if err != nil {
		t.Errorf("Failed to add input : %s", err)
	}

	err = tx.AddPaymentOutput(address2, 5000, false)
	if err != nil {
		t.Errorf("Failed to add output : %s", err)
	}

	err = tx.AddDustOutput(address, true)
	if err != nil {
		t.Errorf("Failed to add output : %s", err)
	}

	// Test single valid private key
	var signingKeys []bitcoin.Key
	signingKeys, err = tx.Sign([]bitcoin.Key{key})
	if err != nil {
		t.Errorf("Failed to sign tx : %s", err)
	}
	t.Logf("Tx Fee : %d", tx.Fee())

	if len(signingKeys) != 1 {
		t.Fatalf("Wrong signingKeys count : got %d, want %d", len(signingKeys), 1)
	}

	if !signingKeys[0].Equal(key) {
		t.Errorf("Wrong signing key : got %s, want %s", signingKeys[0].String(), key.String())
	}

	// Test extra private key
	signingKeys, err = tx.Sign([]bitcoin.Key{key, key2})
	if err != nil {
		t.Errorf("Failed to sign tx with both keys : %s", err)
	}
	t.Logf("Tx Fee : %d", tx.Fee())

	if len(signingKeys) != 1 {
		t.Fatalf("Wrong signingKeys count : got %d, want %d", len(signingKeys), 1)
	}

	if !signingKeys[0].Equal(key) {
		t.Errorf("Wrong signing key : got %s, want %s", signingKeys[0].String(), key.String())
	}

	// Test wrong private key
	_, err = tx.Sign([]bitcoin.Key{key2})
	if errors.Cause(err) == ErrWrongPrivateKey {
		if err != nil {
			t.Errorf("Failed to return wrong private key error : %s", err)
		} else {
			t.Errorf("Failed to return wrong private key error")
		}
	}
	t.Logf("Tx Fee : %d", tx.Fee())

	// Test bad LockingScript
	txMalformed := NewTxBuilder(1.0, 1.0)
	txMalformed.SetChangeAddress(address, "")
	err = txMalformed.AddInput(wire.OutPoint{Hash: *inputTx.MsgTx.TxHash(), Index: 0},
		append(inputTx.MsgTx.TxOut[0].LockingScript, 5), uint64(inputTx.MsgTx.TxOut[0].Value))
	if errors.Cause(err) == ErrWrongScriptTemplate {
		if err != nil {
			t.Errorf("Failed to return \"Not P2PKH Script\" error : %s", err)
		} else {
			t.Errorf("Failed to return \"Not P2PKH Script\" error")
		}
	}
}

func TestSample(t *testing.T) {
	// Load your private key
	wif := "cQDgbH4C7HP3LSJevMSb1dPMCviCPoLwJ28mxnDRJueMSCa72xjm"
	key, err := bitcoin.KeyFromStr(wif)
	if err != nil {
		t.Fatalf("Failed to decode key : %s", err)
	}

	// Decode an address to use for "change".
	// Middle return parameter is the network detected. This should be checked to ensure the address
	//   was encoded for the currently specified network.
	changeAddress, _ := bitcoin.DecodeAddress("mq4htwkZSAG9isuVbEvcLaAiNL59p26W64")

	// Create an instance of the TxBuilder using 1.1 as the dust rate and 1.1 sat/byte fee rate.
	builder := NewTxBuilder(1.1, 1.0)
	builder.SetChangeAddress(bitcoin.NewRawAddressFromAddress(changeAddress), "")

	// Add an input
	// To spend an input you need the txid, output index, and the locking script and value from that output.
	hash, _ := bitcoin.NewHash32FromStr("c762a29a4beb4821ad843590c3f11ffaed38b7eadc74557bdf36da3539921531")
	index := uint32(0)
	value := uint64(2000)
	spendAddress, _ := bitcoin.DecodeAddress("mupiWN44gq3NZmvZuMMyx8KbRwism69Gbw")
	lockingScript, _ := bitcoin.NewRawAddressFromAddress(spendAddress).LockingScript()
	_ = builder.AddInput(*wire.NewOutPoint(hash, index), lockingScript, value)

	// add an output to the recipient
	paymentAddress, _ := bitcoin.DecodeAddress("n1kBjpqmH82jgiRnEHLmFMNv77kvugBomm")
	_ = builder.AddPaymentOutput(bitcoin.NewRawAddressFromAddress(paymentAddress), 1000, false)

	// sign the first and only input
	builder.Sign([]bitcoin.Key{key})

	// get the raw TX bytes
	_, _ = builder.Serialize()
}

func TestTxSigHash(t *testing.T) {
	txData, err := hex.DecodeString("0100000002e5a2041ebfcdb5594616fd090d1065b48dbb3bb0cf75dbc0028ba3e82404665a000000006b483045022100875980a2c82af1ccb3493cf857c3d807f182c334458749b5284e7b207d16e5f402200eaba277fdb8d15e862e488074284bde0b4adfbcfe43cdbc96db29dbd380ad334121034f31d5c213db1a2847fa1a3425e9bdc5f8104d11f74b68434d8365f17acfb6c3ffffffff681506afb99bf4a98a1ab8082438003aee835ebcdc90b0fd5701769d42ac4ef3020000006a47304402201754f8aec11c2aab1c41df9e5717b9f88616cc9b0992f6a8c1f9b510f2d88429022040496964eacd71feaa628ae2824c251b2e098a9b8afb4b61cd4c34daae1d1cf24121034f31d5c213db1a2847fa1a3425e9bdc5f8104d11f74b68434d8365f17acfb6c3ffffffff03b30d0000000000001976a9145ca2479d4bc988bff6a5b67d6bebd24a4ef3ff3d88ac000000000000000095006a02bd000e746573742e746f6b656e697a6564041a0241314c7a0a08fffffffffffffff0100120015080c2d72f5a034c4f596260121e546f6b656e416972204672657175656e7420466c79657220506f696e7473188080d488b4d1a3cc1520808080eb8b91f7fc192a2a4672657175656e7420666c79657220706f696e747320666f7220546f6b656e41697220666c6967687473ce182f00000000001976a9148c9420efb9f98392397a999100a1e62cc7419ec588ac00000000")
	if err != nil {
		t.Fatalf("Failed to deserialize tx hex : %s", err)
	}
	msg := &wire.MsgTx{}
	buf := bytes.NewBuffer(txData)
	err = msg.Deserialize(buf)
	if err != nil {
		t.Fatalf("Failed to deserialize tx : %s", err)
	}

	txData1, err := hex.DecodeString("01000000017fe4b224d93776bd79b6385f30489f61e5cce5799fe7de925f030ae46bf1e3cd000000006a473044022045afb3609660866da172ca51a6185f37a0083ef8bd06f1e93663992fbee26e52022056b1ee844c8d3df24429d59849f297dd7703fe10691b61d7ded4dc82c94405be41210222499db2ea899df5f3ea80cb4fbf8160f1d2de3ee368fce8df72760f4c50154fffffffff02de0c0000000000001976a9148c9420efb9f98392397a999100a1e62cc7419ec588ac0000000000000000356a0e746573742e746f6b656e697a656424004d3201000000000111004d657373616765204d616c666f726d65646928c10e98d5b81500000000")
	if err != nil {
		t.Fatalf("Failed to deserialize tx 1 hex : %s", err)
	}
	msg1 := &wire.MsgTx{}
	buf = bytes.NewBuffer(txData1)
	err = msg1.Deserialize(buf)
	if err != nil {
		t.Fatalf("Failed to deserialize tx 1 : %s", err)
	}

	txData2, err := hex.DecodeString("0100000002f5c02af0fdddab094e1be7c8ac1265f74bfef9d27051d356cf82eaa9bac2b020000000006b4830450221008c6cc6ad165d599eeef3fd9c653496503258abe13a65d75ff66620487dba01e102207c9e28304faafa3398a8622e96c56ee4fe77bc97a25eb3d7c55b5f6721eba22f4121034f31d5c213db1a2847fa1a3425e9bdc5f8104d11f74b68434d8365f17acfb6c3ffffffffafa8a25e86285de9d20e4e2b0e6ded75535eeae151a39811fc0b48047b3bc9d7020000006a4730440220407f4cd703d3c587077cc342a6e77af696e89b95eb864a879cb7a044fae4251a02204cea701b47f3ebbcfd4a24d7908e481187371d9b5676c4ad62f4ad4f8645e3764121034f31d5c213db1a2847fa1a3425e9bdc5f8104d11f74b68434d8365f17acfb6c3ffffffff03ee0d0000000000001976a914b6cd97de385a23dc38b6b9b511d1da3a548c77f688ac0000000000000000926a0e746573742e746f6b656e697a65644c800041314c4f5908fffffffffffffff001000001000000000000e1f505000000005e000000001e546f6b656e416972204672657175656e7420466c79657220506f696e7473000015418b8e9815000060bd88dcf9192a004672657175656e7420666c79657220706f696e747320666f7220546f6b656e41697220666c6967687473eb1b2f00000000001976a9148c9420efb9f98392397a999100a1e62cc7419ec588ac00000000")
	if err != nil {
		t.Fatalf("Failed to deserialize tx 2 hex : %s", err)
	}
	msg2 := &wire.MsgTx{}
	buf = bytes.NewBuffer(txData2)
	err = msg2.Deserialize(buf)
	if err != nil {
		t.Fatalf("Failed to deserialize tx 2 : %s", err)
	}

	inputs := []*wire.MsgTx{msg1, msg2}

	tx, err := NewTxBuilderFromWire(1.1, 1.0, msg, inputs)
	if err != nil {
		t.Fatalf("Failed to build tx : %s", err)
	}

	address, _ := bitcoin.DecodeAddress("1DpK41vJhoPLimRNqJYHJ2ZjG6aMBCgm3D")
	changeAddress := bitcoin.NewRawAddressFromAddress(address)
	tx.SetChangeAddress(changeAddress, "")

	hashCache := &SigHashCache{}
	sighash, err := SignatureHash(msg, 0, msg1.TxOut[0].LockingScript, msg1.TxOut[0].Value,
		SigHashAll+SigHashForkID, hashCache)
	if err != nil {
		t.Fatalf("Failed to generate signature hash : %s", err)
	}
	sighashHex := hex.EncodeToString(sighash[:])

	if sighashHex != "54638b5c5126e187cb3a6e62c28fa595c925d3c1dec50780d5a2116879eaf381" {
		t.Fatalf("Wrong sig hash : \n  got  %s\n  want %s", sighashHex,
			"54638b5c5126e187cb3a6e62c28fa595c925d3c1dec50780d5a2116879eaf381")
	}
}

func randomTxId() *bitcoin.Hash32 {
	rb := make([]byte, 32)
	rand.Read(rb)
	result, _ := bitcoin.NewHash32(rb)
	return result
}

func randomLockingScript() []byte {
	rb := make([]byte, 20)
	rand.Read(rb)
	ra, _ := bitcoin.NewRawAddressPKH(rb)
	result, _ := ra.LockingScript()
	return result
}

func randomAddress() bitcoin.RawAddress {
	rb := make([]byte, 20)
	rand.Read(rb)
	result, _ := bitcoin.NewRawAddressPKH(rb)
	return result
}

func TestAddFunding(t *testing.T) {
	utxos := []bitcoin.UTXO{
		bitcoin.UTXO{
			Hash:          *randomTxId(),
			Index:         0,
			Value:         10000,
			LockingScript: randomLockingScript(),
			KeyID:         "m/0/1",
		},
		bitcoin.UTXO{
			Hash:          *randomTxId(),
			Index:         0,
			Value:         2000,
			LockingScript: randomLockingScript(),
			KeyID:         "m/0/2",
		},
		bitcoin.UTXO{
			Hash:          *randomTxId(),
			Index:         0,
			Value:         1000,
			LockingScript: randomLockingScript(),
			KeyID:         "m/0/3",
		},
	}

	toAddress := randomAddress()
	toScript, err := toAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	changeAddress := randomAddress()
	changeScript, err := changeAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	// Change address needed ***********************************************************************
	tx := NewTxBuilder(1.1, 1.0)
	if err != nil {
		t.Fatalf("Failed to build max send tx : %s", err)
	}
	tx.SetChangeAddress(changeAddress, "")

	err = tx.AddPaymentOutput(toAddress, 600, false)
	if err != nil {
		t.Fatalf("Failed to payment : %s", err)
	}

	err = tx.AddFunding(utxos)
	if err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	fee := float32(tx.Fee())
	t.Logf("Fee : %d", uint64(fee))
	t.Logf("Estimated Fee : %d", uint64(float32(tx.EstimatedSize())*1.1))
	estimatedFee := float32(tx.EstimatedSize()) * 1.1
	low := estimatedFee * 0.95
	high := estimatedFee * 1.05
	if fee < low || fee > high {
		t.Fatalf("Incorrect fee : got %f, want %f", fee, estimatedFee)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[0].LockingScript, toScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[0].LockingScript, toScript)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[1].LockingScript, changeScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[1].LockingScript, changeScript)
	}

	// Already has change output *******************************************************************
	tx = NewTxBuilder(1.1, 1.0)
	if err != nil {
		t.Fatalf("Failed to build max send tx : %s", err)
	}
	tx.SetChangeAddress(changeAddress, "")

	err = tx.AddPaymentOutput(toAddress, 600, false)
	if err != nil {
		t.Fatalf("Failed to payment : %s", err)
	}

	err = tx.AddPaymentOutput(changeAddress, 700, true)
	if err != nil {
		t.Fatalf("Failed to payment : %s", err)
	}

	err = tx.AddFunding(utxos)
	if err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	fee = float32(tx.Fee())
	t.Logf("Fee : %d", uint64(fee))
	t.Logf("Estimated Fee : %d", uint64(float32(tx.EstimatedSize())*1.1))
	estimatedFee = float32(tx.EstimatedSize()) * 1.1
	low = estimatedFee * 0.95
	high = estimatedFee * 1.05
	if fee < low || fee > high {
		t.Fatalf("Incorrect fee : got %f, want %f", fee, estimatedFee)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[0].LockingScript, toScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[0].LockingScript, toScript)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[1].LockingScript, changeScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[1].LockingScript, changeScript)
	}

	// Change is dust ******************************************************************************
	tx = NewTxBuilder(1.1, 1.0)
	if err != nil {
		t.Fatalf("Failed to build max send tx : %s", err)
	}
	tx.SetChangeAddress(changeAddress, "")

	err = tx.AddPaymentOutput(toAddress, 600, false)
	if err != nil {
		t.Fatalf("Failed to payment : %s", err)
	}

	utxos[0].Value = 900
	err = tx.AddFunding(utxos)
	if err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	fee = float32(tx.Fee())
	t.Logf("Fee : %d", uint64(fee))
	t.Logf("Estimated Fee : %d", uint64(float32(tx.EstimatedSize())*1.1))
	estimatedFee = float32(tx.EstimatedSize()) * 1.1
	low = estimatedFee * 0.95
	high = 305
	if fee < low || fee > high {
		t.Fatalf("Incorrect fee : got %f, want %f", fee, estimatedFee)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[0].LockingScript, toScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[0].LockingScript, toScript)
	}

	if len(tx.Outputs) != 1 {
		t.Fatalf("Incorrect output count : got %d, want %d", len(tx.Outputs), 1)
	}
}

func TestSendMax(t *testing.T) {
	utxos := []bitcoin.UTXO{
		bitcoin.UTXO{
			Hash:          *randomTxId(),
			Index:         0,
			Value:         10000,
			LockingScript: randomLockingScript(),
			KeyID:         "m/0/1",
		},
		bitcoin.UTXO{
			Hash:          *randomTxId(),
			Index:         0,
			Value:         2000,
			LockingScript: randomLockingScript(),
			KeyID:         "m/0/2",
		},
		bitcoin.UTXO{
			Hash:          *randomTxId(),
			Index:         0,
			Value:         1000,
			LockingScript: randomLockingScript(),
			KeyID:         "m/0/3",
		},
	}

	toAddress := randomAddress()
	toScript, err := toAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	toAddress2 := randomAddress()
	toScript2, err := toAddress2.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	tx := NewTxBuilder(1.1, 1.0)
	if err != nil {
		t.Fatalf("Failed to build max send tx : %s", err)
	}

	tx.AddPaymentOutput(toAddress, 1000, false)
	if err := tx.AddMaxOutput(toAddress2); err != nil {
		t.Fatalf("Failed to add max output : %s", err)
	}

	err = tx.AddFunding(utxos[:1])
	if err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	if len(tx.Inputs) != 1 {
		t.Fatalf("Incorrect input count : got %d, want %d", len(tx.Inputs), 1)
	}

	if len(tx.Outputs) != 2 {
		t.Fatalf("Incorrect output count : got %d, want %d", len(tx.Outputs), 2)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[0].LockingScript, toScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[0].LockingScript, toScript)
	}

	fee := float32(tx.Fee())
	estimatedFee := float32(tx.EstimatedSize()) * 1.1
	low := estimatedFee * 0.95
	high := estimatedFee * 1.05
	if fee < low || fee > high {
		t.Fatalf("Incorrect fee : got %f, want %f", fee, estimatedFee)
	}

	// Attempt with 3 inputs ***********************************************************************
	tx = NewTxBuilder(1.1, 1.0)
	if err != nil {
		t.Fatalf("Failed to build max send tx : %s", err)
	}

	tx.AddMaxOutput(toAddress)

	err = tx.AddFunding(utxos)
	if err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	if len(tx.Inputs) != 3 {
		t.Fatalf("Incorrect input count : got %d, want %d", len(tx.Inputs), 3)
	}

	if len(tx.Outputs) != 1 {
		t.Fatalf("Incorrect output count : got %d, want %d", len(tx.Outputs), 1)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[0].LockingScript, toScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[0].LockingScript, toScript)
	}

	fee = float32(tx.Fee())
	estimatedFee = float32(tx.EstimatedSize()) * 1.1
	low = estimatedFee * 0.95
	high = estimatedFee * 1.05
	if fee < low || fee > high {
		t.Fatalf("Incorrect fee : got %f, want %f", fee, estimatedFee)
	}

	// Attempt with 2 addresses ********************************************************************
	tx = NewTxBuilder(1.1, 1.0)
	if err != nil {
		t.Fatalf("Failed to build max send tx : %s", err)
	}

	tx.AddPaymentOutput(toAddress, 5000, false)
	tx.AddMaxOutput(toAddress2)

	err = tx.AddFunding(utxos)
	if err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	if len(tx.Inputs) != 3 {
		t.Fatalf("Incorrect input count : got %d, want %d", len(tx.Inputs), 3)
	}

	if len(tx.Outputs) != 2 {
		t.Fatalf("Incorrect output count : got %d, want %d", len(tx.Outputs), 2)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[0].LockingScript, toScript) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[0].LockingScript, toScript)
	}

	if !bytes.Equal(tx.MsgTx.TxOut[1].LockingScript, toScript2) {
		t.Fatalf("Incorrect locking script : \ngot  %s\nwant %s", tx.MsgTx.TxOut[1].LockingScript, toScript2)
	}

	fee = float32(tx.Fee())
	estimatedFee = float32(tx.EstimatedSize()) * 1.1
	low = estimatedFee * 0.95
	high = estimatedFee * 1.05
	if fee < low || fee > high {
		t.Fatalf("Incorrect fee : got %f, want %f", fee, estimatedFee)
	}
}

func Test_LowFeeFunding(t *testing.T) {
	sendKey, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate key : %s", err)
	}

	sendLockingScript, err := sendKey.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	receiveKey, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate key : %s", err)
	}

	receiveLockingScript, err := receiveKey.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create locking script : %s", err)
	}

	changeAddresses := make([]AddressKeyID, 5)
	for i := range changeAddresses {
		changeKey, err := bitcoin.GenerateKey(bitcoin.MainNet)
		if err != nil {
			t.Fatalf("Failed to generate key : %s", err)
		}

		ra, err := changeKey.RawAddress()
		if err != nil {
			t.Fatalf("Failed to create address : %s", err)
		}

		changeAddresses[i] = AddressKeyID{
			Address: ra,
		}
	}

	utxos := []bitcoin.UTXO{
		{
			Index:         0,
			Value:         10000,
			LockingScript: sendLockingScript,
		},
	}
	rand.Read(utxos[0].Hash[:])

	scriptString := "OP_FALSE OP_RETURN 0xbd01 OP_1 0x49 0x11 OP_FALSE OP_4 OP_3 OP_1 OP_1 OP_1 OP_2 OP_1 0x7374616e64617264 OP_2 OP_1 OP_2 0xe803 OP_3 0x743fb262 OP_4 0x844db262"
	script, err := bitcoin.StringToScript(scriptString)
	if err != nil {
		t.Fatalf("Failed to parse script : %s", err)
	}

	tx := NewTxBuilder(0.05, 0.05)

	tx.AddOutput(receiveLockingScript, 1000, false, false)
	tx.AddOutput(script, 0, false, false)

	if err := tx.AddFundingBreakChange(utxos, 10000, changeAddresses); err != nil {
		t.Fatalf("Failed to add funding : %s", err)
	}

	t.Logf(tx.String(bitcoin.MainNet))
	t.Logf("Estimated Size : %d", tx.EstimatedSize())
	t.Logf("Fee : %d", tx.Fee())
	t.Logf("Estimated Fee : %d", tx.EstimatedFee())

	if tx.Fee() != tx.EstimatedFee() {
		t.Fatalf("Wrong fee : got %d, want %d", tx.Fee(), tx.EstimatedFee())
	}
}

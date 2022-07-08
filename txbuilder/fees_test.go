package txbuilder

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"strconv"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"
)

func Test_estimatedFeeValue(t *testing.T) {
	tests := []struct {
		feeRateString string
		feeRate       float32
		size, fee     uint64
	}{
		{
			feeRateString: "0.05",
			feeRate:       0.05,
			size:          360,
			fee:           18,
		},
		{
			feeRateString: "0.05",
			feeRate:       0.05,
			size:          361,
			fee:           19,
		},
		{
			feeRateString: "0.05",
			feeRate:       0.05,
			size:          400,
			fee:           20,
		},
		{
			feeRateString: "0.05",
			feeRate:       0.05,
			size:          401,
			fee:           21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.feeRateString, func(t *testing.T) {
			feeRate64, err := strconv.ParseFloat(tt.feeRateString, 32)
			if err != nil {
				return
			}
			feeRate := float32(feeRate64)

			if feeRate != tt.feeRate {
				t.Errorf("Wrong fee rate : got %f, want %f", feeRate, tt.feeRate)
			}

			t.Logf("size: %d", tt.size)

			fee := estimatedFeeValue(tt.size, float64(tt.feeRate))

			t.Logf("fee: %d", fee)

			if fee != tt.fee {
				t.Errorf("Wrong fee : got %d, want %d", fee, tt.fee)
			}
		})
	}
}

func Test_InputSize(t *testing.T) {
	var hash bitcoin.Hash32
	rand.Read(hash[:])
	key, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate key : %s", err)
	}

	txin := wire.NewTxIn(wire.NewOutPoint(&hash, uint32(rand.Intn(1000))),
		make([]byte, MaximumP2PKHSigScriptSize))
	if txin.SerializeSize() != MaximumP2PKHInputSize {
		t.Errorf("Wrong P2PKH input size : got %d, want %d", txin.SerializeSize(),
			MaximumP2PKHInputSize)
	}

	p2pkhAddress, err := bitcoin.NewRawAddressPKH(bitcoin.Hash160(key.PublicKey().Bytes()))
	if err != nil {
		t.Fatalf("Failed to create PKH address : %s", err)
	}
	p2pkhLockingScript, err := p2pkhAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create PKH locking script : %s", err)
	}

	inputScriptSize, err := UnlockingScriptSize(p2pkhLockingScript)
	if err != nil {
		t.Fatalf("Failed to calculate P2PKH unlocking script size : %s", err)
	}
	if inputScriptSize != MaximumP2PKHSigScriptSize {
		t.Errorf("Wrong P2PKH input script size : got %d, want %d", inputScriptSize,
			MaximumP2PKHSigScriptSize)
	}

	inputSize, err := InputSize(p2pkhLockingScript)
	if err != nil {
		t.Fatalf("Failed to calculate input size : %s", err)
	}
	if inputSize != MaximumP2PKHInputSize {
		t.Errorf("Wrong P2PKH input size : got %d, want %d", inputSize, MaximumP2PKHInputSize)
	}

	t.Logf("Maximum P2PKH Input Size : %d", MaximumP2PKHInputSize)

	txin = wire.NewTxIn(wire.NewOutPoint(&hash, uint32(rand.Intn(1000))),
		make([]byte, MaximumP2PKSigScriptSize))
	if txin.SerializeSize() != MaximumP2PKInputSize {
		t.Errorf("Wrong P2PK input size : got %d, want %d", txin.SerializeSize(),
			MaximumP2PKInputSize)
	}

	p2pkAddress, err := bitcoin.NewRawAddressPublicKey(key.PublicKey())
	if err != nil {
		t.Fatalf("Failed to create PKH address : %s", err)
	}
	p2pkLockingScript, err := p2pkAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create PKH locking script : %s", err)
	}

	inputScriptSize, err = UnlockingScriptSize(p2pkLockingScript)
	if err != nil {
		t.Fatalf("Failed to calculate P2PK unlocking script size : %s", err)
	}
	if inputScriptSize != MaximumP2PKSigScriptSize {
		t.Errorf("Wrong P2PK input script size : got %d, want %d", inputScriptSize,
			MaximumP2PKSigScriptSize)
	}

	inputSize, err = InputSize(p2pkLockingScript)
	if err != nil {
		t.Fatalf("Failed to calculate input size : %s", err)
	}
	if inputSize != MaximumP2PKInputSize {
		t.Errorf("Wrong P2PK input size : got %d, want %d", inputSize, MaximumP2PKInputSize)
	}

	t.Logf("Maximum P2PK Input Size : %d", MaximumP2PKInputSize)
}

func Test_OutputSize(t *testing.T) {
	key, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate key : %s", err)
	}

	txout := wire.NewTxOut(uint64(rand.Intn(100000)), make([]byte, P2PKHOutputScriptSize))
	if txout.SerializeSize() != P2PKHOutputSize {
		t.Errorf("Wrong P2PKH output size : got %d, want %d", txout.SerializeSize(),
			P2PKHOutputSize)
	}

	p2pkhAddress, err := bitcoin.NewRawAddressPKH(bitcoin.Hash160(key.PublicKey().Bytes()))
	if err != nil {
		t.Fatalf("Failed to create PKH address : %s", err)
	}
	p2pkhLockingScript, err := p2pkhAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create PKH locking script : %s", err)
	}
	if len(p2pkhLockingScript) != P2PKHOutputScriptSize {
		t.Errorf("Wrong P2PKH output script size : got %d, want %d", len(p2pkhLockingScript),
			P2PKHOutputScriptSize)
	}

	outputSize := OutputSize(p2pkhLockingScript)
	if outputSize != P2PKHOutputSize {
		t.Errorf("Wrong P2PK output size : got %d, want %d", outputSize, P2PKHOutputSize)
	}

	t.Logf("P2PKH Output Size : %d", P2PKHOutputSize)

	txout = wire.NewTxOut(uint64(rand.Intn(100000)), make([]byte, P2PKOutputScriptSize))
	if txout.SerializeSize() != P2PKOutputSize {
		t.Errorf("Wrong P2PK output size : got %d, want %d", txout.SerializeSize(),
			P2PKOutputSize)
	}

	p2pkAddress, err := bitcoin.NewRawAddressPublicKey(key.PublicKey())
	if err != nil {
		t.Fatalf("Failed to create PKH address : %s", err)
	}
	p2pkLockingScript, err := p2pkAddress.LockingScript()
	if err != nil {
		t.Fatalf("Failed to create PKH locking script : %s", err)
	}
	if len(p2pkLockingScript) != P2PKOutputScriptSize {
		t.Errorf("Wrong P2PK output script size : got %d, want %d", len(p2pkLockingScript),
			P2PKOutputScriptSize)
	}

	outputSize = OutputSize(p2pkLockingScript)
	if outputSize != P2PKOutputSize {
		t.Errorf("Wrong P2PK output size : got %d, want %d", outputSize, P2PKOutputSize)
	}

	t.Logf("P2PK Output Size : %d", P2PKOutputSize)
}

func Test_MultiPKH_EstimatedSize(t *testing.T) {
	h := `0100000002aa8e2fb1f15ab879870d1c6dc03026728832195a876ba0f920f8424ce713a4d208000000d8483045022100d36703dc712ab29583710e0081335a8bc05b486d5af7eac785ee5f4f29ec3b5802202a904ccc84d70ee1f13b5f8de9f707efcc26eb1bf79554a40a43adffa5d317bd4121034408301bf102018c5415a054887c06d4b27aedfc1df9df00ad74d0ed56d2ce2751483045022100fb3f3d87167f60b5eb0d564b57c815c01996c5f0cc9dc22841cee1a486b8544e02205a5b656cae72c46d5a2e6ec1c2884a4dae56959db7bdff10854f15fa5ed7b6b24121034bb34e72cf04af301e077af747ab8981bc14851f74030bb66064413a19ce181051ffffffff6c38425894b907ca666b449b2ee3bf8a8d0ebe56b3c5b131f8824b911e47f35900000000d64730440220187604c2ef20ff8ad449523d78b2becae5aad8bd41a866dc8260fbbc006146cd022020e02c2c99c4e418d27d8024448e1d6d298f0e52237312ab2ddf91ff785ab9a841210313d7863b0e3ee9f155f483948aa7dbb5f48fcf9d7a6483552d48043e4b047bbf51473044022004e9b02481e53e61212d5b23fdb67aac9f717f024362a37b05a9d17974b2492b022054baf2e2299484be340dbdccbb5356739165ca2689e069ecfba7b98236a9d08f412102efe6e5edd29dbe5ff33b29f50b38f6ff17aa77bfc54b156c99d768ce4a49936a51ffffffff047f0d0000000000001976a9140e045746a5ed34dc71b44f5c21278da509399ad888ac0000000000000000fd1c01006a02bd0008746573742e544b4e041a0254314d06010a83021203434f551a144d86521e2d83b2cd547670002dbde5bf257f5cca220210642a2f0a2b220202ac474c9dfa06820b3d40adc0d2e17dd3e99467b986849b901dfea21ff37e0cedc0c2dc58e0c060dd102e2a2f0a2b220202fcf73dcb8cb32e17b33018e40f37d45733ad28e00057e4594dba21cb2d16e9402217719b94108c3810022a2f0a2b220202b99e42408959135cc6e323e8c2010d6de347444bc9b97b2fd96a293e840f3ba98c021a185702e6ec102f2a190a152081b85ffc7a2ae7c146b8135460ff06b8b639808010012a190a1520454637daf88055c4dc5aa0352abcb3efc01d3c3410022a190a1520d525f083554a57b05386465d4c6c7ca1eb9df22d100200000000000000003b006a02bd0008746573742e544b4e041a024d312718ec0722220a20496e206f72646572207369676e6174757265732073686f756c6420776f726b3f583f00000000000041006b6376a914ee3cac0497094c4c26a20d972af784e60b59b88588ad6c8b6b686376a914cc7b57c2b3fc09d0a7388aa3b220c9aed24544c088ad6c8b6b68526ca100000000`
	b, _ := hex.DecodeString(h)

	tx := &wire.MsgTx{}
	if err := tx.Deserialize(bytes.NewReader(b)); err != nil {
		t.Fatalf("Failed to deserialize tx : %s", err)
	}

	var inputs []*wire.MsgTx
	hs := []string{
		`010000000179cf072af7d01871c133a194808299c5e951d9d2964cb5a6419b73c7c0758b20000000006a473044022072bb1eada803fd96ef800c5308a4c5252beb042841c199e9235d0d97c77383c202203db4ef59fd4a947bdcf3aa07bec9893fad9745420b92a11afb81cea3d000c4c94121029bf25b0aeba1584e470eaec5e3ae5f574a9ce07d570ffa96efcb6c41c6ff7ee2ffffffff0b88000000000000001976a91446586fbb5ad97c394ad07bbb803019cdf21c2c8688ac88000000000000001976a914e01f43eca88376327030d81bac688347a188f08988ac88000000000000001976a914474fb3e331c1bfdd22befbe47d2858327407103088ac88000000000000001976a914c89413fe3e6da7e8354a46517483c9a2ed2d4ab088ac88000000000000001976a914db3d8d482f4efc214b98b51c575a7c619e68190788ac88000000000000001976a914d10fa74c5ffe9202e14de975537b13cbbddd369888ac88000000000000001976a9148805e960787f121ac5c607b1d6c49e6dc98d55a088ac88000000000000001976a9145a527cf493d2fbcead7309b2a5f0abe01b19c9f688aca60000000000000041006b6376a914c5e42cdbd4fce4c0c5006010c7f4bc894bc871e888ad6c8b6b686376a914f477abaad1a313bc3c84f56538022b16cbd7a88f88ad6c8b6b68526ca1d0070000000000001976a914f96e8c8e1fa79f9da85a423181d8fccc352f290188ac00000000000000006d006a02bd0008746573742e544b4e041a0254324c580a4c1203434f551a144d86521e2d83b2cd547670002dbde5bf257f5cca2200220208012204080210012205080310df0322040804100522040805100522040806102622040807103222040808106410fac9aadd9387d2de1600000000`,
		`010000000595cb960661374f421f559fadb2bb12a9a3db64f93b8453d6b4af61480382e6cc030000006a47304402205f3c264defad11ff6f1ec1f86576b216fb21793dc6fe972195c744e09fa73374022054554d683991b793f4d01bbe3e5c2181d643061a2e6944374467ab5b4bd55a73412102f60fa6ff06b5e63986870918045b5edf92379e6571ae56d1f73c9563948b5f3fffffffff40cec02349a75ab57d5b6a4fc748807bdb3559413d86d24ff25febb653d8faa2000000006b483045022100a642e2a1f860f448fa869a6be4ba13be6dbc99568b26c04346030d2c56d269be022039dbd83f4d3b1d4e8ade45c3b05cdc02c7c4c804fe441c240b8a51391e0336154121020d2673bda11152ee94f7bc7c6f086c5b49453ebc0c7ee98e86e45d53a0216777ffffffff40cec02349a75ab57d5b6a4fc748807bdb3559413d86d24ff25febb653d8faa2010000006b483045022100d14d3cd369cc6b56abddb5f5ffe4a4025134e1fb904207f69b025e0007eff8e402206581b33b778ab03e22889e8a383f73c6c2c2c54b3f449559b8a666f468f7fee84121033015ba486a6e5e1ba04b07cca7c72ef84835e079ef21f9c6306112629a716071ffffffffaca48136d94dde15770c984e37ce710c68403ec0e711de16795118f79bb8e3da030000006b483045022100e30df78f86d67aa890621e3c3a28fb099d9a4dec0db2c6ded3e8002ec34209bb02206b4016d153dd18e49b19a40c394fb13ce567fa3d36ec0e4868ba1c37bb33f9f841210229346c817369d88a4b00aaba40ee26e79bf44d6029a9cb82e29f0a1e13f61331ffffffffaca48136d94dde15770c984e37ce710c68403ec0e711de16795118f79bb8e3da040000006b483045022100c109543cd70c1b1fdc7880ab14d103a92f4f10dae46ff3a64c903339105132310220292ec447c7aa6779524881228a1ce77a0036ad1cebff7475cbbb90fc2319ce7e412102a45a3e681e3356e7aa3b15dcb90ef7b7f6af1290249077948201f5cc506c9298ffffffff04204e00000000000041006b6376a914da6cb0cdfbcf3aa202dd2863466e11ca779fef1b88ad6c8b6b686376a91435928cf39b8a92f5acbc6840b5799de98e4e4d6a88ad6c8b6b68526ca1102700000000000041006b6376a914b69e7dcd1e9f9396f888cabb72ac8d6443392def88ad6c8b6b686376a91473db1151b022aab982dffb8aae48bf84392ab9a988ad6c8b6b68526ca160050000000000001976a9149ab75466b6140da78ee7e0fe2b7a72dc14e13e4c88ac10270000000000001976a9141bd51dc56eb8cb27d4ca25d4b9b0c868fe14cccc88ac00000000`,
	}
	for _, inputH := range hs {
		inputB, _ := hex.DecodeString(inputH)
		inputTx := &wire.MsgTx{}
		if err := inputTx.Deserialize(bytes.NewReader(inputB)); err != nil {
			t.Fatalf("Failed to deserialize tx : %s", err)
		}

		inputs = append(inputs, inputTx)
	}

	txb, err := NewTxBuilderFromWire(0.5, 0.25, tx, inputs)
	if err != nil {
		t.Fatalf("Failed to create txbuilder : %s", err)
	}

	estSize := txb.EstimatedSize() // was 991

	t.Logf("Estimated Size %d, actual size %d", estSize, 993)
	if estSize < 993 || estSize > 1025 {
		t.Fatalf("Estimated size out of range : %d", estSize)
	}
}

func Test_P2PKH_EstimatedSize(t *testing.T) {
	h := `0100000003a1249e40b1047bf0702d77d09f55eb88e89f9d684e943e6eb1936a824f380db7020000006a47304402206f6245bc62941e3f43b3aa853f2c64fc798e2a83caaf7bf4b8fe08e6e2974c7b0220353689ba3561a960bc14c5beb10edcb64b6ad77123d84a77d1fbecc61e057aa5412102336e8679a315e5f16723d19f94056fe3cc3fe37263f728b71e7eea50f6decf47ffffffff9480fd8c010af12d7099bd3cbc4375d0a2be973ec24f322cba30bf42ced9d541030000006b483045022100821ffe1a9a36cbf5f7d382350f8bdc4668b0608f228880f7a3520f1f4a49fc090220163355d6b82b50555d71f22da9bf4f35c3181ca85bc565d3429eb39ac341336f41210203038f66cdf9a6beb48991c332d21abb4d60beb0e6ec73a9e42492e0a6a44e78ffffffff8754fb862e2a7e4bb14c392f9c1bd4e1f6b4b72516baaeeb66c541e864228be8030000006a473044022022495fe533f78e1cc1066d026fa81b8030dcc1c9e69581dba964752259b95a6102204c19bec804b3d16a914caa76f710883652fdae31dbd8196347bc3934cb742349412102ce0c7db4a525145ffea1ecf75715b9059b7e27aff99a87eceab967ad6e43b2d9ffffffff053f090000000000001976a914309e4522405a71759cdc0197a0d6d6fb1796367088ac00000000000000007f006a02bd0008746573742e544b4e041a0243314c6a0a075465737420356260018801d00fc2011520ddcdabd4c9c195ee1b3932fa208cb8f2c289e050ca011520da5af90da50d64afbbb61f588a3bab8aebde6cfdd2011520f1f02f19bcc1d1a7ad71b493badeeae87bc2b5d8d80101f201055553412020fa0105555341202088000000000000001976a914bac7e4588d08cdd9bbe6e61a8255fcd80bd4cce788ac931e0000000000001976a91463ed87536e5c44a34135f4779cc0ab7d3770e1e288ac88000000000000001976a91440aee384d50251c0edbb1f9c1d90f8aa342ef0aa88ac00000000`
	b, _ := hex.DecodeString(h)

	tx := &wire.MsgTx{}
	if err := tx.Deserialize(bytes.NewReader(b)); err != nil {
		t.Fatalf("Failed to deserialize tx : %s", err)
	}

	var inputs []*wire.MsgTx
	hs := []string{
		`0100000002ce8a5aa8403710e63692e7958892af05c381fe7149f2b5e23ac72ae2b67c80a1020000006a47304402203b4352f231a04bfca7dc879facae12a14241e3e76ff3b1c600a2a5fdd88150e8022037b2cc43e50bc113fc4e1a8c5c6d5966ce9a12be88e388829d6a979e756310fe412102336e8679a315e5f16723d19f94056fe3cc3fe37263f728b71e7eea50f6decf47ffffffffe74f0b2269ca18a4cba05f45b4aa54f862a3fe0be1831af61c059e07c5e5d1dd030000006a473044022038fab323431ae9c06dcd134159c715d7c85cfdc693328eff832ffcc099e18ddb022074fe2f3ac7eb4f0b64d9f032e33b6ac5af6e8f644a2ed2a9bf83df2653700881412103b53dab76fe03df0f1fbd64c1c6384fa49943f3ba0361a02af88eaa3559829056ffffffff040e090000000000001976a914309e4522405a71759cdc0197a0d6d6fb1796367088ac000000000000000032006a02bd0008746573742e544b4e041a0241311e50645a03434f5562153208436f75706f6e20354a0908011203555344180288000000000000001976a914bac7e4588d08cdd9bbe6e61a8255fcd80bd4cce788acdb3a0000000000001976a914ee6a659717fb62d31b0a4cf89696e0eaf5141b1688ac00000000`,
		`01000000037fcc4cf577de92d995c64dc3a46bb0af6419bf571b123f4d2c5c323717056573000000006a4730440220242fe060c978f6e4aa1e5379c52693263f3a99421355726dab5e1fd249593f0302202f2adb6033ee1f4450a08721755efd1c55e362e0f837d861bd06e9aca7811d7b412103afc273d61979ea44ef70213a8ee4efe28465923707df6d7e8d4c58e782db4147ffffffffb77989e0033bdea7002326643e848041b7e3b382d2951b467dc16d32ff324ec6030000006b483045022100dc30518edf070fb4667ce1bd43f4acd7d13100ccedf9572cc2c518bce863d93702202b37113b09a98538cda8b187904186c096ff14a488ba37340ebb52c9de65e85741210203038f66cdf9a6beb48991c332d21abb4d60beb0e6ec73a9e42492e0a6a44e78ffffffffdaa391d4ab33122a5912a19a3420ad3f9547a2919ab2e8e7c99fb2ed2591f444000000006b483045022100b91c6ac0f61fbf013099665ad08dddd292e93e787b185c99a600ae40e1ea68f002205d56b6825a12d0f2a233f7425dca034e87ddf68979ef6d93c1ea9ffd4f9611bc4121035306aa9e10c4b728b1f048aae8c15fbc2a6b308d6e397895353ac7a17ce11c68ffffffff0440090000000000001976a914a9b228970e93e19850eb792d5009fb2bf751fde188ac000000000000000081006a02bd0008746573742e544b4e041a0243314c6c0a09547765656e20626f7060018801d00fc20115202c8984078bab231548977f5f664f1f7c45d04b93ca0115207ab2bef5f9a7077d23f74ca3d3fda1858fea5ab5d2011520f1f02f19bcc1d1a7ad71b493badeeae87bc2b5d8d80101f201055553412020fa010555534120208f150000000000001976a9141cec8401189cd1af346953fbb53bde229ea9085688ac88000000000000001976a91440aee384d50251c0edbb1f9c1d90f8aa342ef0aa88ac00000000`,
		`0100000003766094a9c8376cbbd7f52f274c58e624f0101c49ccdb278468516535717c655b000000006a47304402205099fdbe65c58110223581a56c2d5940b4b80fa665d9fabab327753888e19d7202205ee328a6130875eb831dd28f50034911b4d21a87a6e463bccbfaf764eb9cd3c44121034ce03818dc24d9ee25a391c6aed1a47896ceb3276a944b0ece2338aad465d3efffffffff0449a33f64ea80c0e3c0d4d9f4ff95563525d2e8ffbaab7a52f082cc9adb857f030000006b483045022100ed99093117cd030d97c9172edbfdcc983d17f1b97cfa2857e2b8d03f0cbbce900220197feea7ca1045f6510c1720b6e8b5352552367e578dd0f1d8825dddc6ff30f541210203038f66cdf9a6beb48991c332d21abb4d60beb0e6ec73a9e42492e0a6a44e78ffffffff9e635c67bc91fed79728a2cf5647f020d80fb38a4736d1ba0d1c7f1b3e988dfe040000006b483045022100bb87f8d6a2f53e2427f59f38fa84e47d2bc57bfca311718cb9544a984cbe405802206b9ec84e7f25cfe26b4e539677169727408c88aa8af8762745c92338b9454f40412103453af2fe77b555d1d0ee1f9b78759c765b4d154eabe42cc9bd3b75d48f7558d8ffffffff053e090000000000001976a9145d4014f1c3ddea537603ce7f7a2e8deaa8c76fe888ac00000000000000007e006a02bd0008746573742e544b4e041a0243314c690a0654657374203460018801d00fc2011520f6eb50582b4e29a1844320f8f23f16d156f0ffb2ca011520da5af90da50d64afbbb61f588a3bab8aebde6cfdd2011520f1f02f19bcc1d1a7ad71b493badeeae87bc2b5d8d80101f201055553412020fa0105555341202088000000000000001976a914964872c3d2f6a7263cf0fb28ae88d7f5a646a8a588ace2280000000000001976a9149e9dc7863e8faeb107390b7f78a2858adfb78fbe88ac88000000000000001976a91440aee384d50251c0edbb1f9c1d90f8aa342ef0aa88ac00000000`,
	}
	for _, inputH := range hs {
		inputB, _ := hex.DecodeString(inputH)
		inputTx := &wire.MsgTx{}
		if err := inputTx.Deserialize(bytes.NewReader(inputB)); err != nil {
			t.Fatalf("Failed to deserialize tx : %s", err)
		}

		inputs = append(inputs, inputTx)
	}

	txb, err := NewTxBuilderFromWire(0.5, 0.25, tx, inputs)
	if err != nil {
		t.Fatalf("Failed to create txbuilder : %s", err)
	}

	estSize := txb.EstimatedSize()

	t.Logf("Estimated Size %d, actual size %d", estSize, 724)
	if estSize < 724 || estSize > 750 {
		t.Fatalf("Estimated size out of range : %d", estSize)
	}
}

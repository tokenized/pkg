package bitcoin

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	btcdwire "github.com/btcsuite/btcd/wire"
)

type Network uint32

const (
	MainNet       Network = 0xe8f3e1e3
	TestNet       Network = 0xf4f3e5f4
	StressTestNet Network = 0xf9c4cefb
	RegTestNet    Network = 0xfabfb5da
	InvalidNet    Network = 0x00000000
)

var (
	// MainNetParams defines the network parameters for the BSV Main Network.
	MainNetParams chaincfg.Params

	// TestNetParams defines the network parameters for the BSV Test Network.
	TestNetParams chaincfg.Params

	// StressTestNetParams defines the network parameters for the BSV Stress Test Network.
	StressTestNetParams chaincfg.Params

	// RegTestNetParams defines the network parameters for the BSV Regression Network.
	RegTestNetParams chaincfg.Params
)

func NetworkFromString(name string) Network {
	switch name {
	case "mainnet":
		return MainNet
	case "testnet":
		return TestNet
	case "stn":
		return StressTestNet
	case "regtest":
		return RegTestNet
	}

	return InvalidNet
}

func NetworkName(net Network) string {
	switch net {
	case MainNet:
		return "mainnet"
	case TestNet:
		return "testnet"
	case StressTestNet:
		return "stn"
	case RegTestNet:
		return "regtest"
	case InvalidNet:
		return "invalid"
	}

	return "testnet"
}

func (n Network) String() string {
	return NetworkName(n)
}

// MarshalText returns the text encoding of the public key.
// Implements encoding.TextMarshaler interface.
func (n Network) MarshalText() ([]byte, error) {
	return []byte(NetworkName(n)), nil
}

// UnmarshalText parses a text encoded public key and sets the value of this object.
// Implements encoding.TextUnmarshaler interface.
func (n *Network) UnmarshalText(text []byte) error {
	*n = NetworkFromString(string(text))

	if *n == InvalidNet {
		return ErrInvalidNetwork
	}

	return nil
}

func NewChainParams(network string) *chaincfg.Params {
	switch network {
	default:
	case "mainnet":
		return &MainNetParams
	case "testnet":
		return &TestNetParams
	case "stn":
		return &StressTestNetParams
	case "regtest":
		return &RegTestNetParams
	}

	return nil
}

func init() {
	// Setup the MainNet params
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Name = "mainnet"
	MainNetParams.Net = btcdwire.BitcoinNet(MainNet)

	// the params need to be registed to use them.
	if err := chaincfg.Register(&MainNetParams); err != nil {
		fmt.Printf("WARNING failed to register MainNetParams")
	}

	// Setup the TestNet params
	TestNetParams = chaincfg.TestNet3Params
	TestNetParams.Name = "testnet"
	TestNetParams.Net = btcdwire.BitcoinNet(TestNet)

	// the params need to be registed to use them.
	if err := chaincfg.Register(&TestNetParams); err != nil {
		fmt.Printf("WARNING failed to register TestNetParams")
	}

	// Setup the STN params
	StressTestNetParams = chaincfg.TestNet3Params
	StressTestNetParams.Name = "stn"
	StressTestNetParams.Net = btcdwire.BitcoinNet(StressTestNet)

	// the params need to be registed to use them.
	if err := chaincfg.Register(&StressTestNetParams); err != nil {
		fmt.Printf("WARNING failed to register StressTestNetParams")
	}

	// Setup Reg Net
	RegTestNetParams = chaincfg.RegressionNetParams
	RegTestNetParams.Name = "regtest"
	RegTestNetParams.Net = btcdwire.BitcoinNet(RegTestNet)

	// the params need to be registed to use them.
	if err := chaincfg.Register(&RegTestNetParams); err != nil {
		fmt.Printf("WARNING failed to register RegTestNetParams")
	}
}

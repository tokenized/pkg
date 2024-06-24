module github.com/tokenized/pkg

go 1.22

toolchain go1.22.2

require (
	github.com/aws/aws-sdk-go v1.35.3
	github.com/bitcoin-sv/go-sdk v0.0.0-20240429162443-77aa5d129a95
	github.com/btcsuite/btcd v0.24.0
	github.com/btcsuite/btcd/btcutil v1.1.5
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0
	github.com/btcsuite/btcutil v1.0.2
	github.com/davecgh/go-spew v1.1.1
	github.com/go-test/deep v1.0.8
	github.com/gomodule/redigo v1.8.2
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.5.0
	github.com/pkg/errors v0.9.1
	github.com/scottjbarr/redis v0.0.1
	github.com/tokenized/config v0.2.2
	github.com/tokenized/logger v0.1.3
	github.com/tokenized/threads v0.1.2
	github.com/tyler-smith/go-bip32 v0.0.0-20170922074101-2c9cfd177564
	golang.org/x/crypto v0.22.0
)

require (
	github.com/FactomProject/basen v0.0.0-20150613233007-fe3947df716e // indirect
	github.com/FactomProject/btcutilecc v0.0.0-20130527213604-d3a63a5752ec // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.1.3 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792 // indirect
	github.com/cmars/basen v0.0.0-20150613233007-fe3947df716e // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	golang.org/x/sys v0.19.0 // indirect
	launchpad.net/gocheck v0.0.0-00010101000000-000000000000 // indirect
)

replace launchpad.net/gocheck => gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f

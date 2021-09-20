package miner_id

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/json"

	"github.com/pkg/errors"
)

const (
	MinerIDProtocolID = uint32(0xAC1EED88)

	minerIDSerializeVersion = uint8(0)
)

var (
	ErrNotMinerID              = errors.New("Not Miner ID Script")
	ErrInvalidMinerIDSignature = errors.New("Invalid Signature")
	ErrMissingMinerIDSignature = errors.New("Missing Signature")
)

type MinerID struct {
	Version string `json:"version"`
	Height  uint32 `json:"height"`

	PreviousMinerID    *bitcoin.PublicKey `json:"prevMinerId"`
	PreviousMinerIDSig *bitcoin.Signature `json:"prevMinerIdSig"`
	DynamicMinerID     *bitcoin.PublicKey `json:"dynamicMinerId"`

	MinerID bitcoin.PublicKey `json:"minerId"`

	ValidityCheckTx *MinerIDValidityCheckTx `json:"vctx"`

	Contact *MinerIDContact `json:"minerContact"`
}

type MinerIDValidityCheckTx struct {
	TxID  bitcoin.Hash32 `json:"txId"`
	Index uint32         `json:"vout"`
}

type MinerIDContact struct {
	Name                *string `json:"name"`
	MerchantAPIEndpoint *string `json:"merchantAPIEndPoint"`
}

// VerifyPrevious verifies the signature of the previous miner ID linking it to this miner ID.
func (mid *MinerID) VerifyPrevious() error {
	if mid.PreviousMinerID == nil {
		return nil
	}

	if mid.PreviousMinerIDSig == nil {
		return ErrMissingMinerIDSignature
	}

	prevCheck := mid.PreviousMinerID.String() + mid.MinerID.String() +
		mid.ValidityCheckTx.TxID.String()
	hash := sha256.Sum256([]byte(prevCheck))

	if !mid.PreviousMinerIDSig.Verify(hash[:], *mid.PreviousMinerID) {
		return errors.Wrap(ErrInvalidMinerIDSignature, "previous miner id")
	}

	return nil
}

func (mid MinerID) Serialize(w io.Writer) error {
	if _, err := w.Write([]byte{byte(minerIDSerializeVersion)}); err != nil {
		return errors.Wrap(err, "version")
	}

	// Version
	length := uint8(len(mid.Version))
	if err := binary.Write(w, binary.LittleEndian, length); err != nil {
		return errors.Wrap(err, "Version length")
	}
	if _, err := w.Write([]byte(mid.Version)); err != nil {
		return errors.Wrap(err, "Version")
	}

	// Height
	if err := binary.Write(w, binary.LittleEndian, mid.Height); err != nil {
		return errors.Wrap(err, "height")
	}

	// PreviousMinerID
	if mid.PreviousMinerID == nil {
		length := uint8(0)
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return errors.Wrap(err, "PreviousMinerID length")
		}
	} else {
		b := mid.PreviousMinerID.Bytes()
		length := uint8(len(b))
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return errors.Wrap(err, "PreviousMinerID length")
		}
		if err := binary.Write(w, binary.LittleEndian, b); err != nil {
			return errors.Wrap(err, "PreviousMinerID")
		}
	}

	// PreviousMinerIDSig
	if mid.PreviousMinerIDSig == nil {
		length := uint8(0)
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return errors.Wrap(err, "PreviousMinerIDSig length")
		}
	} else {
		b := mid.PreviousMinerIDSig.Bytes()
		length := uint8(len(b))
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return errors.Wrap(err, "PreviousMinerIDSig length")
		}
		if err := binary.Write(w, binary.LittleEndian, b); err != nil {
			return errors.Wrap(err, "PreviousMinerIDSig")
		}
	}

	// DynamicMinerID
	if mid.DynamicMinerID == nil {
		length := uint8(0)
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return errors.Wrap(err, "DynamicMinerID length")
		}
	} else {
		b := mid.DynamicMinerID.Bytes()
		length := uint8(len(b))
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return errors.Wrap(err, "DynamicMinerID length")
		}
		if err := binary.Write(w, binary.LittleEndian, b); err != nil {
			return errors.Wrap(err, "DynamicMinerID")
		}
	}

	// MinerID
	if err := binary.Write(w, binary.LittleEndian, mid.MinerID.Bytes()); err != nil {
		return errors.Wrap(err, "MinerID")
	}

	// ValidityCheckTx
	if mid.ValidityCheckTx == nil {
		if _, err := w.Write([]byte{byte(0)}); err != nil {
			return errors.Wrap(err, "ValidityCheckTx exists")
		}
	} else {
		if _, err := w.Write([]byte{byte(1)}); err != nil {
			return errors.Wrap(err, "ValidityCheckTx exists")
		}

		// TxID
		if err := binary.Write(w, binary.LittleEndian,
			mid.ValidityCheckTx.TxID.Bytes()); err != nil {
			return errors.Wrap(err, "ValidityCheckTx.TxID")
		}

		// Index
		if err := binary.Write(w, binary.LittleEndian, mid.ValidityCheckTx.Index); err != nil {
			return errors.Wrap(err, "ValidityCheckTx.Index length")
		}
	}

	// Contact
	if mid.Contact == nil {
		if _, err := w.Write([]byte{byte(0)}); err != nil {
			return errors.Wrap(err, "Contact exists")
		}
	} else {
		if _, err := w.Write([]byte{byte(1)}); err != nil {
			return errors.Wrap(err, "Contact exists")
		}

		// Name
		if mid.Contact.Name == nil {
			length := uint8(0)
			if err := binary.Write(w, binary.LittleEndian, length); err != nil {
				return errors.Wrap(err, "Contact.Name length")
			}
		} else {
			length := uint8(len(*mid.Contact.Name))
			if err := binary.Write(w, binary.LittleEndian, length); err != nil {
				return errors.Wrap(err, "Contact.Name length")
			}
			if _, err := w.Write([]byte(*mid.Contact.Name)); err != nil {
				return errors.Wrap(err, "Contact.Name")
			}
		}

		// MerchantAPIEndpoint
		if mid.Contact.MerchantAPIEndpoint == nil {
			length := uint8(0)
			if err := binary.Write(w, binary.LittleEndian, length); err != nil {
				return errors.Wrap(err, "Contact.MerchantAPIEndpoint length")
			}
		} else {
			length := uint8(len(*mid.Contact.MerchantAPIEndpoint))
			if err := binary.Write(w, binary.LittleEndian, length); err != nil {
				return errors.Wrap(err, "Contact.MerchantAPIEndpoint length")
			}
			if _, err := w.Write([]byte(*mid.Contact.MerchantAPIEndpoint)); err != nil {
				return errors.Wrap(err, "Contact.MerchantAPIEndpoint")
			}
		}
	}

	return nil
}

func (mid *MinerID) Deserialize(r io.Reader) error {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "version")
	}
	if uint8(b[0]) != minerIDSerializeVersion {
		return fmt.Errorf("Wrong serialize version : got %d, want %d", uint8(b[0]),
			minerIDSerializeVersion)
	}

	// Version
	var length uint8
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return errors.Wrap(err, "Version length")
	}

	b = make([]byte, length)
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "Version")
	}
	mid.Version = string(b)

	// Height
	if err := binary.Read(r, binary.LittleEndian, &mid.Height); err != nil {
		return errors.Wrap(err, "height")
	}

	// PreviousMinerID
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return errors.Wrap(err, "PreviousMinerID length")
	}

	if length == 0 {
		mid.PreviousMinerID = nil
	} else {
		b := make([]byte, length)
		if _, err := io.ReadFull(r, b); err != nil {
			return errors.Wrap(err, "PreviousMinerID")
		}

		key, err := bitcoin.PublicKeyFromBytes(b)
		if err != nil {
			return errors.Wrap(err, "parse PreviousMinerID")
		}
		mid.PreviousMinerID = &key
	}

	// PreviousMinerIDSig
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return errors.Wrap(err, "PreviousMinerIDSig length")
	}

	if length == 0 {
		mid.PreviousMinerIDSig = nil
	} else {
		b := make([]byte, length)
		if _, err := io.ReadFull(r, b); err != nil {
			return errors.Wrap(err, "PreviousMinerIDSig")
		}

		sig, err := bitcoin.SignatureFromBytes(b)
		if err != nil {
			return errors.Wrap(err, "parse PreviousMinerIDSig")
		}
		mid.PreviousMinerIDSig = &sig
	}

	// DynamicMinerID
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return errors.Wrap(err, "DynamicMinerID length")
	}

	if length == 0 {
		mid.DynamicMinerID = nil
	} else {
		b := make([]byte, length)
		if _, err := io.ReadFull(r, b); err != nil {
			return errors.Wrap(err, "DynamicMinerID")
		}

		key, err := bitcoin.PublicKeyFromBytes(b)
		if err != nil {
			return errors.Wrap(err, "parse DynamicMinerID")
		}
		mid.DynamicMinerID = &key
	}

	// MinerID
	b = make([]byte, bitcoin.PublicKeyCompressedLength)
	if _, err := io.ReadFull(r, b); err != nil {
		return errors.Wrap(err, "MinerID")
	}

	key, err := bitcoin.PublicKeyFromBytes(b)
	if err != nil {
		return errors.Wrap(err, "parse MinerID")
	}
	mid.MinerID = key

	// ValidityCheckTx
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return errors.Wrap(err, "ValidityCheckTx exists")
	}

	if length == 0 {
		mid.ValidityCheckTx = nil
	} else {
		mid.ValidityCheckTx = &MinerIDValidityCheckTx{}

		// TxID
		b = make([]byte, bitcoin.Hash32Size)
		if _, err := io.ReadFull(r, b); err != nil {
			return errors.Wrap(err, "ValidityCheckTx.TxID")
		}

		txid, err := bitcoin.NewHash32(b)
		if err != nil {
			return errors.Wrap(err, "parse ValidityCheckTx.TxID")
		}
		mid.ValidityCheckTx.TxID = *txid

		// Index
		if err := binary.Read(r, binary.LittleEndian, &mid.ValidityCheckTx.Index); err != nil {
			return errors.Wrap(err, "ValidityCheckTx.Index")
		}
	}

	// Contact
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return errors.Wrap(err, "Contact exists")
	}

	if length == 0 {
		mid.Contact = nil
	} else {
		mid.Contact = &MinerIDContact{}

		// Name
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return errors.Wrap(err, "Contact.Name length")
		}

		if length == 0 {
			mid.Contact.Name = nil
		} else {
			b := make([]byte, length)
			if _, err := io.ReadFull(r, b); err != nil {
				return errors.Wrap(err, "Contact.Name")
			}

			s := string(b)
			mid.Contact.Name = &s
		}

		// MerchantAPIEndpoint
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return errors.Wrap(err, "Contact.MerchantAPIEndpoint length")
		}

		if length == 0 {
			mid.Contact.MerchantAPIEndpoint = nil
		} else {
			b := make([]byte, length)
			if _, err := io.ReadFull(r, b); err != nil {
				return errors.Wrap(err, "Contact.MerchantAPIEndpoint")
			}

			s := string(b)
			mid.Contact.MerchantAPIEndpoint = &s
		}
	}

	return nil
}

func ParseMinerIDFromScript(script []byte) (*MinerID, error) {
	if len(script) < 7 {
		return nil, ErrNotMinerID
	}

	buf := bytes.NewReader(script)

	b, err := buf.ReadByte()
	if err != nil {
		return nil, errors.Wrap(err, "read op return")
	}

	if b != bitcoin.OP_RETURN {
		if b != bitcoin.OP_FALSE {
			return nil, errors.Wrap(ErrNotMinerID, "not OP_FALSE")
		}

		b, err = buf.ReadByte()
		if err != nil {
			return nil, errors.Wrap(err, "read op return")
		}

		if b != bitcoin.OP_RETURN {
			return nil, errors.Wrap(ErrNotMinerID, "not OP_RETURN")
		}
	}

	// Protocol ID
	_, protocolIDBytes, err := bitcoin.ParsePushDataScript(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parse protocol ID")
	}
	if len(protocolIDBytes) != 4 {
		return nil, errors.Wrapf(ErrNotMinerID, "protocol id size %d", len(protocolIDBytes))
	}

	protocolID := binary.BigEndian.Uint32(protocolIDBytes)
	if protocolID != MinerIDProtocolID {
		return nil, errors.Wrap(ErrNotMinerID, "wrong protocol id")
	}

	_, documentBytes, err := bitcoin.ParsePushDataScript(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parse static document")
	}

	var result MinerID
	if err := json.Unmarshal(documentBytes, &result); err != nil {
		return nil, errors.Wrap(err, "Unmarshal static document")
	}

	_, documentSigBytes, err := bitcoin.ParsePushDataScript(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parse static document signature")
	}

	documentSig, err := bitcoin.SignatureFromBytes(documentSigBytes)
	if err != nil {
		return nil, errors.Wrap(err, "static document signature")
	}

	hash := sha256.Sum256(documentBytes)
	if !documentSig.Verify(hash[:], result.MinerID) {
		return nil, errors.Wrap(ErrInvalidMinerIDSignature, "miner id")
	}

	if err := result.VerifyPrevious(); err != nil {
		return nil, err
	}

	// There is also an optional dynamic document.

	return &result, nil
}

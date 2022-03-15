package bitcoin

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type UTXO struct {
	Hash          Hash32 `db:"hash" json:"hash"`
	Index         uint32 `db:"index" json:"index"`
	Value         uint64 `db:"value" json:"value"`
	LockingScript Script `db:"locking_script" json:"locking_script"`

	// Optional identifier for external use to track the key needed to spend the UTXO.
	KeyID string `json:"key_id,omitempty"`
}

func (u UTXO) ID() string {
	return fmt.Sprintf("%s:%d", u.Hash.String(), u.Index)
}

func (u UTXO) Address() (RawAddress, error) {
	return RawAddressFromLockingScript(u.LockingScript)
}

func (utxo UTXO) Equal(other UTXO) bool {
	if !utxo.Hash.Equal(&other.Hash) {
		return false
	}

	if !bytes.Equal(utxo.LockingScript, other.LockingScript) {
		return false
	}

	if utxo.Index != other.Index {
		return false
	}

	if utxo.Value != other.Value {
		return false
	}

	return true
}

func (utxo UTXO) Write(w io.Writer) error {
	if err := utxo.Hash.Serialize(w); err != nil {
		return err
	}

	scriptSize := uint32(len(utxo.LockingScript))
	if err := binary.Write(w, binary.LittleEndian, &scriptSize); err != nil {
		return err
	}

	if _, err := w.Write(utxo.LockingScript); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, &utxo.Index); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, &utxo.Value); err != nil {
		return err
	}

	return nil
}

func (utxo *UTXO) Read(r io.Reader) error {
	hash, err := DeserializeHash32(r)
	if err != nil {
		return err
	}
	utxo.Hash = *hash

	var scriptSize uint32
	if err := binary.Read(r, binary.LittleEndian, &scriptSize); err != nil {
		return err
	}

	utxo.LockingScript = make([]byte, int(scriptSize))
	if _, err := io.ReadFull(r, utxo.LockingScript); err != nil {
		return err
	}

	if err := binary.Read(r, binary.LittleEndian, &utxo.Index); err != nil {
		return err
	}

	if err := binary.Read(r, binary.LittleEndian, &utxo.Value); err != nil {
		return err
	}

	return nil
}

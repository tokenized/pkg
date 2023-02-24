// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

const (
	// TxVersion is the current latest supported transaction version.
	TxVersion = 1

	// MaxTxInSequenceNum is the maximum sequence number the sequence field
	// of a transaction input can be.
	MaxTxInSequenceNum uint32 = 0xffffffff

	// MaxPrevOutIndex is the maximum index the index field of a previous
	// outpoint can be.
	MaxPrevOutIndex uint32 = 0xffffffff

	// SequenceLockTimeDisabled is a flag that if set on a transaction
	// input's sequence number, the sequence number will not be interpreted
	// as a relative locktime.
	SequenceLockTimeDisabled = 1 << 31

	// SequenceLockTimeIsSeconds is a flag that if set on a transaction
	// input's sequence number, the relative locktime has units of 512
	// seconds.
	SequenceLockTimeIsSeconds = 1 << 22

	// SequenceLockTimeMask is a mask that extracts the relative locktime
	// when masked against the transaction input sequence number.
	SequenceLockTimeMask = 0x0000ffff

	// SequenceLockTimeGranularity is the defined time based granularity
	// for seconds-based relative time locks. When converting from seconds
	// to a sequence number, the value is right shifted by this amount,
	// therefore the granularity of relative time locks in 512 or 2^9
	// seconds. Enforced relative lock times are multiples of 512 seconds.
	SequenceLockTimeGranularity = 9

	// defaultTxInOutAlloc is the default size used for the backing array for
	// transaction inputs and outputs.  The array will dynamically grow as needed,
	// but this figure is intended to provide enough space for the number of
	// inputs and outputs in a typical transaction without needing to grow the
	// backing array multiple times.
	defaultTxInOutAlloc = 15

	// minTxInPayload is the minimum payload size for a transaction input.
	// PreviousOutPoint.Hash + PreviousOutPoint.Index 4 bytes + Varint for
	// UnlockingScript length 1 byte + Sequence 4 bytes.
	minTxInPayload = 9 + bitcoin.Hash32Size

	// maxTxInPerMessage is the maximum number of transactions inputs that
	// a transaction which fits into a message could possibly have.
	maxTxInPerMessage = (MaxMessagePayload / minTxInPayload) + 1

	// minTxOutPayload is the minimum payload size for a transaction output.
	// Value 8 bytes + Varint for LockingScript length 1 byte.
	minTxOutPayload = 9

	// maxTxOutPerMessage is the maximum number of transactions outputs that
	// a transaction which fits into a message could possibly have.
	maxTxOutPerMessage = (MaxMessagePayload / minTxOutPayload) + 1

	// minTxPayload is the minimum payload size for a transaction.  Note
	// that any realistically usable transaction must have at least one
	// input or output, but that is a rule enforced at a higher layer, so
	// it is intentionally not included here.
	// Version 4 bytes + Varint number of transaction inputs 1 byte + Varint
	// number of transaction outputs 1 byte + LockTime 4 bytes + min input
	// payload + min output payload.
	minTxPayload = 10

	// freeListMaxScriptSize is the size of each buffer in the free list
	// that	is used for deserializing scripts from the wire before they are
	// concatenated into a single contiguous buffers.  This value was chosen
	// because it is slightly more than twice the size of the vast majority
	// of all "standard" scripts.  Larger scripts are still deserialized
	// properly as the free list will simply be bypassed for them.
	freeListMaxScriptSize = 512

	// freeListMaxItems is the number of buffers to keep in the free list
	// to use for script deserialization.  This value allows up to 100
	// scripts per transaction being simultaneously deserialized by 125
	// peers.  Thus, the peak usage of the free list is 12,500 * 512 =
	// 6,400,000 bytes.
	freeListMaxItems = 12500
)

// scriptFreeList defines a free list of byte slices (up to the maximum number
// defined by the freeListMaxItems constant) that have a cap according to the
// freeListMaxScriptSize constant.  It is used to provide temporary buffers for
// deserializing scripts in order to greatly reduce the number of allocations
// required.
//
// The caller can obtain a buffer from the free list by calling the Borrow
// function and should return it via the Return function when done using it.
type scriptFreeList chan []byte

// Borrow returns a byte slice from the free list with a length according the
// provided size.  A new buffer is allocated if there are any items available.
//
// When the size is larger than the max size allowed for items on the free list
// a new buffer of the appropriate size is allocated and returned.  It is safe
// to attempt to return said buffer via the Return function as it will be
// ignored and allowed to go the garbage collector.
func (c scriptFreeList) Borrow(size uint64) []byte {
	if size > freeListMaxScriptSize {
		return make([]byte, size)
	}

	var buf []byte
	select {
	case buf = <-c:
	default:
		buf = make([]byte, freeListMaxScriptSize)
	}
	return buf[:size]
}

// Return puts the provided byte slice back on the free list when it has a cap
// of the expected length.  The buffer is expected to have been obtained via
// the Borrow function.  Any slices that are not of the appropriate size, such
// as those whose size is greater than the largest allowed free list item size
// are simply ignored so they can go to the garbage collector.
func (c scriptFreeList) Return(buf []byte) {
	// Ignore any buffers returned that aren't the expected size for the
	// free list.
	if cap(buf) != freeListMaxScriptSize {
		return
	}

	// Return the buffer to the free list when it's not full.  Otherwise let
	// it be garbage collected.
	select {
	case c <- buf:
	default:
		// Let it go to the garbage collector.
	}
}

// Create the concurrent safe free list to use for script deserialization.  As
// previously described, this free list is maintained to significantly reduce
// the number of allocations.
var scriptPool scriptFreeList = make(chan []byte, freeListMaxItems)

// OutPoint defines a bitcoin data type that is used to track previous
// transaction outputs.
type OutPoint struct {
	Hash  bitcoin.Hash32 `json:"hash"`
	Index uint32         `json:"index"`
}

// NewOutPoint returns a new bitcoin transaction outpoint point with the
// provided hash and index.
func NewOutPoint(hash *bitcoin.Hash32, index uint32) *OutPoint {
	return &OutPoint{
		Hash:  *hash,
		Index: index,
	}
}

// OutPointFromStr parses a string into an outpoint. The format is "<txid:index>".
func OutPointFromStr(s string) (*OutPoint, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return nil, errors.New("Invalid format: wrong colon count")
	}

	hash, err := bitcoin.NewHash32FromStr(parts[0])
	if err != nil {
		return nil, errors.Wrap(err, "invalid hash")
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, errors.Wrap(err, "invalid index")
	}

	return NewOutPoint(hash, uint32(index)), nil
}

// String returns the OutPoint in the human-readable form "hash:index".
func (o OutPoint) String() string {
	// Allocate enough for hash string, colon, and 10 digits.  Although
	// at the time of writing, the number of digits can be no greater than
	// the length of the decimal representation of maxTxOutPerMessage, the
	// maximum message payload may increase in the future and this
	// optimization may go unnoticed, so allocate space for 10 decimal
	// digits, which will fit any uint32.
	buf := make([]byte, 2*bitcoin.Hash32Size+1, 2*bitcoin.Hash32Size+1+10)
	copy(buf, o.Hash.String())
	buf[2*bitcoin.Hash32Size] = ':'
	buf = strconv.AppendUint(buf, uint64(o.Index), 10)
	return string(buf)
}

// OutpointHash generates the Hash for the transaction.
func (op OutPoint) OutpointHash() *bitcoin.Hash32 {
	buf := bytes.NewBuffer(make([]byte, 0, bitcoin.Hash32Size+4))
	buf.Write(op.Hash[:])
	binary.Write(buf, endian, uint32(op.Index))
	result, _ := bitcoin.NewHash32(bitcoin.Sha256(buf.Bytes()))
	return result
}

// Serialize encodes op to the bitcoin protocol encoding for an OutPoint to w.
func (op *OutPoint) Serialize(w io.Writer) error {
	if err := op.Hash.Serialize(w); err != nil {
		return err
	}

	return binary.Write(w, endian, op.Index)
}

// Deserialize decodes op from the bitcoin protocol encoding for an OutPoint.
func (op *OutPoint) Deserialize(r io.Reader) error {
	if err := op.Hash.Deserialize(r); err != nil {
		return err
	}

	return binary.Read(r, endian, &op.Index)
}

// TxIn defines a bitcoin transaction input.
type TxIn struct {
	PreviousOutPoint OutPoint       `json:"outpoint"`
	UnlockingScript  bitcoin.Script `json:"script"`
	Sequence         uint32         `json:"sequence"`
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction input.
func (t *TxIn) SerializeSize() int {
	// Outpoint Hash 32 bytes + Outpoint Index 4 bytes + Sequence 4 bytes +
	// serialized varint size for the length of UnlockingScript +
	// UnlockingScript bytes.
	return 40 + VarIntSerializeSize(uint64(len(t.UnlockingScript))) +
		len(t.UnlockingScript)
}

// NewTxIn returns a new bitcoin transaction input with the provided
// previous outpoint point and signature script with a default sequence of
// MaxTxInSequenceNum.
func NewTxIn(prevOut *OutPoint, unlockingScript bitcoin.Script) *TxIn {
	return &TxIn{
		PreviousOutPoint: *prevOut,
		UnlockingScript:  unlockingScript,
		Sequence:         MaxTxInSequenceNum,
	}
}

// TxOut defines a bitcoin transaction output.
type TxOut struct {
	Value         uint64         `json:"value"`
	LockingScript bitcoin.Script `json:"locking_script"`
}

// Serialize encodes to into the bitcoin protocol encoding for a transaction
// output (TxOut) to w.
func (t *TxOut) Serialize(w io.Writer, pver uint32, version int32) error {
	return writeTxOut(w, pver, version, t)
}

// Deserialize decodes t from the bitcoin protocol encoding for a TxOut.
func (t *TxOut) Deserialize(r io.Reader, pver uint32, version int32) error {
	return readTxOut(r, pver, version, t)
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction output.
func (t *TxOut) SerializeSize() int {
	// Value 8 bytes + serialized varint size for the length of LockingScript +
	// LockingScript bytes.
	return 8 + VarIntSerializeSize(uint64(len(t.LockingScript))) + len(t.LockingScript)
}

// MarshalText implements encoding.TextMarshaler for json and other text encoding packages.
func (t TxOut) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	if err := t.Serialize(&buf, 0, 1); err != nil {
		return nil, errors.Wrap(err, "serialize txout")
	}

	return []byte(hex.EncodeToString(buf.Bytes())), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for json and other text encoding packages.
func (t *TxOut) UnmarshalText(text []byte) error {
	b, err := hex.DecodeString(string(text))
	if err != nil {
		return errors.Wrap(err, "decode hex")
	}

	if err := t.Deserialize(bytes.NewReader(b), 0, 1); err != nil {
		return errors.Wrap(err, "deserialize txout")
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler for binary encoding packages.
func (t TxOut) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if err := t.Serialize(&buf, 0, 1); err != nil {
		return nil, errors.Wrap(err, "serialize txout")
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for binary encoding packages.
func (t *TxOut) UnmarshalBinary(b []byte) error {
	if err := t.Deserialize(bytes.NewReader(b), 0, 1); err != nil {
		return errors.Wrap(err, "deserialize txout")
	}

	return nil
}

// NewTxOut returns a new bitcoin transaction output with the provided
// transaction value and public key script.
func NewTxOut(value uint64, lockingScript bitcoin.Script) *TxOut {
	return &TxOut{
		Value:         value,
		LockingScript: lockingScript,
	}
}

// MsgTx implements the Message interface and represents a bitcoin tx message.
// It is used to deliver transaction information in response to a getdata
// message (MsgGetData) for a given transaction.
//
// Use the AddTxIn and AddTxOut functions to build up the list of transaction
// inputs and outputs.
type MsgTx struct {
	Version  int32
	TxIn     []*TxIn
	TxOut    []*TxOut
	LockTime uint32
}

// AddTxIn adds a transaction input to the message.
func (msg *MsgTx) AddTxIn(ti *TxIn) {
	msg.TxIn = append(msg.TxIn, ti)
}

// AddTxOut adds a transaction output to the message.
func (msg *MsgTx) AddTxOut(to *TxOut) {
	msg.TxOut = append(msg.TxOut, to)
}

// TxHash generates the Hash for the transaction.
func (msg *MsgTx) TxHash() *bitcoin.Hash32 {
	// Encode the transaction and calculate double sha256 on the result.
	// Ignore the error returns since the only way the encode could fail
	// is being out of memory or due to nil pointers, both of which would
	// cause a run-time panic.
	hasher := sha256.New()
	_ = msg.Serialize(hasher)
	result := bitcoin.Hash32(sha256.Sum256(hasher.Sum(nil)))
	return &result
}

// UnsignedTxHash generates the Hash for the transaction excluding unlocking scripts.
func (msg *MsgTx) UnsignedTxHash() *bitcoin.Hash32 {
	// Encode the transaction and calculate double sha256 on the result.
	// Ignore the error returns since the only way the encode could fail
	// is being out of memory or due to nil pointers, both of which would
	// cause a run-time panic.
	hasher := sha256.New()
	_ = msg.SerializeUnsigned(hasher)
	result := bitcoin.Hash32(sha256.Sum256(hasher.Sum(nil)))
	return &result
}

func (msg *MsgTx) String() string {
	result := fmt.Sprintf("TxId: %s (%d bytes)\n", msg.TxHash(), msg.SerializeSize())
	result += fmt.Sprintf("  Version: %d\n", msg.Version)
	result += "  Inputs:\n\n"
	for _, input := range msg.TxIn {
		result += fmt.Sprintf("    Outpoint: %d - %s\n", input.PreviousOutPoint.Index,
			input.PreviousOutPoint.Hash.String())
		result += fmt.Sprintf("    Script: %s\n", input.UnlockingScript)
		result += fmt.Sprintf("    Sequence: %x\n\n", input.Sequence)
	}
	result += "  Outputs:\n\n"
	for _, output := range msg.TxOut {
		result += fmt.Sprintf("    Value: %.08f\n", float32(output.Value)/100000000.0)
		result += fmt.Sprintf("    Script: %s\n\n", output.LockingScript)
	}
	result += fmt.Sprintf("  LockTime: %d\n", msg.LockTime)
	return result
}

func (msg *MsgTx) StringWithAddresses(net bitcoin.Network) string {
	result := fmt.Sprintf("TxId: %s\n", msg.TxHash())
	result += fmt.Sprintf("  Version: %d\n", msg.Version)
	result += "  Inputs:\n\n"
	for _, input := range msg.TxIn {
		result += fmt.Sprintf("    Outpoint: %d - %s\n", input.PreviousOutPoint.Index,
			input.PreviousOutPoint.Hash)
		result += fmt.Sprintf("    Script: %s\n", input.UnlockingScript)
		result += fmt.Sprintf("    Sequence: %x\n", input.Sequence)

		// Address
		ra, err := bitcoin.RawAddressFromUnlockingScript(input.UnlockingScript)
		if err == nil {
			ad := bitcoin.NewAddressFromRawAddress(ra, net)
			result += fmt.Sprintf("    Address: %s\n", ad)
		}

		result += "\n"
	}
	result += "  Outputs:\n\n"
	for _, output := range msg.TxOut {
		result += fmt.Sprintf("    Value: %.08f\n", float32(output.Value)/100000000.0)
		result += fmt.Sprintf("    Script: %s\n", output.LockingScript)

		// Address
		ra, err := bitcoin.RawAddressFromLockingScript(output.LockingScript)
		if err == nil {
			ad := bitcoin.NewAddressFromRawAddress(ra, net)
			result += fmt.Sprintf("    Address: %s\n", ad)
		}

		result += "\n"
	}
	result += fmt.Sprintf("  LockTime: %d\n", msg.LockTime)
	return result
}

// Copy creates a deep copy of a transaction so that the original does not get
// modified when the copy is manipulated.
func (msg *MsgTx) Copy() *MsgTx {
	// Create new tx and start by copying primitive values and making space
	// for the transaction inputs and outputs.
	newTx := MsgTx{
		Version:  msg.Version,
		TxIn:     make([]*TxIn, 0, len(msg.TxIn)),
		TxOut:    make([]*TxOut, 0, len(msg.TxOut)),
		LockTime: msg.LockTime,
	}

	// Deep copy the old TxIn data.
	for _, oldTxIn := range msg.TxIn {
		// Deep copy the old previous outpoint.
		oldOutPoint := oldTxIn.PreviousOutPoint
		newOutPoint := OutPoint{}
		newOutPoint.Hash.SetBytes(oldOutPoint.Hash[:])
		newOutPoint.Index = oldOutPoint.Index

		// Deep copy the old signature script.
		var newScript []byte
		oldScript := oldTxIn.UnlockingScript
		oldScriptLen := len(oldScript)
		if oldScriptLen > 0 {
			newScript = make([]byte, oldScriptLen)
			copy(newScript, oldScript[:oldScriptLen])
		}

		// Create new txIn with the deep copied data and append it to
		// new Tx.
		newTxIn := TxIn{
			PreviousOutPoint: newOutPoint,
			UnlockingScript:  newScript,
			Sequence:         oldTxIn.Sequence,
		}
		newTx.TxIn = append(newTx.TxIn, &newTxIn)
	}

	// Deep copy the old TxOut data.
	for _, oldTxOut := range msg.TxOut {
		// Deep copy the old LockingScript
		var newScript []byte
		oldScript := oldTxOut.LockingScript
		oldScriptLen := len(oldScript)
		if oldScriptLen > 0 {
			newScript = make([]byte, oldScriptLen)
			copy(newScript, oldScript[:oldScriptLen])
		}

		// Create new txOut with the deep copied data and append it to
		// new Tx.
		newTxOut := TxOut{
			Value:         oldTxOut.Value,
			LockingScript: newScript,
		}
		newTx.TxOut = append(newTx.TxOut, &newTxOut)
	}

	return &newTx
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding transactions stored to disk, such as in a
// database, as opposed to decoding transactions from the wire.
func (msg *MsgTx) BtcDecode(r io.Reader, pver uint32) error {
	var version int32
	err := binary.Read(r, endian, &version)
	if err != nil {
		return err
	}
	msg.Version = int32(version)

	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Prevent more input transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(maxTxInPerMessage) {
		str := fmt.Sprintf("too many input transactions to fit into "+
			"max message size [count %d, max %d]", count,
			maxTxInPerMessage)
		return messageError("MsgTx.BtcDecode", str)
	}

	// returnScriptBuffers is a closure that returns any script buffers that
	// were borrowed from the pool when there are any deserialization
	// errors.  This is only valid to call before the final step which
	// replaces the scripts with the location in a contiguous buffer and
	// returns them.
	returnScriptBuffers := func() {
		for _, txIn := range msg.TxIn {
			if txIn == nil || txIn.UnlockingScript == nil {
				continue
			}
			scriptPool.Return(txIn.UnlockingScript)
		}
		for _, txOut := range msg.TxOut {
			if txOut == nil || txOut.LockingScript == nil {
				continue
			}
			scriptPool.Return(txOut.LockingScript)
		}
	}

	// Deserialize the inputs.
	var totalScriptSize uint64
	txIns := make([]TxIn, count)
	msg.TxIn = make([]*TxIn, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		ti := &txIns[i]
		msg.TxIn[i] = ti
		err = readTxIn(r, pver, msg.Version, ti)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		totalScriptSize += uint64(len(ti.UnlockingScript))
	}

	count, err = ReadVarInt(r, pver)
	if err != nil {
		returnScriptBuffers()
		return err
	}

	// Prevent more output transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(maxTxOutPerMessage) {
		returnScriptBuffers()
		str := fmt.Sprintf("too many output transactions to fit into "+
			"max message size [count %d, max %d]", count,
			maxTxOutPerMessage)
		return messageError("MsgTx.BtcDecode", str)
	}

	// Deserialize the outputs.
	txOuts := make([]TxOut, count)
	msg.TxOut = make([]*TxOut, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		to := &txOuts[i]
		msg.TxOut[i] = to
		err = readTxOut(r, pver, msg.Version, to)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		totalScriptSize += uint64(len(to.LockingScript))
	}

	err = binary.Read(r, endian, &msg.LockTime)
	if err != nil {
		returnScriptBuffers()
		return err
	}

	// Create a single allocation to house all of the scripts and set each
	// input signature script and output public key script to the
	// appropriate subslice of the overall contiguous buffer.  Then, return
	// each individual script buffer back to the pool so they can be reused
	// for future deserializations.  This is done because it significantly
	// reduces the number of allocations the garbage collector needs to
	// track, which in turn improves performance and drastically reduces the
	// amount of runtime overhead that would otherwise be needed to keep
	// track of millions of small allocations.
	//
	// NOTE: It is no longer valid to call the returnScriptBuffers closure
	// after these blocks of code run because it is already done and the
	// scripts in the transaction inputs and outputs no longer point to the
	// buffers.
	var offset uint64
	scripts := make([]byte, totalScriptSize)
	for i := 0; i < len(msg.TxIn); i++ {
		// Copy the signature script into the contiguous buffer at the
		// appropriate offset.
		signatureScript := msg.TxIn[i].UnlockingScript
		copy(scripts[offset:], signatureScript)

		// Reset the signature script of the transaction input to the
		// slice of the contiguous buffer where the script lives.
		scriptSize := uint64(len(signatureScript))
		end := offset + scriptSize
		msg.TxIn[i].UnlockingScript = scripts[offset:end:end]
		offset += scriptSize

		// Return the temporary script buffer to the pool.
		scriptPool.Return(signatureScript)
	}
	for i := 0; i < len(msg.TxOut); i++ {
		// Copy the public key script into the contiguous buffer at the
		// appropriate offset.
		pkScript := msg.TxOut[i].LockingScript
		copy(scripts[offset:], pkScript)

		// Reset the public key script of the transaction output to the
		// slice of the contiguous buffer where the script lives.
		scriptSize := uint64(len(pkScript))
		end := offset + scriptSize
		msg.TxOut[i].LockingScript = scripts[offset:end:end]
		offset += scriptSize

		// Return the temporary script buffer to the pool.
		scriptPool.Return(pkScript)
	}

	return nil
}

// Deserialize decodes a transaction from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field in the transaction.  This function differs from BtcDecode
// in that BtcDecode decodes from the bitcoin wire protocol as it was sent
// across the network.  The wire encoding can technically differ depending on
// the protocol version and doesn't even really need to match the format of a
// stored transaction at all.  As of the time this comment was written, the
// encoded transaction is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgTx) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of BtcDecode.
	return msg.BtcDecode(r, 0)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding transactions to be stored to disk, such as in a
// database, as opposed to encoding transactions for the wire.
func (msg *MsgTx) BtcEncode(w io.Writer, pver uint32) error {
	err := binary.Write(w, endian, uint32(msg.Version))
	if err != nil {
		return err
	}

	count := uint64(len(msg.TxIn))
	err = WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, ti := range msg.TxIn {
		err = writeTxIn(w, pver, msg.Version, ti)
		if err != nil {
			return err
		}
	}

	count = uint64(len(msg.TxOut))
	err = WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, to := range msg.TxOut {
		err = writeTxOut(w, pver, msg.Version, to)
		if err != nil {
			return err
		}
	}

	return binary.Write(w, endian, uint32(msg.LockTime))
}

// Serialize encodes the transaction to w using a format that suitable for
// long-term storage such as a database while respecting the Version field in
// the transaction.  This function differs from BtcEncode in that BtcEncode
// encodes the transaction to the bitcoin wire protocol in order to be sent
// across the network.  The wire encoding can technically differ depending on
// the protocol version and doesn't even really need to match the format of a
// stored transaction at all.  As of the time this comment was written, the
// encoded transaction is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgTx) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of BtcEncode.
	return msg.BtcEncode(w, 0)
}

func (msg *MsgTx) BtcEncodeUnsigned(w io.Writer, pver uint32) error {
	err := binary.Write(w, endian, uint32(msg.Version))
	if err != nil {
		return err
	}

	count := uint64(len(msg.TxIn))
	err = WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, ti := range msg.TxIn {
		err = writeTxInUnsigned(w, pver, msg.Version, ti)
		if err != nil {
			return err
		}
	}

	count = uint64(len(msg.TxOut))
	err = WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, to := range msg.TxOut {
		err = writeTxOut(w, pver, msg.Version, to)
		if err != nil {
			return err
		}
	}

	return binary.Write(w, endian, uint32(msg.LockTime))
}

func (msg *MsgTx) SerializeUnsigned(w io.Writer) error {
	return msg.BtcEncodeUnsigned(w, 0)
}

// SerializeSize returns the number of bytes it would take to serialize the
// the transaction.
func (msg *MsgTx) SerializeSize() int {
	// Version 4 bytes + LockTime 4 bytes + Serialized varint size for the
	// number of transaction inputs and outputs.
	n := 8 + VarIntSerializeSize(uint64(len(msg.TxIn))) +
		VarIntSerializeSize(uint64(len(msg.TxOut)))

	for _, txIn := range msg.TxIn {
		n += txIn.SerializeSize()
	}

	for _, txOut := range msg.TxOut {
		n += txOut.SerializeSize()
	}

	return n
}

// MarshalText implements encoding.TextMarshaler for json and other text encoding packages.
func (msg MsgTx) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	if err := msg.Serialize(&buf); err != nil {
		return nil, errors.Wrap(err, "serialize tx")
	}

	return []byte(hex.EncodeToString(buf.Bytes())), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for json and other text encoding packages.
func (msg *MsgTx) UnmarshalText(text []byte) error {
	b, err := hex.DecodeString(string(text))
	if err != nil {
		return errors.Wrap(err, "decode hex")
	}

	if err := msg.Deserialize(bytes.NewReader(b)); err != nil {
		return errors.Wrap(err, "deserialize tx")
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler for binary encoding packages.
func (msg MsgTx) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if err := msg.Serialize(&buf); err != nil {
		return nil, errors.Wrap(err, "serialize tx")
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for binary encoding packages.
func (msg *MsgTx) UnmarshalBinary(b []byte) error {
	if err := msg.Deserialize(bytes.NewReader(b)); err != nil {
		return errors.Wrap(err, "deserialize tx")
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgTx) Command() string {
	return CmdTx
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgTx) MaxPayloadLength(pver uint32) uint64 {
	return MaxBlockPayload
}

// LockingScriptLocs returns a slice containing the start of each public key script
// within the raw serialized transaction.  The caller can easily obtain the
// length of each script by using len on the script available via the
// appropriate transaction output entry.
func (msg *MsgTx) LockingScriptLocs() []int {
	numTxOut := len(msg.TxOut)
	if numTxOut == 0 {
		return nil
	}

	// The starting offset in the serialized transaction of the first
	// transaction output is:
	//
	// Version 4 bytes + serialized varint size for the number of
	// transaction inputs and outputs + serialized size of each transaction
	// input.
	n := 4 + VarIntSerializeSize(uint64(len(msg.TxIn))) +
		VarIntSerializeSize(uint64(numTxOut))
	for _, txIn := range msg.TxIn {
		n += txIn.SerializeSize()
	}

	// Calculate and set the appropriate offset for each public key script.
	pkScriptLocs := make([]int, numTxOut)
	for i, txOut := range msg.TxOut {
		// The offset of the script in the transaction output is:
		//
		// Value 8 bytes + serialized varint size for the length of
		// LockingScript.
		n += 8 + VarIntSerializeSize(uint64(len(txOut.LockingScript)))
		pkScriptLocs[i] = n
		n += len(txOut.LockingScript)
	}

	return pkScriptLocs
}

// NewMsgTx returns a new bitcoin tx message that conforms to the Message
// interface.  The return instance has a default version of TxVersion and there
// are no transaction inputs or outputs.  Also, the lock time is set to zero
// to indicate the transaction is valid immediately as opposed to some time in
// future.
func NewMsgTx(version int32) *MsgTx {
	return &MsgTx{
		Version: version,
		TxIn:    make([]*TxIn, 0, defaultTxInOutAlloc),
		TxOut:   make([]*TxOut, 0, defaultTxInOutAlloc),
	}
}

// readOutPoint reads the next sequence of bytes from r as an OutPoint.
func readOutPoint(r io.Reader, pver uint32, version int32, op *OutPoint) error {
	_, err := io.ReadFull(r, op.Hash[:])
	if err != nil {
		return err
	}

	err = binary.Read(r, endian, &op.Index)
	return err
}

// readScript reads a variable length byte array that represents a transaction
// script.  It is encoded as a varInt containing the length of the array
// followed by the bytes themselves.  An error is returned if the length is
// greater than the passed maxAllowed parameter which helps protect against
// memory exhuastion attacks and forced panics thorugh malformed messages.  The
// fieldName parameter is only used for the error message so it provides more
// context in the error.
func readScript(r io.Reader, pver uint32, maxAllowed uint64, fieldName string) ([]byte, error) {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return nil, err
	}

	// Prevent byte array larger than the max message size.  It would
	// be possible to cause memory exhaustion and panics without a sane
	// upper bound on this count.
	if count > maxAllowed {
		str := fmt.Sprintf("%s is larger than the max allowed size "+
			"[count %d, max %d]", fieldName, count, maxAllowed)
		return nil, messageError("readScript", str)
	}

	b := scriptPool.Borrow(count)
	_, err = io.ReadFull(r, b)
	if err != nil {
		scriptPool.Return(b)
		return nil, err
	}
	return b, nil
}

// readTxIn reads the next sequence of bytes from r as a transaction input
// (TxIn).
func readTxIn(r io.Reader, pver uint32, version int32, ti *TxIn) error {
	err := readOutPoint(r, pver, version, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	ti.UnlockingScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction input signature script")
	if err != nil {
		return err
	}

	return readElement(r, &ti.Sequence)
}

// writeTxIn encodes ti to the bitcoin protocol encoding for a transaction
// input (TxIn) to w.
func writeTxIn(w io.Writer, pver uint32, version int32, ti *TxIn) error {
	err := ti.PreviousOutPoint.Serialize(w)
	if err != nil {
		return err
	}

	err = WriteVarBytes(w, pver, ti.UnlockingScript)
	if err != nil {
		return err
	}

	return binary.Write(w, endian, uint32(ti.Sequence))
}

func writeTxInUnsigned(w io.Writer, pver uint32, version int32, ti *TxIn) error {
	err := ti.PreviousOutPoint.Serialize(w)
	if err != nil {
		return err
	}

	return binary.Write(w, endian, uint32(ti.Sequence))
}

// readTxOut reads the next sequence of bytes from r as a transaction output
// (TxOut).
func readTxOut(r io.Reader, pver uint32, version int32, to *TxOut) error {
	err := readElement(r, &to.Value)
	if err != nil {
		return err
	}

	to.LockingScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction output public key script")
	return err
}

// writeTxOut encodes to into the bitcoin protocol encoding for a transaction
// output (TxOut) to w.
func writeTxOut(w io.Writer, pver uint32, version int32, to *TxOut) error {
	err := binary.Write(w, endian, uint64(to.Value))
	if err != nil {
		return err
	}

	return WriteVarBytes(w, pver, to.LockingScript)
}

func (tx *MsgTx) Clear() {
	tx.Version = 1
	tx.TxIn = nil
	tx.TxOut = nil
	tx.LockTime = 0
}

// Scan converts from a database column.
func (tx *MsgTx) Scan(data interface{}) error {
	if data == nil {
		tx.Clear()
		return nil
	}

	b, ok := data.([]byte)
	if !ok {
		return errors.New("MsgTx db column not bytes")
	}

	if len(b) == 0 {
		tx.Clear()
		return nil
	}

	// Copy byte slice because it will be wiped out by the database after this call.
	c := make([]byte, len(b))
	copy(c, b)

	// Decode into raw address
	return tx.Deserialize(bytes.NewReader(c))
}

// Bytes returns the byte encoded format of the tx.
func (tx MsgTx) Bytes() []byte {
	buf := &bytes.Buffer{}
	tx.Serialize(buf)
	return buf.Bytes()
}

func (tx *MsgTx) TxID() bitcoin.Hash32 {
	return *tx.TxHash()
}

func (tx *MsgTx) InputCount() int {
	return len(tx.TxIn)
}

func (tx *MsgTx) Input(index int) *TxIn {
	return tx.TxIn[index]
}

func (tx *MsgTx) GetMsgTx() *MsgTx {
	return tx
}

func (tx *MsgTx) OutputCount() int {
	return len(tx.TxOut)
}

func (tx *MsgTx) Output(index int) *TxOut {
	return tx.TxOut[index]
}

// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

// MessageHeaderSize is the number of bytes in a bitcoin message header.
// Bitcoin network (magic) 4 bytes + command 12 bytes + payload length 4 bytes +
// checksum 4 bytes.
const MessageHeaderSize = 24

// CommandSize is the fixed size of all commands in the common bitcoin message
// header.  Shorter commands must be zero padded.
const CommandSize = 12

// MaxMessagePayload is the maximum bytes a message can be regardless of other
// individual limits imposed by messages themselves.
const MaxMessagePayload = 0x0000ffffffffffff

// Commands used in bitcoin message headers which describe the type of message.
const (
	CmdVersion     = "version"
	CmdVerAck      = "verack"
	CmdGetAddr     = "getaddr"
	CmdAddr        = "addr"
	CmdGetBlocks   = "getblocks"
	CmdInv         = "inv"
	CmdGetData     = "getdata"
	CmdNotFound    = "notfound"
	CmdBlock       = "block"
	CmdTx          = "tx"
	CmdGetHeaders  = "getheaders"
	CmdHeaders     = "headers"
	CmdPing        = "ping"
	CmdPong        = "pong"
	CmdAlert       = "alert"
	CmdMemPool     = "mempool"
	CmdFilterAdd   = "filteradd"
	CmdFilterClear = "filterclear"
	CmdFilterLoad  = "filterload"
	CmdMerkleBlock = "merkleblock"
	CmdReject      = "reject"
	CmdSendHeaders = "sendheaders"
	CmdFeeFilter   = "feefilter"
	CmdProtoconf   = "protoconf"
	CmdExtended    = "extmsg" // added in protocol version 70016
)

// Message is an interface that describes a bitcoin message.  A type that
// implements Message has complete control over the representation of its data
// and may therefore contain additional or fewer fields than those which
// are used directly in the protocol encoded message.
type Message interface {
	BtcDecode(io.Reader, uint32) error
	BtcEncode(io.Writer, uint32) error
	Command() string
	MaxPayloadLength(uint32) uint64
}

// MessageHeader defines the header structure for all bitcoin P2P messages.
type MessageHeader struct {
	Network  bitcoin.Network // 4 bytes
	Command  [12]byte        // 12 bytes
	Length   uint64          // only actually use 4 bytes unless it is an extended message
	Checksum [4]byte         // 4 bytes
}

func (hdr MessageHeader) CommandString() string {
	return string(bytes.TrimRight(hdr.Command[:], string(rune(0))))
}

// makeEmptyMessage creates a message of the appropriate concrete type based
// on the command.
func makeEmptyMessage(command string) (Message, error) {
	var msg Message
	switch command {
	case CmdVersion:
		msg = &MsgVersion{}

	case CmdVerAck:
		msg = &MsgVerAck{}

	case CmdGetAddr:
		msg = &MsgGetAddr{}

	case CmdAddr:
		msg = &MsgAddr{}

	case CmdGetBlocks:
		msg = &MsgGetBlocks{}

	case CmdBlock:
		msg = &MsgBlock{}

	case CmdInv:
		msg = &MsgInv{}

	case CmdGetData:
		msg = &MsgGetData{}

	case CmdNotFound:
		msg = &MsgNotFound{}

	case CmdTx:
		msg = &MsgTx{}

	case CmdPing:
		msg = &MsgPing{}

	case CmdPong:
		msg = &MsgPong{}

	case CmdGetHeaders:
		msg = &MsgGetHeaders{}

	case CmdHeaders:
		msg = &MsgHeaders{}

	case CmdAlert:
		msg = &MsgAlert{}

	case CmdMemPool:
		msg = &MsgMemPool{}

	case CmdFilterAdd:
		msg = &MsgFilterAdd{}

	case CmdFilterClear:
		msg = &MsgFilterClear{}

	case CmdFilterLoad:
		msg = &MsgFilterLoad{}

	case CmdMerkleBlock:
		msg = &MsgMerkleBlock{}

	case CmdReject:
		msg = &MsgReject{}

	case CmdSendHeaders:
		msg = &MsgSendHeaders{}

	case CmdFeeFilter:
		msg = &MsgFeeFilter{}

	case CmdExtended:
		msg = &MsgExtended{}

	default:
		return nil, fmt.Errorf("unhandled command [%s]", command)
	}
	return msg, nil
}

// messageHeader defines the header structure for all bitcoin protocol messages.
type messageHeader struct {
	magic    BitcoinNet // 4 bytes
	command  string     // 12 bytes
	length   uint32     // 4 bytes
	checksum [4]byte    // 4 bytes
}

// readMessageHeader reads a bitcoin message header from r.
func readMessageHeader(r io.Reader) (int, *messageHeader, error) {
	// Since readElements doesn't return the amount of bytes read, attempt
	// to read the entire header into a buffer first in case there is a
	// short read so the proper amount of read bytes are known.  This works
	// since the header is a fixed size.
	var headerBytes [MessageHeaderSize]byte
	n, err := io.ReadFull(r, headerBytes[:])
	if err != nil {
		// If read failed assume closed connection since net package doesn't give consistent errors.
		return n, nil, messageTypeError("ReadMessage", MessageErrorConnectionClosed,
			err.Error())
	}
	hr := bytes.NewReader(headerBytes[:])

	// Create and populate a messageHeader struct from the raw header bytes.
	hdr := messageHeader{}
	var command [CommandSize]byte
	readElements(hr, &hdr.magic, &command, &hdr.length, &hdr.checksum)

	// Strip trailing zeros from command string.
	hdr.command = string(bytes.TrimRight(command[:], string(rune(0))))

	return n, &hdr, nil
}

// discardInput reads n bytes from reader r in chunks and discards the read
// bytes.  This is used to skip payloads when various errors occur and helps
// prevent rogue nodes from causing massive memory allocation through forging
// header length.
func discardInput(r io.Reader, n uint32) {
	maxSize := uint32(10 * 1024) // 10k at a time
	numReads := n / maxSize
	bytesRemaining := n % maxSize
	if n > 0 {
		buf := make([]byte, maxSize)
		for i := uint32(0); i < numReads; i++ {
			io.ReadFull(r, buf)
		}
	}
	if bytesRemaining > 0 {
		buf := make([]byte, bytesRemaining)
		io.ReadFull(r, buf)
	}
}

// WriteMessageN writes a bitcoin Message to w including the necessary header
// information and returns the number of bytes written.    This function is the
// same as WriteMessage except it also returns the number of bytes written.
func WriteMessageN(w io.Writer, msg Message, pver uint32, btcnet BitcoinNet) (uint64, error) {
	totalBytes := uint64(0)

	// Enforce max command size.
	var command [CommandSize]byte
	cmd := msg.Command()
	if len(cmd) > CommandSize {
		str := fmt.Sprintf("command [%s] is too long [max %v]",
			cmd, CommandSize)
		return totalBytes, messageError("WriteMessage", str)
	}
	copy(command[:], []byte(cmd))

	// Encode the message payload.
	var bw bytes.Buffer
	err := msg.BtcEncode(&bw, pver)
	if err != nil {
		return totalBytes, err
	}
	payload := bw.Bytes()
	lenp := len(payload)

	// Enforce maximum overall message payload.
	if lenp > MaxMessagePayload {
		str := fmt.Sprintf("message payload is too large - encoded "+
			"%d bytes, but maximum message payload is %d bytes",
			lenp, MaxMessagePayload)
		return totalBytes, messageError("WriteMessage", str)
	}

	// Enforce maximum message payload based on the message type.
	mpl := msg.MaxPayloadLength(pver)
	if uint64(lenp) > mpl {
		str := fmt.Sprintf("message payload is too large - encoded "+
			"%d bytes, but maximum message payload size for "+
			"messages of type [%s] is %d.", lenp, cmd, mpl)
		return totalBytes, messageError("WriteMessage", str)
	}

	// Create header for the message.
	hdr := messageHeader{}
	hdr.magic = btcnet
	if lenp >= math.MaxUint32 {
		// Convert to extended message
		hdr.command = CmdExtended
		hdr.length = math.MaxUint32
		copy(command[:], []byte(CmdExtended))

		extendedMsg := NewMsgExtended(cmd, payload)
		var extendedBuffer bytes.Buffer
		err := extendedMsg.BtcEncode(&extendedBuffer, pver)
		if err != nil {
			return totalBytes, err
		}
		payload = extendedBuffer.Bytes()

	} else {
		hdr.command = cmd
		hdr.length = uint32(lenp)
	}
	copy(hdr.checksum[:], bitcoin.DoubleSha256(payload)[0:4])

	// Encode the header for the message.  This is done to a buffer
	// rather than directly to the writer since writeElements doesn't
	// return the number of bytes written.
	hw := bytes.NewBuffer(make([]byte, 0, MessageHeaderSize))
	writeElements(hw, hdr.magic, command, hdr.length, hdr.checksum)

	// Write header.
	n, err := w.Write(hw.Bytes())
	totalBytes += uint64(n)
	if err != nil {
		return totalBytes, err
	}

	// Write payload.
	n, err = w.Write(payload)
	totalBytes += uint64(n)
	return totalBytes, err
}

// WriteMessage writes a bitcoin Message to w including the necessary header
// information.  This function is the same as WriteMessageN except it doesn't
// doesn't return the number of bytes written.  This function is mainly provided
// for backwards compatibility with the original API, but it's also useful for
// callers that don't care about byte counts.
func WriteMessage(w io.Writer, msg Message, pver uint32, btcnet BitcoinNet) error {
	_, err := WriteMessageN(w, msg, pver, btcnet)
	return err
}

// ReadMessageN reads, validates, and parses the next bitcoin Message from r for
// the provided protocol version and bitcoin network.  It returns the number of
// bytes read in addition to the parsed Message and raw bytes which comprise the
// message.  This function is the same as ReadMessage except it also returns the
// number of bytes read.
func ReadMessageN(r io.Reader, pver uint32, btcnet BitcoinNet) (uint64, Message, []byte, error) {
	totalBytes := uint64(0)
	n, hdr, err := readMessageHeader(r)
	totalBytes += uint64(n)
	if err != nil {
		return totalBytes, nil, nil, errors.Wrap(err, "read header")
	}

	// Check for messages from the wrong bitcoin network.
	if hdr.magic != btcnet {
		discardInput(r, hdr.length)
		str := fmt.Sprintf("[%v]", hdr.magic)
		return totalBytes, nil, nil, messageTypeError("ReadMessage", MessageErrorWrongNetwork, str)
	}

	// Enforce maximum message payload.
	if uint64(hdr.length) > MaxMessagePayload {
		str := fmt.Sprintf("message payload is too large - header "+
			"indicates %d bytes, but max message payload is %d "+
			"bytes.", hdr.length, MaxMessagePayload)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	// Check for malformed commands.
	command := hdr.command
	if !utf8.ValidString(command) {
		discardInput(r, hdr.length)
		return totalBytes, nil, nil, messageTypeError("ReadMessage", MessageErrorUnknownCommand,
			command)
	}

	// Create struct of appropriate message type based on the command.
	msg, err := makeEmptyMessage(command)
	if err != nil {
		discardInput(r, hdr.length)
		return totalBytes, nil, nil, messageTypeError("ReadMessage", MessageErrorUnknownCommand,
			command)
	}

	// Check for maximum length based on the message type as a malicious client
	// could otherwise create a well-formed header and set the length to max
	// numbers in order to exhaust the machine's memory.
	mpl := msg.MaxPayloadLength(pver)
	if uint64(hdr.length) > mpl {
		discardInput(r, hdr.length)
		str := fmt.Sprintf("payload exceeds max length - header "+
			"indicates %v bytes, but max payload size for "+
			"messages of type [%v] is %v.", hdr.length, command, mpl)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	// Read payload.
	payload := make([]byte, hdr.length)
	n, err = io.ReadFull(r, payload)
	totalBytes += uint64(n)
	if err != nil {
		// If read failed assume closed connection since net package doesn't give consistent errors.
		return totalBytes, nil, nil, messageTypeError("ReadMessage", MessageErrorConnectionClosed,
			err.Error())
	}

	// Test checksum.
	checksum := bitcoin.DoubleSha256(payload)[0:4]
	if !bytes.Equal(checksum, hdr.checksum[:]) {
		str := fmt.Sprintf("payload checksum failed - header "+
			"indicates %v, but actual checksum is %v.", hdr.checksum, checksum)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	// Unmarshal message.  NOTE: This must be a *bytes.Buffer since the
	// MsgVersion BtcDecode function requires it.
	pr := bytes.NewBuffer(payload)
	err = msg.BtcDecode(pr, pver)
	if err != nil {
		return totalBytes, nil, nil, err
	}

	return totalBytes, msg, payload, nil
}

// ReadMessage reads, validates, and parses the next bitcoin Message from r for
// the provided protocol version and bitcoin network.  It returns the parsed
// Message and raw bytes which comprise the message.  This function only differs
// from ReadMessageN in that it doesn't return the number of bytes read.  This
// function is mainly provided for backwards compatibility with the original
// API, but it's also useful for callers that don't care about byte counts.
func ReadMessage(r io.Reader, pver uint32, btcnet BitcoinNet) (Message, []byte, error) {
	_, msg, buf, err := ReadMessageN(r, pver, btcnet)
	return msg, buf, err
}

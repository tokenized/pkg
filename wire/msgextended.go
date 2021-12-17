package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"

	"github.com/pkg/errors"
)

// MsgExtended is a message that allows messages larger than 2^32 bytes to be sent in the P2P
// protocol. It was added in protocol version 70016.
// https://github.com/bitcoin-sv-specs/protocol/blob/master/p2p/large_messages.md
type MsgExtended struct {
	ExtCommand string
	Length     uint64
	Payload    []byte
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgExtended) BtcDecode(r io.Reader, pver uint32) error {
	var command [CommandSize]byte
	if _, err := io.ReadFull(r, command[:]); err != nil {
		// If read failed assume closed connection since net package doesn't give consistent errors.
		return messageTypeError("ReadMessage", MessageErrorConnectionClosed, err.Error())
	}
	msg.ExtCommand = string(bytes.TrimRight(command[:], string(rune(0))))

	if err := binary.Read(r, endian, &msg.Length); err != nil {
		// If read failed assume closed connection since net package doesn't give consistent errors.
		return messageTypeError("ReadMessage", MessageErrorConnectionClosed, err.Error())
	}

	msg.Payload = make([]byte, msg.Length)
	if _, err := io.ReadFull(r, msg.Payload); err != nil {
		// If read failed assume closed connection since net package doesn't give consistent errors.
		return messageTypeError("ReadMessage", MessageErrorConnectionClosed, err.Error())
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgExtended) BtcEncode(w io.Writer, pver uint32) error {
	var command [CommandSize]byte
	copy(command[:], []byte(msg.ExtCommand))

	if _, err := w.Write(command[:]); err != nil {
		return errors.Wrap(err, "command")
	}

	if err := binary.Write(w, endian, msg.Length); err != nil {
		return errors.Wrap(err, "length")
	}

	if _, err := w.Write(msg.Payload); err != nil {
		return errors.Wrap(err, "payload")
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgExtended) Command() string {
	return CmdExtended
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgExtended) MaxPayloadLength(pver uint32) uint64 {
	return math.MaxUint64
}

// NewMsgExtended returns a new bitcoin extended message that conforms to
// the Message interface.  See MsgMsgExtended for details.
func NewMsgExtended(command string, payload []byte) *MsgExtended {
	return &MsgExtended{
		ExtCommand: command,
		Length:     uint64(len(payload)),
		Payload:    payload,
	}
}

package wire

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

const (
	ProtoconfNumberOfFields = 2 // used as version
)

type MsgProtoconf struct {
	NumberOfFields          uint64
	MaxReceivePayloadLength uint32
	StreamPolicies          string
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgProtoconf) BtcDecode(r io.Reader, pver uint32) error {
	numberOfFields, err := ReadVarInt(r, pver)
	if err != nil {
		return errors.Wrap(err, "read numberOfFields")
	}
	msg.NumberOfFields = numberOfFields

	if numberOfFields == 0 {
		return messageError("MsgProtoconf.BtcDecode", "protoconf numberOfFields must not be zero")
	}

	if err := binary.Read(r, endian, &msg.MaxReceivePayloadLength); err != nil {
		return errors.Wrap(err, "max receive payload length")
	}

	if numberOfFields == 1 {
		return nil
	}

	streamPolicies, err := ReadVarString(r, pver)
	if err != nil {
		return errors.Wrap(err, "read stream policies")
	}
	msg.StreamPolicies = streamPolicies

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgProtoconf) BtcEncode(w io.Writer, pver uint32) error {
	if err := WriteVarInt(w, pver, uint64(ProtoconfNumberOfFields)); err != nil {
		return err
	}

	if err := binary.Write(w, endian, msg.MaxReceivePayloadLength); err != nil {
		return errors.Wrap(err, "max receive payload length")
	}

	if err := WriteVarString(w, pver, msg.StreamPolicies); err != nil {
		return err
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgProtoconf) Command() string {
	return CmdProtoconf
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgProtoconf) MaxPayloadLength(pver uint32) uint64 {
	return MaxMessagePayload
}

// NewMsgProtoconf returns a new bitcoin headers message that conforms to the
// Message interface.  See MsgHeaders for details.
func NewMsgProtoconf() *MsgProtoconf {
	return &MsgProtoconf{
		NumberOfFields:          uint64(ProtoconfNumberOfFields),
		MaxReceivePayloadLength: 1048576,
		StreamPolicies:          "Default",
	}
}

package bsor

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

var (
	ErrInvalidID       = errors.New("Invalid ID")
	ErrValueConversion = errors.New("Value Conversion")
)

type Element struct {
	ID    uint64 `json:"id"`
	Value *bitcoin.ScriptItem
}

func ParseElement(buf *bytes.Reader) (*Element, error) {
	idItem, err := bitcoin.ParseScript(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parse id")
	}

	id, err := bitcoin.ScriptNumberValue(idItem)
	if err != nil {
		return nil, errors.Wrap(err, "id number")
	}

	valueItem, err := bitcoin.ParseScript(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parse value")
	}

	return &Element{
		ID:    uint64(id),
		Value: valueItem,
	}, nil
}

func (element *Element) Write(buf *bytes.Buffer) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(int64(element.ID))); err != nil {
		return errors.Wrap(err, "id")
	}

	if err := element.Value.Write(buf); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func Marshal(value interface{}) (bitcoin.Script, error) {
	buf := &bytes.Buffer{}

	var fields reflect.Type
	var values reflect.Value
	if reflect.ValueOf(value).Kind() == reflect.Ptr {
		fields = reflect.TypeOf(value).Elem()
		values = reflect.ValueOf(value).Elem()
	} else {
		fields = reflect.TypeOf(value)
		values = reflect.ValueOf(value)
	}

	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		fieldValue := values.Field(i)

		if !fieldValue.CanInterface() {
			continue // not exported
		}
		iface := fieldValue.Interface()

		elementIDString := field.Tag.Get("bsor")
		if len(elementIDString) == 0 {
			continue // no BSOR ID specified
		}

		elementID, err := strconv.ParseUint(elementIDString, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidID, "%s: \"%s\"", field.Name, elementIDString)
		}

		element := &Element{
			ID: elementID,
		}

		switch field.Type.Kind() {
		case reflect.Struct:

			continue

		case reflect.Array:

			continue

		case reflect.Slice:

			continue

		case reflect.String:
			value, ok := iface.(string)
			if ok {
				if len(value) > 0 {
					if err := writeBytes(buf, elementID, []byte(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "bool", field.Name)
			}

			continue

		case reflect.Bool:
			value, ok := iface.(bool)
			if ok {
				if value {
					if err := writeOpCode(buf, elementID, bitcoin.OP_TRUE); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "bool", field.Name)
			}

			continue

		case reflect.Int:
			value, ok := iface.(int)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "int", field.Name)
			}

			continue

		case reflect.Int8:
			value, ok := iface.(int8)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "int8", field.Name)
			}

			continue

		case reflect.Int16:
			value, ok := iface.(int16)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "int16", field.Name)
			}

			continue

		case reflect.Int32:
			value, ok := iface.(int32)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "int32", field.Name)
			}

			continue

		case reflect.Int64:
			value, ok := iface.(int64)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, value); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "int64", field.Name)
			}

			continue

		case reflect.Uint:
			value, ok := iface.(uint)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "uint", field.Name)
			}

			continue

		case reflect.Uint8:
			value, ok := iface.(uint8)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "uint8", field.Name)
			}

			continue

		case reflect.Uint16:
			value, ok := iface.(uint16)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "uint16", field.Name)
			}

			continue

		case reflect.Uint32:
			value, ok := iface.(uint32)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "uint32", field.Name)
			}

			continue

		case reflect.Uint64:
			value, ok := iface.(uint64)
			if ok {
				if value != 0 {
					if err := writeInteger(buf, elementID, int64(value)); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "uint64", field.Name)
			}

			continue

		case reflect.Float32:
			value, ok := iface.(float32)
			if ok {
				if value != 0 {
					if err := writeFloat32(buf, elementID, value); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "float32", field.Name)
			}

			continue

		case reflect.Float64:
			value, ok := iface.(float64)
			if ok {
				if value != 0 {
					if err := writeFloat64(buf, elementID, value); err != nil {
						return nil, errors.Wrapf(err, "write: %s", field.Name)
					}
				}
			} else {
				return nil, errors.Wrapf(ErrValueConversion, "%s: %s", "float64", field.Name)
			}

			continue
		}

		if binaryMarshaler, ok := iface.(encoding.BinaryMarshaler); ok {
			b, err := binaryMarshaler.MarshalBinary()
			if err != nil {
				return nil, errors.Wrapf(err, "binary marshal: %s", field.Name)
			}

			element.Value = &bitcoin.ScriptItem{
				Type: bitcoin.ScriptItemTypePushData,
				Data: b,
			}

			if err := element.Write(buf); err != nil {
				return nil, errors.Wrapf(err, "write element: %s", field.Name)
			}

			continue
		}

		return nil, fmt.Errorf("Field not a supported type: %s", field.Name)
	}

	return buf.Bytes(), nil
}

func writeOpCode(buf *bytes.Buffer, id uint64, value byte) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
		return errors.Wrap(err, "id")
	}

	if _, err := buf.Write([]byte{value}); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeBytes(buf *bytes.Buffer, id uint64, value []byte) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
		return errors.Wrap(err, "id")
	}

	if err := bitcoin.WritePushDataScript(buf, value); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeInteger(buf *bytes.Buffer, id uint64, value int64) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
		return errors.Wrap(err, "id")
	}

	if _, err := buf.Write(bitcoin.PushNumberScript(value)); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeFloat32(buf *bytes.Buffer, id uint64, value float32) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
		return errors.Wrap(err, "id")
	}

	dataBuf := &bytes.Buffer{}
	if err := binary.Write(dataBuf, binary.LittleEndian, value); err != nil {
		return errors.Wrap(err, "binary")
	}

	if err := bitcoin.WritePushDataScript(buf, dataBuf.Bytes()); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeFloat64(buf *bytes.Buffer, id uint64, value float64) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
		return errors.Wrap(err, "id")
	}

	dataBuf := &bytes.Buffer{}
	if err := binary.Write(dataBuf, binary.LittleEndian, value); err != nil {
		return errors.Wrap(err, "binary")
	}

	if err := bitcoin.WritePushDataScript(buf, dataBuf.Bytes()); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

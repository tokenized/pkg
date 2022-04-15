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

func marshalObject(buf *bytes.Buffer, object interface{}) error {
	objectType := reflect.TypeOf(object)
	if objectType.Kind() != reflect.Struct {
		return fmt.Errorf("Marshal object is not a struct: %s", objectType.Name())
	}

	objectValue := reflect.ValueOf(object)
	objectFieldCount := objectType.NumField()

	var fieldCount int64 // number of fields marshalled into the script
	fieldBuf := &bytes.Buffer{}
	for i := 0; i < objectFieldCount; i++ {
		field := objectType.Field(i)
		fieldValue := objectValue.Field(i)

		if wasAdded, err := marshalField(fieldBuf, field, fieldValue); err != nil {
			return errors.Wrapf(err, "marshal field %d: %s (type %s)", i, field.Name,
				field.Type.Name())
		} else if wasAdded {
			fieldCount++
		}
	}

	if _, err := buf.Write(bitcoin.PushNumberScript(fieldCount)); err != nil {
		return errors.Wrap(err, "write field count")
	}

	if _, err := buf.Write(fieldBuf.Bytes()); err != nil {
		return errors.Wrap(err, "write fields")
	}

	return nil
}

func marshalField(buf *bytes.Buffer, field reflect.StructField,
	fieldValue reflect.Value) (bool, error) {

	if !fieldValue.CanInterface() {
		return false, nil // not exported, "private" lower case field name
	}
	iface := fieldValue.Interface()

	idString := field.Tag.Get("bsor")
	if len(idString) == 0 {
		return false, errors.Wrap(ErrInvalidID, "missing \"bsor\" tag")
	}

	if idString == "-" {
		return false, nil // this field was explicitly excluded from BSOR data
	}

	id, err := strconv.ParseUint(idString, 10, 64)
	if err != nil {
		return false, errors.Wrapf(ErrInvalidID, "bsor tag: \"%s\"", idString)
	}

	if fieldValue.IsZero() {
		return false, nil // zero value / empty field
	}

	if binaryMarshaler, ok := iface.(encoding.BinaryMarshaler); ok {
		b, err := binaryMarshaler.MarshalBinary()
		if err != nil {
			return false, errors.Wrapf(err, "binary marshal")
		}

		if len(b) == 0 {
			return false, nil
		}

		if err := writeID(buf, id); err != nil {
			return false, errors.Wrap(err, "id")
		}

		if err := writeBytes(buf, b); err != nil {
			return false, errors.Wrap(err, "bytes")
		}

		return true, nil
	}

	switch field.Type.Kind() {
	case reflect.Ptr:
		elem := fieldValue.Elem()
		if !elem.CanInterface() {
			return false, nil // not exported, "private" lower case field name
		}

		if elem.Kind() != reflect.Struct {
			return true, marshalPrimative(buf, id, elem.Kind(), elem.Interface())
		}

		if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
			return false, errors.Wrap(err, "id")
		}

		if err := marshalObject(buf, elem.Interface()); err != nil {
			return false, errors.Wrap(err, "marshal struct")
		}

		return true, nil

	case reflect.Struct:
		if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
			return false, errors.Wrap(err, "id")
		}

		if err := marshalObject(buf, iface); err != nil {
			return false, errors.Wrap(err, "marshal struct")
		}

		return true, nil

	case reflect.Array:
		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array
			// Convert to byte slice
			l := fieldValue.Len()
			b := make([]byte, l)
			for i := 0; i < l; i++ {
				index := fieldValue.Index(i)
				indexInterface := index.Interface()
				val, ok := indexInterface.(byte)
				if !ok {
					return false, errors.Wrap(ErrValueConversion, "byte array index")
				}
				b[i] = val
			}

			if err := writeID(buf, id); err != nil {
				return false, errors.Wrap(err, "id")
			}

			if err := writeBytes(buf, b); err != nil {
				return false, errors.Wrap(err, "bytes")
			}

			return true, nil
		}

		// TODO Add support for arrays of structs and other primative types. --ce

		return false, fmt.Errorf("Field type not implemented: Array: %s", elem.Name())

	case reflect.Slice:
		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte slice
			if err := writeID(buf, id); err != nil {
				return false, errors.Wrap(err, "id")
			}

			if err := writeBytes(buf, fieldValue.Bytes()); err != nil {
				return false, errors.Wrap(err, "bytes")
			}

			return true, nil
		}

		if err := writeID(buf, id); err != nil {
			return false, errors.Wrap(err, "id")
		}

		if err := writeCount(buf, uint64(fieldValue.Len())); err != nil {
			return false, errors.Wrap(err, "write count")
		}

		l := fieldValue.Len()
		for i := 0; i < l; i++ {
			index := fieldValue.Index(i)
			if err := writePrimative(buf, elem.Kind(), index.Interface()); err != nil {
				return false, errors.Wrapf(err, "write item %d", i)
			}
		}

		return true, nil

	default:
		return true, marshalPrimative(buf, id, field.Type.Kind(), iface)
	}
}

func marshalPrimative(buf *bytes.Buffer, id uint64, kind reflect.Kind, iface interface{}) error {
	if err := writeID(buf, id); err != nil {
		return errors.Wrap(err, "id")
	}

	if err := writePrimative(buf, kind, iface); err != nil {
		return errors.Wrap(err, "primative")
	}

	return nil
}

func writePrimative(buf *bytes.Buffer, kind reflect.Kind, iface interface{}) error {
	switch kind {
	case reflect.String:
		value, ok := iface.(string)
		if !ok {
			return ErrValueConversion
		}

		if err := writeBytes(buf, []byte(value)); err != nil {
			return errors.Wrap(err, "bytes")
		}

		return nil

	case reflect.Bool:
		// IsZero was already checked above so we know it is a true boolean value.
		if err := writeOpCode(buf, bitcoin.OP_TRUE); err != nil {
			return errors.Wrap(err, "op code")
		}

		return nil

	case reflect.Int:
		value, ok := iface.(int)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int8:
		value, ok := iface.(int8)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int16:
		value, ok := iface.(int16)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int32:
		value, ok := iface.(int32)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int64:
		value, ok := iface.(int64)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, value); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint:
		value, ok := iface.(uint)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint8:
		value, ok := iface.(uint8)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint16:
		value, ok := iface.(uint16)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint32:
		value, ok := iface.(uint32)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint64:
		value, ok := iface.(uint64)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Float32:
		value, ok := iface.(float32)
		if !ok {
			return ErrValueConversion
		}

		if err := writeFloat32(buf, value); err != nil {
			return errors.Wrap(err, "float32")
		}

		return nil

	case reflect.Float64:
		value, ok := iface.(float64)
		if !ok {
			return ErrValueConversion
		}

		if err := writeFloat64(buf, value); err != nil {
			return errors.Wrap(err, "float64")
		}

		return nil

	default:
		return errors.Wrap(ErrValueConversion, "unknown type")
	}
}

func writeID(buf *bytes.Buffer, id uint64) error {
	_, err := buf.Write(bitcoin.PushNumberScript(int64(id)))
	return err
}

func writeCount(buf *bytes.Buffer, count uint64) error {
	_, err := buf.Write(bitcoin.PushNumberScript(int64(count)))
	return err
}

func writeOpCode(buf *bytes.Buffer, value byte) error {
	if _, err := buf.Write([]byte{value}); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeBytes(buf *bytes.Buffer, value []byte) error {
	if err := bitcoin.WritePushDataScript(buf, value); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeInteger(buf *bytes.Buffer, value int64) error {
	if _, err := buf.Write(bitcoin.PushNumberScript(value)); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeFloat32(buf *bytes.Buffer, value float32) error {
	dataBuf := &bytes.Buffer{}
	if err := binary.Write(dataBuf, binary.LittleEndian, value); err != nil {
		return errors.Wrap(err, "binary")
	}

	if err := bitcoin.WritePushDataScript(buf, dataBuf.Bytes()); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func writeFloat64(buf *bytes.Buffer, value float64) error {
	dataBuf := &bytes.Buffer{}
	if err := binary.Write(dataBuf, binary.LittleEndian, value); err != nil {
		return errors.Wrap(err, "binary")
	}

	if err := bitcoin.WritePushDataScript(buf, dataBuf.Bytes()); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

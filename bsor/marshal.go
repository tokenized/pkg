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
	value := reflect.ValueOf(object)
	typ := value.Type()
	kind := typ.Kind()
	if kind == reflect.Ptr {
		if value.IsNil() {
			if err := writeOpCode(buf, bitcoin.OP_FALSE); err != nil {
				return errors.Wrap(err, "is nil")
			}

			return nil
		} else {
			if err := writeOpCode(buf, bitcoin.OP_TRUE); err != nil {
				return errors.Wrap(err, "is not nil")
			}
		}

		value = value.Elem()
		typ = typ.Elem()
		kind = typ.Kind()
	}

	if kind != reflect.Struct {
		return marshalPrimitive(buf, kind, value.Interface())
	}

	if binaryMarshaler, ok := object.(encoding.BinaryMarshaler); ok {
		fmt.Printf("Using binary marshaller\n")
		b, err := binaryMarshaler.MarshalBinary()
		if err != nil {
			return errors.Wrapf(err, "binary marshal")
		}

		if len(b) == 0 {
			return nil
		}

		if err := writeBytes(buf, b); err != nil {
			return errors.Wrap(err, "bytes")
		}

		return nil
	}

	objectFieldCount := typ.NumField()

	var fieldCount int64 // number of fields marshalled into the script
	fieldBuf := &bytes.Buffer{}
	for i := 0; i < objectFieldCount; i++ {
		field := typ.Field(i)
		fieldValue := value.Field(i)

		if wasAdded, err := marshalField(fieldBuf, field, fieldValue); err != nil {
			return errors.Wrapf(err, "marshal field: %s (%s)", field.Name,
				typeName(field.Type))
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

func typeName(typ reflect.Type) string {
	kind := typ.Kind()
	switch kind {
	case reflect.Ptr, reflect.Slice, reflect.Array:
		return fmt.Sprintf("%s:%s", kind, typeName(typ.Elem()))
	case reflect.Struct:
		return typ.Name()
	default:
		return kind.String()
	}
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
		return false, errors.Wrapf(ErrInvalidID, "bsor tag invalid integer: \"%s\"", idString)
	}

	if id == 0 {
		return false, errors.Wrapf(ErrInvalidID, "bsor tag can't be zero: \"%s\"", idString)
	}

	if fieldValue.IsZero() {
		return false, nil // zero value / empty field
	}

	switch field.Type.Kind() {
	case reflect.Ptr:
		elem := fieldValue.Elem()
		if !elem.CanInterface() {
			return false, nil // not exported, "private" lower case field name
		}

		if elem.Kind() != reflect.Struct {
			if err := writeID(buf, id); err != nil {
				return false, errors.Wrap(err, "id")
			}

			if fieldValue.IsNil() {
				if err := writeOpCode(buf, bitcoin.OP_FALSE); err != nil {
					return false, errors.Wrap(err, "nil")
				}
			} else {
				if err := writeOpCode(buf, bitcoin.OP_TRUE); err != nil {
					return false, errors.Wrap(err, "not nil")
				}
			}

			return true, marshalPrimitive(buf, elem.Kind(), elem.Interface())
		}

		if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
			return false, errors.Wrap(err, "id")
		}

		if err := marshalObject(buf, elem.Interface()); err != nil {
			return false, errors.Wrap(err, "marshal struct")
		}

		return true, nil

	// case reflect.Map: // TODO Add map support --ce

	case reflect.Struct:
		if _, err := buf.Write(bitcoin.PushNumberScript(int64(id))); err != nil {
			return false, errors.Wrap(err, "id")
		}

		if err := marshalObject(buf, iface); err != nil {
			return false, errors.Wrap(err, "marshal struct")
		}

		return true, nil

	case reflect.Array:
		if err := writeID(buf, id); err != nil {
			return false, errors.Wrap(err, "id")
		}

		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			// Convert to byte slice
			l := fieldValue.Len()
			b := make([]byte, l)
			for i := 0; i < l; i++ {
				index := fieldValue.Index(i)

				if index.Kind() == reflect.Ptr {
					if index.IsNil() {
						if err := writeOpCode(buf, bitcoin.OP_FALSE); err != nil {
							return false, errors.Wrap(err, "nil")
						}

						continue
					} else {
						if err := writeOpCode(buf, bitcoin.OP_TRUE); err != nil {
							return false, errors.Wrap(err, "not nil")
						}
					}
				}

				indexInterface := index.Interface()
				val, ok := indexInterface.(byte)
				if !ok {
					return false, errors.Wrap(ErrValueConversion, "byte array index")
				}
				b[i] = val
			}

			if err := writeBytes(buf, b); err != nil {
				return false, errors.Wrap(err, "bytes")
			}

			return true, nil
		}

		// Fixed Size Array encoding
		// Count does not need to be encoded as it is fixed.
		l := fieldValue.Len()
		for i := 0; i < l; i++ {
			index := fieldValue.Index(i)
			if err := marshalObject(buf, index.Interface()); err != nil {
				return false, errors.Wrapf(err, "write item %d", i)
			}
		}

		return true, nil

	case reflect.Slice:
		if err := writeID(buf, id); err != nil {
			return false, errors.Wrap(err, "id")
		}

		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte slice (Binary Data)
			if err := writeBytes(buf, fieldValue.Bytes()); err != nil {
				return false, errors.Wrap(err, "bytes")
			}

			return true, nil
		}

		// Array encoding
		if err := writeCount(buf, uint64(fieldValue.Len())); err != nil {
			return false, errors.Wrap(err, "write count")
		}

		l := fieldValue.Len()
		for i := 0; i < l; i++ {
			index := fieldValue.Index(i)
			if err := marshalObject(buf, index.Interface()); err != nil {
				return false, errors.Wrapf(err, "write item %d", i)
			}
		}

		return true, nil

	default:
		if err := writeID(buf, id); err != nil {
			return false, errors.Wrap(err, "id")
		}

		return true, marshalPrimitive(buf, field.Type.Kind(), iface)
	}
}

func marshalPrimitive(buf *bytes.Buffer, kind reflect.Kind, object interface{}) error {
	switch kind {
	case reflect.Ptr:
		typ := reflect.TypeOf(object)
		return marshalPrimitive(buf, typ.Elem().Kind(), object)

	case reflect.String:
		value, ok := object.(string)
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
		value, ok := object.(int)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int8:
		value, ok := object.(int8)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int16:
		value, ok := object.(int16)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int32:
		value, ok := object.(int32)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Int64:
		value, ok := object.(int64)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, value); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint:
		value, ok := object.(uint)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint8:
		value, ok := object.(uint8)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint16:
		value, ok := object.(uint16)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint32:
		value, ok := object.(uint32)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Uint64:
		value, ok := object.(uint64)
		if !ok {
			return ErrValueConversion
		}

		if err := writeInteger(buf, int64(value)); err != nil {
			return errors.Wrap(err, "integer")
		}

		return nil

	case reflect.Float32:
		value, ok := object.(float32)
		if !ok {
			return ErrValueConversion
		}

		if err := writeFloat32(buf, value); err != nil {
			return errors.Wrap(err, "float32")
		}

		return nil

	case reflect.Float64:
		value, ok := object.(float64)
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

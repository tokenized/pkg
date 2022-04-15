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

type FieldIndexes map[uint64]int

func (f *FieldIndexes) add(id uint64, index int) error {
	if _, exists := (*f)[id]; exists {
		return ErrDuplicateID
	}

	(*f)[id] = index
	return nil
}

func (f FieldIndexes) find(id uint64) *int {
	index, exists := f[id]
	if !exists {
		return nil
	}

	return &index
}

func unmarshalObject(buf *bytes.Reader, value reflect.Value) error {
	if value.Type().Kind() != reflect.Struct {
		return unmarshalPrimative(buf, value)
	}

	objectFieldCount := value.Type().NumField()

	fields := make(FieldIndexes)
	for i := 0; i < objectFieldCount; i++ {
		field := value.Type().Field(i)
		fieldValue := value.Field(i)

		idString := field.Tag.Get("bsor")
		if !fieldValue.CanInterface() {
			if len(idString) > 0 {
				return errors.Wrap(ErrInvalidID, "\"bsor\" tag on unexported field")
			}
			continue // not exported, "private" lower case field name
		}

		if len(idString) == 0 {
			return errors.Wrap(ErrInvalidID, "missing \"bsor\" tag")
		}

		if idString == "-" {
			continue // this field was explicitly excluded from BSOR data
		}

		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			return errors.Wrapf(ErrInvalidID, "bsor tag: \"%s\"", idString)
		}

		if err := fields.add(id, i); err != nil {
			return errors.Wrap(err, field.Name)
		}
	}

	fieldCount, err := readCount(buf)
	if err != nil {
		return errors.Wrap(err, "field count")
	}

	for i := 0; i < int(fieldCount); i++ {
		idItem, err := bitcoin.ParseScript(buf)
		if err != nil {
			return errors.Wrap(err, "parse field id")
		}

		id, err := bitcoin.ScriptNumberValue(idItem)
		if err != nil {
			return errors.Wrap(err, "field id number")
		}

		fieldIndex := fields.find(uint64(id))
		if fieldIndex == nil {
			return fmt.Errorf("Field %d not found in object", id)
		}

		field := value.Type().Field(*fieldIndex)
		fieldValue := value.Field(*fieldIndex)
		if err := unmarshalField(buf, field, fieldValue); err != nil {
			return errors.Wrapf(err, "unmarshal field: %s (id %d) (type %s)", field.Name, id,
				field.Type.Name())
		}
	}

	return nil
}

func unmarshalField(buf *bytes.Reader, field reflect.StructField, fieldValue reflect.Value) error {
	if !fieldValue.CanInterface() {
		return nil // not exported, "private" lower case field name
	}
	iface := fieldValue.Interface()

	if binaryMarshaler, ok := iface.(encoding.BinaryUnmarshaler); ok {
		b, err := readBytes(buf)
		if err != nil {
			return errors.Wrapf(err, "bytes")
		}

		if err := binaryMarshaler.UnmarshalBinary(b); err != nil {
			return errors.Wrapf(err, "binary marshal")
		}

		return nil
	}

	switch field.Type.Kind() {
	case reflect.Ptr:
		elem := field.Type.Elem()
		value := reflect.New(field.Type.Elem())

		if elem.Kind() != reflect.Struct {
			if err := unmarshalPrimative(buf, value.Elem()); err != nil {
				return errors.Wrap(err, "primative")
			}

			fieldValue.Set(value)
			return nil
		}

		if err := unmarshalObject(buf, value.Elem()); err != nil {
			return errors.Wrap(err, "object")
		}

		fieldValue.Set(value)
		return nil

	case reflect.Struct:
		if err := unmarshalObject(buf, fieldValue); err != nil {
			return errors.Wrap(err, "object")
		}

		return nil

	case reflect.Array:
		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8:
			b, err := readBytes(buf)
			if err != nil {
				return errors.Wrap(err, "bytes")
			}

			reflect.Copy(fieldValue, reflect.ValueOf(b))
			return nil
		}

		// TODO Add support for arrays of structs and other primative types. --ce

		return fmt.Errorf("Field type not implemented: Array: %s", elem.Name())

	case reflect.Slice:
		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8:
			b, err := readBytes(buf)
			if err != nil {
				return errors.Wrap(err, "bytes")
			}

			fieldValue.SetBytes(b)
			return nil
		}

		count, err := readCount(buf)
		if err != nil {
			return errors.Wrap(err, "count")
		}

		slice := reflect.MakeSlice(field.Type, int(count), int(count))
		for i := uint64(0); i < count; i++ {
			if err := unmarshalObject(buf, slice.Index(int(i))); err != nil {
				return errors.Wrapf(err, "item %d", i)
			}

		}

		fieldValue.Set(slice)
		return nil

	default:
		return unmarshalPrimative(buf, fieldValue)
	}
}

func unmarshalPrimative(buf *bytes.Reader, value reflect.Value) error {
	switch value.Type().Kind() {
	case reflect.String:
		b, err := readBytes(buf)
		if err != nil {
			return errors.Wrap(err, "bytes")
		}

		value.SetString(string(b))
		return nil

	case reflect.Bool:
		v, err := readInteger(buf)
		if err != nil {
			return errors.Wrap(err, "bool")
		}

		value.SetBool(v != 0)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := readInteger(buf)
		if err != nil {
			return errors.Wrap(err, "integer")
		}

		value.SetInt(int64(v))
		return nil

	case reflect.Float32:
		val, err := readFloat32(buf)
		if err != nil {
			return errors.Wrap(err, "read float32")
		}

		value.SetFloat(float64(val))
		return nil

	case reflect.Float64:
		val, err := readFloat64(buf)
		if err != nil {
			return errors.Wrap(err, "read float64")
		}

		value.SetFloat(val)
		return nil

	case reflect.Ptr:
		return errors.Wrap(ErrValueConversion, "pointer")

	case reflect.Struct:
		return errors.Wrap(ErrValueConversion, "struct")

	case reflect.Array:
		return errors.Wrap(ErrValueConversion, "array")

	case reflect.Slice:
		return errors.Wrap(ErrValueConversion, "slice")

	default:
		return errors.Wrap(ErrValueConversion, "unknown type")
	}
}

func readID(buf *bytes.Reader) (uint64, error) {
	item, err := bitcoin.ParseScript(buf)
	if err != nil {
		return 0, errors.Wrap(err, "parse")
	}

	id, err := bitcoin.ScriptNumberValue(item)
	if err != nil {
		return 0, errors.Wrap(err, "number")
	}

	return uint64(id), nil
}

func readCount(buf *bytes.Reader) (uint64, error) {
	item, err := bitcoin.ParseScript(buf)
	if err != nil {
		return 0, errors.Wrap(err, "parse")
	}

	count, err := bitcoin.ScriptNumberValue(item)
	if err != nil {
		return 0, errors.Wrap(err, "number")
	}

	return uint64(count), nil
}

func readBytes(buf *bytes.Reader) ([]byte, error) {
	item, err := bitcoin.ParseScript(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parse script")
	}

	switch item.Type {
	case bitcoin.ScriptItemTypePushData:
		return item.Data, nil

	case bitcoin.ScriptItemTypeOpCode:
		return []byte{item.OpCode}, nil

	default:
		return nil, bitcoin.ErrInvalidScriptItemType
	}
}

func readInteger(buf *bytes.Reader) (uint64, error) {
	item, err := bitcoin.ParseScript(buf)
	if err != nil {
		return 0, errors.Wrap(err, "parse script")
	}

	value, err := bitcoin.ScriptNumberValue(item)
	if err != nil {
		return 0, errors.Wrap(err, "number")
	}

	return uint64(value), nil
}

func readFloat32(buf *bytes.Reader) (float32, error) {
	b, err := readBytes(buf)
	if err != nil {
		return 0.0, errors.Wrap(err, "bytes")
	}

	var val float32
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &val); err != nil {
		return 0.0, errors.Wrap(err, "float32")
	}

	return val, nil
}

func readFloat64(buf *bytes.Reader) (float64, error) {
	b, err := readBytes(buf)
	if err != nil {
		return 0.0, errors.Wrap(err, "bytes")
	}

	var val float64
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &val); err != nil {
		return 0.0, errors.Wrap(err, "float64")
	}

	return val, nil
}

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

func NewFieldIndexes(typ reflect.Type, value reflect.Value) (*FieldIndexes, error) {
	fieldCount := typ.NumField()

	fields := make(FieldIndexes)
	for i := 0; i < fieldCount; i++ {
		field := typ.Field(i)
		idString := field.Tag.Get("bsor")
		fieldValue := value.Field(i)
		if !fieldValue.CanInterface() {
			if len(idString) > 0 {
				return nil, errors.Wrap(ErrInvalidID, "\"bsor\" tag on unexported field")
			}
			continue // not exported, "private" lower case field name
		}

		if len(idString) == 0 {
			return nil, errors.Wrap(ErrInvalidID, "missing \"bsor\" tag")
		}

		if idString == "-" {
			continue // this field was explicitly excluded from BSOR data
		}

		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidID, "bsor tag invalid integer: \"%s\"", idString)
		}

		if id == 0 {
			return nil, errors.Wrapf(ErrInvalidID, "bsor tag can't be zero: \"%s\"", idString)
		}

		if err := fields.add(id, i); err != nil {
			return nil, errors.Wrap(err, field.Name)
		}
	}

	return &fields, nil
}

func unmarshalObject(buf *bytes.Reader, value reflect.Value) error {
	typ := value.Type()
	kind := typ.Kind()
	var valuePtr reflect.Value
	newValue := value
	isPtr := false
	if kind == reflect.Ptr {
		notNil, err := readInteger(buf)
		if err != nil {
			return errors.Wrap(err, "nil")
		}

		if notNil == 0 {
			return nil
		}

		newValue = newValue.Elem()
		typ = typ.Elem()
		kind = typ.Kind()
		isPtr = true
	}

	if kind != reflect.Struct {
		return unmarshalPrimitive(buf, value)
	}

	if isPtr {
		// Special handling is required to create the object for pointer types.
		valuePtr = reflect.New(typ)
		newValue = valuePtr.Elem()
	}

	fieldCount, err := readCount(buf)
	if err != nil {
		return errors.Wrap(err, "field count")
	}

	if fieldCount == 0 {
		return nil
	}

	fields, err := NewFieldIndexes(typ, newValue)
	if err != nil {
		return errors.Wrap(err, "field indexes")
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
			return fmt.Errorf("Field %d not found in %s", id, typ.Name())
		}

		field := typ.Field(*fieldIndex)
		fieldValue := reflect.New(field.Type).Elem()
		if err := unmarshalField(buf, field, fieldValue); err != nil {
			return errors.Wrapf(err, "unmarshal field: %s (id %d) (type %s)", field.Name, id,
				field.Type.Name())
		}
		newValue.Field(*fieldIndex).Set(fieldValue)
	}

	if isPtr {
		value.Set(valuePtr)
	}

	return nil
}

func unmarshalField(buf *bytes.Reader, field reflect.StructField, fieldValue reflect.Value) error {
	if !fieldValue.CanInterface() {
		return nil // not exported, "private" lower case field name
	}
	iface := fieldValue.Interface()

	kind := field.Type.Kind()
	if binaryMarshaler, ok := iface.(encoding.BinaryUnmarshaler); ok {
		// TODO Handle pointers --ce
		b, err := readBytes(buf)
		if err != nil {
			return errors.Wrapf(err, "bytes")
		}

		if err := binaryMarshaler.UnmarshalBinary(b); err != nil {
			return errors.Wrapf(err, "binary marshal")
		}

		return nil
	}

	switch kind {
	case reflect.Ptr:
		elem := field.Type.Elem()
		value := reflect.New(field.Type.Elem())

		if elem.Kind() != reflect.Struct {
			notNil, err := readInteger(buf)
			if err != nil {
				return errors.Wrap(err, "nil")
			}

			if notNil == 0 {
				return nil
			}

			if err := unmarshalPrimitive(buf, value.Elem()); err != nil {
				return errors.Wrap(err, "primitive")
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
		case reflect.Uint8: // byte array (Binary Data)
			b, err := readBytes(buf)
			if err != nil {
				return errors.Wrap(err, "bytes")
			}

			reflect.Copy(fieldValue, reflect.ValueOf(b))
			return nil
		}

		// Fixed Size Array encoding
		ptr := reflect.New(field.Type)
		array := ptr.Elem()
		for i := 0; i < fieldValue.Len(); i++ {
			if err := unmarshalObject(buf, array.Index(int(i))); err != nil {
				return errors.Wrapf(err, "item %d", i)
			}
		}

		fieldValue.Set(array)
		return nil

	case reflect.Slice:
		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte slice (Binary Data)
			b, err := readBytes(buf)
			if err != nil {
				return errors.Wrap(err, "bytes")
			}

			fieldValue.SetBytes(b)
			return nil
		}

		// Array encoding
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
		return unmarshalPrimitive(buf, fieldValue)
	}
}

func unmarshalPrimitive(buf *bytes.Reader, value reflect.Value) error {
	typ := value.Type()
	switch typ.Kind() {
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
			return errors.Wrap(err, "float32")
		}

		value.SetFloat(float64(val))
		return nil

	case reflect.Float64:
		val, err := readFloat64(buf)
		if err != nil {
			return errors.Wrap(err, "float64")
		}

		value.SetFloat(val)
		return nil

	case reflect.Ptr:
		// Special handling is required to create the object for pointer types.
		valuePtr := reflect.New(typ.Elem())
		newValue := valuePtr.Elem()

		if err := unmarshalObject(buf, newValue); err != nil {
			return errors.Wrap(err, "pointer")
		}

		value.Set(valuePtr)
		return nil

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

package bsor

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
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
				return nil, errors.Wrapf(ErrInvalidID, "\"bsor\" tag on unexported field: %s",
					field.Name)
			}
			continue // not exported, "private" lower case field name
		}

		if len(idString) == 0 {
			return nil, errors.Wrapf(ErrInvalidID, "missing \"bsor\" tag: %s", field.Name)
		}

		if idString == "-" {
			continue // this field was explicitly excluded from BSOR data
		}

		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidID, "bsor tag invalid integer: \"%s\": %s", idString,
				field.Name)
		}

		if id == 0 {
			return nil, errors.Wrapf(ErrInvalidID, "bsor tag can't be zero: \"%s\": %s", idString,
				field.Name)
		}

		if err := fields.add(id, i); err != nil {
			return nil, errors.Wrap(err, field.Name)
		}
	}

	return &fields, nil
}

func unmarshalObject(scriptItems *bitcoin.ScriptItems, value reflect.Value, inArray bool) error {
	typ := value.Type()
	kind := typ.Kind()
	var valuePtr reflect.Value
	newValue := value
	isPtr := false
	if kind == reflect.Ptr {
		isPtr = true
		newValue = newValue.Elem()
		typ = typ.Elem()
		kind = typ.Kind()

		val := reflect.New(typ)
		ifacePtr := val.Interface()
		_, isBinaryUnmarshaler := ifacePtr.(encoding.BinaryUnmarshaler)
		if inArray || isBinaryUnmarshaler {
			notNil, err := readUnsignedInteger(scriptItems)
			if err != nil {
				return errors.Wrap(err, "number")
			}

			if notNil == 0 {
				return nil
			}
		}
	}

	// Check for pointer unmarshaller
	val := reflect.New(typ)
	ifacePtr := val.Interface()
	if binaryUnmarshaler, ok := ifacePtr.(encoding.BinaryUnmarshaler); ok {
		b, err := readBytes(scriptItems)
		if err != nil {
			return errors.Wrapf(err, "bytes")
		}

		if err := binaryUnmarshaler.UnmarshalBinary(b); err != nil {
			return errors.Wrapf(err, "binary unmarshal")
		}

		if isPtr {
			value.Set(val)
		} else {
			value.Set(val.Elem())
		}

		return nil
	}

	if kind != reflect.Struct {
		return unmarshalPrimitive(scriptItems, value, inArray)
	}

	fieldCount, err := readCount(scriptItems)
	if err != nil {
		return errors.Wrap(err, "field count")
	}

	if fieldCount == 0 {
		return nil
	}

	if isPtr {
		// Special handling is required to create the object for pointer types.
		valuePtr = reflect.New(typ)
		newValue = valuePtr.Elem()
	}

	fields, err := NewFieldIndexes(typ, newValue)
	if err != nil {
		return errors.Wrap(err, "field indexes")
	}

	for i := 0; i < int(fieldCount); i++ {
		nextScriptItem, err := nextScriptItem(scriptItems)
		if err != nil {
			return errors.Wrap(err, "number")
		}

		id, err := bitcoin.ScriptNumberValue(nextScriptItem)
		if err != nil {
			return errors.Wrap(err, "field id number")
		}

		fieldIndex := fields.find(uint64(id))
		if fieldIndex == nil {
			return fmt.Errorf("Field %d not found in %s", id, typ.Name())
		}

		field := typ.Field(*fieldIndex)
		fieldValue := reflect.New(field.Type) // must use elem to be "assignable"

		if err := unmarshalField(scriptItems, field, fieldValue.Elem()); err != nil {
			return errors.Wrapf(err, "unmarshal field: %s (id %d) (%s)", field.Name, id,
				typeName(field.Type))
		}
		newValue.Field(*fieldIndex).Set(fieldValue.Elem())
	}

	if isPtr {
		value.Set(valuePtr)
	}

	return nil
}

func unmarshalField(scriptItems *bitcoin.ScriptItems, field reflect.StructField,
	fieldValue reflect.Value) error {

	if !fieldValue.CanInterface() {
		return nil // not exported, "private" lower case field name
	}

	kind := field.Type.Kind()
	if kind == reflect.Ptr {
		elem := field.Type.Elem()
		value := reflect.New(elem)

		if err := unmarshalObject(scriptItems, value.Elem(), false); err != nil {
			return errors.Wrap(err, "ptr object")
		}

		fieldValue.Set(value)
		return nil
	}

	switch kind {
	// case reflect.Map: // TODO Add map support --ce

	case reflect.Struct:
		if err := unmarshalObject(scriptItems, fieldValue, false); err != nil {
			return errors.Wrap(err, "struct")
		}

		return nil

	default:
		return unmarshalPrimitive(scriptItems, fieldValue, false)
	}
}

func unmarshalPrimitive(scriptItems *bitcoin.ScriptItems, value reflect.Value, inArray bool) error {
	typ := value.Type()
	switch typ.Kind() {
	case reflect.String:
		b, err := readBytes(scriptItems)
		if err != nil {
			return errors.Wrap(err, "bytes")
		}

		value.SetString(string(b))
		return nil

	case reflect.Bool:
		v, err := readUnsignedInteger(scriptItems)
		if err != nil {
			return errors.Wrap(err, "bool")
		}

		value.SetBool(v != 0)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := readInteger(scriptItems)
		if err != nil {
			return errors.Wrap(err, "integer")
		}

		value.SetInt(v)
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := readUnsignedInteger(scriptItems)
		if err != nil {
			return errors.Wrap(err, "integer")
		}

		value.SetUint(v)
		return nil

	case reflect.Float32:
		val, err := readFloat32(scriptItems)
		if err != nil {
			return errors.Wrap(err, "float32")
		}

		value.SetFloat(float64(val))
		return nil

	case reflect.Float64:
		val, err := readFloat64(scriptItems)
		if err != nil {
			return errors.Wrap(err, "float64")
		}

		value.SetFloat(val)
		return nil

	case reflect.Ptr:
		// Special handling is required to create the object for pointer types.
		valuePtr := reflect.New(typ.Elem())
		newValue := valuePtr.Elem()

		if err := unmarshalObject(scriptItems, newValue, inArray); err != nil {
			return errors.Wrap(err, "pointer")
		}

		value.Set(valuePtr)
		return nil

	case reflect.Struct:
		return errors.Wrapf(ErrValueConversion, "struct: %s", typeName(typ))

	case reflect.Array:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			b, err := readBytes(scriptItems)
			if err != nil {
				return errors.Wrap(err, "fixed bytes")
			}

			reflect.Copy(value, reflect.ValueOf(b))
			return nil
		}

		// Fixed Size Array encoding
		ptr := reflect.New(typ)
		array := ptr.Elem()
		for i := 0; i < value.Len(); i++ {
			if err := unmarshalObject(scriptItems, array.Index(int(i)), true); err != nil {
				return errors.Wrapf(err, "item %d", i)
			}
		}

		value.Set(array)
		return nil

	case reflect.Slice:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte slice (Binary Data)
			b, err := readBytes(scriptItems)
			if err != nil {
				return errors.Wrap(err, "bytes")
			}

			value.SetBytes(b)
			return nil
		}

		// Array encoding
		count, err := readCount(scriptItems)
		if err != nil {
			return errors.Wrap(err, "count")
		}

		slice := reflect.MakeSlice(typ, int(count), int(count))
		for i := uint64(0); i < count; i++ {
			if err := unmarshalObject(scriptItems, slice.Index(int(i)), true); err != nil {
				return errors.Wrapf(err, "item %d", i)
			}
		}

		value.Set(slice)
		return nil

	default:
		return errors.Wrap(ErrValueConversion, "unknown type")
	}
}

func nextScriptItem(scriptItems *bitcoin.ScriptItems) (*bitcoin.ScriptItem, error) {
	if len(*scriptItems) == 0 {
		return nil, io.EOF
	}

	result := (*scriptItems)[0]
	*scriptItems = (*scriptItems)[1:]
	return result, nil
}

func readCount(scriptItems *bitcoin.ScriptItems) (uint64, error) {
	item, err := nextScriptItem(scriptItems)
	if err != nil {
		return 0, err
	}

	count, err := bitcoin.ScriptNumberValue(item)
	if err != nil {
		return 0, errors.Wrap(err, "number")
	}

	return uint64(count), nil
}

func readBytes(scriptItems *bitcoin.ScriptItems) ([]byte, error) {
	item, err := nextScriptItem(scriptItems)
	if err != nil {
		return nil, err
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

func readInteger(scriptItems *bitcoin.ScriptItems) (int64, error) {
	item, err := nextScriptItem(scriptItems)
	if err != nil {
		return 0, err
	}

	value, err := bitcoin.ScriptNumberValue(item)
	if err != nil {
		return 0, errors.Wrap(err, "number")
	}

	return value, nil
}

func readUnsignedInteger(scriptItems *bitcoin.ScriptItems) (uint64, error) {
	item, err := nextScriptItem(scriptItems)
	if err != nil {
		return 0, err
	}

	value, err := bitcoin.ScriptNumberValueUnsigned(item)
	if err != nil {
		return 0, errors.Wrap(err, "number")
	}

	return value, nil
}

func readFloat32(scriptItems *bitcoin.ScriptItems) (float32, error) {
	b, err := readBytes(scriptItems)
	if err != nil {
		return 0.0, errors.Wrap(err, "bytes")
	}

	var val float32
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &val); err != nil {
		return 0.0, errors.Wrap(err, "float32")
	}

	return val, nil
}

func readFloat64(scriptItems *bitcoin.ScriptItems) (float64, error) {
	b, err := readBytes(scriptItems)
	if err != nil {
		return 0.0, errors.Wrap(err, "bytes")
	}

	var val float64
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &val); err != nil {
		return 0.0, errors.Wrap(err, "float64")
	}

	return val, nil
}

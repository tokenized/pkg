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

func marshalObject(object interface{}, inArray bool) (bitcoin.ScriptItems, error) {
	binaryMarshaler, isBinaryMarshaler := object.(encoding.BinaryMarshaler)
	value := reflect.ValueOf(object)
	typ := value.Type()
	kind := typ.Kind()
	var result bitcoin.ScriptItems
	if kind == reflect.Ptr {
		typ = typ.Elem()
		kind = typ.Kind()

		if value.IsNil() {
			return bitcoin.ScriptItems{bitcoin.NewOpCodeScriptItem(bitcoin.OP_FALSE)}, nil
		}

		if inArray || isBinaryMarshaler {
			result = append(result, bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE))
		}

		value = value.Elem()
	}

	if isBinaryMarshaler {
		b, err := binaryMarshaler.MarshalBinary()
		if err != nil {
			return nil, errors.Wrapf(err, "binary marshal")
		}

		// if len(b) == 0 {
		// 	return pushOpCount, nil
		// }

		return append(result, bitcoin.NewPushDataScriptItem(b)), nil
	}

	if kind != reflect.Struct {
		primitiveScriptItems, err := marshalPrimitive(value, inArray)
		if err != nil {
			return nil, errors.Wrap(err, "primitive")
		}

		return append(result, primitiveScriptItems...), nil
	}

	objectFieldCount := typ.NumField()

	var fieldCount int64 // number of fields marshalled into the script
	var fieldsScriptItems bitcoin.ScriptItems
	for i := 0; i < objectFieldCount; i++ {
		field := typ.Field(i)
		fieldValue := value.Field(i)

		fieldScriptItems, err := marshalField(field, fieldValue)
		if err != nil {
			return nil, errors.Wrapf(err, "marshal field: %s (%s)", field.Name,
				typeName(field.Type))
		} else if len(fieldScriptItems) > 0 {
			fieldCount++
			fieldsScriptItems = append(fieldsScriptItems, fieldScriptItems...)
		}
	}

	result = append(result, bitcoin.PushNumberScriptItem(fieldCount))
	result = append(result, fieldsScriptItems...)
	return result, nil
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

func marshalField(field reflect.StructField,
	fieldValue reflect.Value) (bitcoin.ScriptItems, error) {

	if !fieldValue.CanInterface() {
		return nil, nil // not exported, "private" lower case field name
	}
	iface := fieldValue.Interface()

	idString := field.Tag.Get("bsor")
	if len(idString) == 0 {
		return nil, errors.Wrap(ErrInvalidID, "missing \"bsor\" tag")
	}

	if idString == "-" {
		return nil, nil // this field was explicitly excluded from BSOR data
	}

	id, err := strconv.ParseUint(idString, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(ErrInvalidID, "bsor tag invalid integer: \"%s\"", idString)
	}

	if id == 0 {
		return nil, errors.Wrapf(ErrInvalidID, "bsor tag can't be zero: \"%s\"", idString)
	}

	if fieldValue.IsZero() {
		return nil, nil // zero value / empty field
	}

	typ := fieldValue.Type()
	switch typ.Kind() {
	case reflect.Ptr:
		elem := fieldValue.Elem()
		if !elem.CanInterface() {
			return nil, nil // not exported, "private" lower case field name
		}

		if fieldValue.IsNil() {
			return nil, nil // null
		}

		result := bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))}

		objectScriptItems, err := marshalObject(elem.Interface(), false)
		if err != nil {
			return nil, errors.Wrap(err, "ptr object")
		}

		return append(result, objectScriptItems...), nil

	// case reflect.Map: // TODO Add map support --ce

	case reflect.Struct:
		objectScriptItems, err := marshalObject(iface, false)
		if err != nil {
			return nil, errors.Wrap(err, "struct")
		}

		return append(bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))},
			objectScriptItems...), nil

	default:
		result := bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))}

		primitiveScriptItems, err := marshalPrimitive(fieldValue, false)
		if err != nil {
			return nil, errors.Wrap(err, "primitive")
		}

		return append(result, primitiveScriptItems...), nil
	}
}

func marshalPrimitive(value reflect.Value, inArray bool) (bitcoin.ScriptItems, error) {
	var result bitcoin.ScriptItems
	typ := value.Type()
	switch typ.Kind() {
	case reflect.Ptr:
		if !value.IsNil() && inArray {
			result = append(result, bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE))
		}

		primitiveScriptItems, err := marshalPrimitive(value, inArray)
		if err != nil {
			return nil, errors.Wrap(err, "ptr")
		}

		return append(result, primitiveScriptItems...), err

	case reflect.String:
		return bitcoin.ScriptItems{bitcoin.NewPushDataScriptItem([]byte(value.String()))}, nil

	case reflect.Bool:
		// IsZero was already checked above so we know it is a true boolean value.
		return bitcoin.ScriptItems{bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE)}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(value.Int())}, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItemUnsigned(value.Uint())}, nil

	case reflect.Float32:
		return bitcoin.ScriptItems{float32ScriptItem(float32(value.Float()))}, nil

	case reflect.Float64:
		return bitcoin.ScriptItems{float64ScriptItem(value.Float())}, nil

	case reflect.Array:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			// Convert to byte slice
			l := value.Len()
			b := make([]byte, l)
			for i := 0; i < l; i++ {
				index := value.Index(i)
				indexInterface := index.Interface()
				val, ok := indexInterface.(byte)
				if !ok {
					return nil, errors.Wrap(ErrValueConversion, "byte array index")
				}
				b[i] = val
			}

			return append(result, bitcoin.NewPushDataScriptItem(b)), nil
		}

		// Fixed Size Array encoding
		// Count does not need to be encoded as it is fixed.
		l := value.Len()
		for i := 0; i < l; i++ {
			index := value.Index(i)
			objectScriptItems, err := marshalObject(index.Interface(), true)
			if err != nil {
				return nil, errors.Wrapf(err, "write item %d", i)
			}

			result = append(result, objectScriptItems...)
		}

		return result, nil

	case reflect.Slice:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte slice (Binary Data)
			return append(result, bitcoin.NewPushDataScriptItem(value.Bytes())), nil
		}

		// Array encoding
		result = append(result, bitcoin.PushNumberScriptItem(int64(value.Len())))

		l := value.Len()
		for i := 0; i < l; i++ {
			index := value.Index(i)
			objectScriptItems, err := marshalObject(index.Interface(), true)
			if err != nil {
				return nil, errors.Wrapf(err, "write item %d", i)
			}

			result = append(result, objectScriptItems...)
		}

		return result, nil

	default:
		return nil, errors.Wrapf(ErrValueConversion, "unknown type: %s", typeName(value.Type()))
	}
}

func float32ScriptItem(value float32) *bitcoin.ScriptItem {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, value)
	return bitcoin.NewPushDataScriptItem(buf.Bytes())
}

func float64ScriptItem(value float64) *bitcoin.ScriptItem {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, value)
	return bitcoin.NewPushDataScriptItem(buf.Bytes())
}

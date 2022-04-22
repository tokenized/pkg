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

func marshalObject(object interface{}) (bitcoin.ScriptItems, error) {
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

		if needsNilIndicator(kind) || isBinaryMarshaler {
			result = append(result, bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE))
		}

		value = value.Elem()
	}

	if kind != reflect.Struct {
		primitiveScriptItems, err := marshalPrimitive(kind, value.Interface())
		if err != nil {
			return nil, errors.Wrap(err, "primitive")
		}

		return append(result, primitiveScriptItems...), nil
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

func needsNilIndicator(kind reflect.Kind) bool {
	switch kind {
	case reflect.Slice, reflect.Struct:
		return false
	default:
		return true
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

	switch field.Type.Kind() {
	case reflect.Ptr:
		elem := fieldValue.Elem()
		if !elem.CanInterface() {
			return nil, nil // not exported, "private" lower case field name
		}

		if fieldValue.IsNil() {
			return nil, nil // null
		}

		result := bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))}

		if elem.Kind() != reflect.Struct {
			if fieldValue.IsNil() {
				result = append(result, bitcoin.NewOpCodeScriptItem(bitcoin.OP_FALSE))
			} else {
				result = append(result, bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE))
			}

			primitiveScriptItems, err := marshalPrimitive(elem.Kind(), elem.Interface())
			if err != nil {
				return nil, errors.Wrap(err, "primitive")
			}

			return append(result, primitiveScriptItems...), nil
		}

		objectScriptItems, err := marshalObject(elem.Interface())
		if err != nil {
			return nil, errors.Wrap(err, "struct")
		}

		return append(result, objectScriptItems...), nil

	// case reflect.Map: // TODO Add map support --ce

	case reflect.Struct:
		objectScriptItems, err := marshalObject(iface)
		if err != nil {
			return nil, errors.Wrap(err, "struct")
		}

		return append(bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))},
			objectScriptItems...), nil

	case reflect.Array:
		result := bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))}

		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			// Convert to byte slice
			l := fieldValue.Len()
			b := make([]byte, l)
			for i := 0; i < l; i++ {
				index := fieldValue.Index(i)
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
		l := fieldValue.Len()
		for i := 0; i < l; i++ {
			index := fieldValue.Index(i)
			objectScriptItems, err := marshalObject(index.Interface())
			if err != nil {
				return nil, errors.Wrapf(err, "write item %d", i)
			}

			result = append(result, objectScriptItems...)
		}

		return result, nil

	case reflect.Slice:
		result := bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))}

		elem := field.Type.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte slice (Binary Data)
			return append(result, bitcoin.NewPushDataScriptItem(fieldValue.Bytes())), nil
		}

		// Array encoding
		result = append(result, bitcoin.PushNumberScriptItem(int64(fieldValue.Len())))

		l := fieldValue.Len()
		for i := 0; i < l; i++ {
			index := fieldValue.Index(i)
			objectScriptItems, err := marshalObject(index.Interface())
			if err != nil {
				return nil, errors.Wrapf(err, "write item %d", i)
			}

			result = append(result, objectScriptItems...)
		}

		return result, nil

	default:
		result := bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(id))}

		primitiveScriptItems, err := marshalPrimitive(field.Type.Kind(), iface)
		if err != nil {
			return nil, errors.Wrap(err, "primitive")
		}

		return append(result, primitiveScriptItems...), nil
	}
}

func marshalPrimitive(kind reflect.Kind, object interface{}) (bitcoin.ScriptItems, error) {
	var result bitcoin.ScriptItems
	switch kind {
	case reflect.Ptr:
		typ := reflect.TypeOf(object)
		value := reflect.ValueOf(object)
		elemKind := typ.Elem().Kind()
		if !value.IsNil() && needsNilIndicator(elemKind) {
			result = append(result, bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE))
		}

		primitiveScriptItems, err := marshalPrimitive(elemKind, object)
		if err != nil {
			return nil, errors.Wrap(err, "ptr")
		}

		return append(result, primitiveScriptItems...), err

	case reflect.String:
		value, ok := object.(string)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.NewPushDataScriptItem([]byte(value))}, nil

	case reflect.Bool:
		// IsZero was already checked above so we know it is a true boolean value.
		return bitcoin.ScriptItems{bitcoin.NewOpCodeScriptItem(bitcoin.OP_TRUE)}, nil

	case reflect.Int:
		value, ok := object.(int)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Int8:
		value, ok := object.(int8)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Int16:
		value, ok := object.(int16)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Int32:
		value, ok := object.(int32)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Int64:
		value, ok := object.(int64)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Uint:
		value, ok := object.(uint)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Uint8:
		value, ok := object.(uint8)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Uint16:
		value, ok := object.(uint16)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Uint32:
		value, ok := object.(uint32)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Uint64:
		value, ok := object.(uint64)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{bitcoin.PushNumberScriptItem(int64(value))}, nil

	case reflect.Float32:
		value, ok := object.(float32)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{float32ScriptItem(value)}, nil

	case reflect.Float64:
		value, ok := object.(float64)
		if !ok {
			return nil, ErrValueConversion
		}

		return bitcoin.ScriptItems{float64ScriptItem(value)}, nil

	default:
		return nil, errors.Wrap(ErrValueConversion, "unknown type")
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

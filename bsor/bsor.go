package bsor

import (
	"fmt"
	"reflect"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

var (
	ErrInvalidID       = errors.New("Invalid Field ID")
	ErrDuplicateID     = errors.New("Duplicate Field ID")
	ErrValueConversion = errors.New("Value Conversion")
)

func Marshal(object interface{}) (bitcoin.ScriptItems, error) {
	return marshalObject(object)
}

// Unmarshal reads the object from the scrip items and returns any script items remaining after the
// object has been parsed.
func Unmarshal(scriptItems bitcoin.ScriptItems, object interface{}) (bitcoin.ScriptItems, error) {
	objectType := reflect.TypeOf(object)
	objectValue := reflect.ValueOf(object)
	if objectType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Unmarshal object is not a ptr: %s", objectType.Name())
	}
	if objectValue.IsNil() {
		return nil, errors.New("Unmarshal object is nil")
	}

	objectType = objectType.Elem()
	objectValue = objectValue.Elem()
	if objectType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("Unmarshal object is not a struct: %s", objectType.Kind())
	}

	if err := unmarshalObject(&scriptItems, objectValue); err != nil {
		return nil, errors.Wrap(err, "object")
	}

	return scriptItems, nil
}

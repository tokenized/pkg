package bsor

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/pkg/errors"
)

var (
	ErrInvalidID       = errors.New("Invalid Element ID")
	ErrDuplicateID     = errors.New("Duplicate Element ID")
	ErrValueConversion = errors.New("Value Conversion")
)

func Marshal(object interface{}) (bitcoin.Script, error) {
	buf := &bytes.Buffer{}
	if err := marshalObject(buf, object); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func Unmarshal(script bitcoin.Script, object interface{}) error {
	objectType := reflect.TypeOf(object)
	objectValue := reflect.ValueOf(object)
	if objectType.Kind() != reflect.Ptr {
		return fmt.Errorf("Unmarshal object is not a ptr: %s", objectType.Name())
	}
	if objectValue.IsNil() {
		return errors.New("Unmarshal object is nil")
	}

	objectType = objectType.Elem()
	objectValue = objectValue.Elem()
	if objectType.Kind() != reflect.Struct {
		return fmt.Errorf("Unmarshal object is not a struct: %s", objectType.Kind())
	}

	return unmarshalObject(bytes.NewReader(script), objectValue)
}

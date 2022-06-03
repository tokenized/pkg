package bsor

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
)

const (
	BaseTypeInvalid = BaseType(0)
	BaseTypeStruct  = BaseType(1)
	BaseTypeArray   = BaseType(2)
	BaseTypePointer = BaseType(3)
	BaseTypeString  = BaseType(4)
	BaseTypeBinary  = BaseType(5)
	BaseTypeBool    = BaseType(6)
	BaseTypeInt     = BaseType(7)
	BaseTypeUint    = BaseType(8)
	BaseTypeFloat   = BaseType(9)
)

var (
	binaryMarshalerType      = reflect.TypeOf(new(BinaryMarshaler)).Elem()
	fixedBinaryMarshalerType = reflect.TypeOf(new(FixedBinaryMarshaler)).Elem()
)

type Definition interface {
	Name() string
	Write(w io.Writer) error
}

type Definitions []Definition

type StructDefinition struct {
	name   string             `json:"name"`
	Fields []*FieldDefinition `json:"fields"`
}

type BaseDefinition struct {
	name string    `json:"name"`
	Type FieldType `json:"type"`
}

type FieldDefinition struct {
	Name string    `json:"name"`
	Type FieldType `json:"type"`
	ID   uint      `json:"id"`
}

type FieldType struct {
	Name     string   `json:"name"`
	BaseType BaseType `json:"base_type"`
	Size     uint     `json:"size,omitempty"`
}

type BaseType uint8

// BuildDefinitions uses golang reflection to build object definitions from a type.
func BuildDefinitions(types ...reflect.Type) (Definitions, error) {
	var definitions Definitions
	for i, typ := range types {
		if err := buildDefinition(typ, &definitions); err != nil {
			return nil, errors.Wrapf(err, "type %d", i)
		}
	}

	return definitions, nil
}

func buildDefinition(typ reflect.Type, definitions *Definitions) error {
	kind := typ.Kind()

	if kind == reflect.Struct {
		return buildStructDefinition(typ, definitions)
	}

	fieldType, err := buildFieldType(typ, 0, definitions)
	if err != nil {
		return errors.Wrap(err, "field type")
	}

	*definitions = append(*definitions, &BaseDefinition{
		name: typ.Name(),
		Type: *fieldType,
	})

	return nil
}

func buildStructDefinition(typ reflect.Type, definitions *Definitions) error {
	kind := typ.Kind()

	if kind != reflect.Struct {
		return errors.New("Not a struct")
	}

	structDefinition := &StructDefinition{
		name: typ.Name(),
	}

	objectFieldCount := typ.NumField()
	for i := 0; i < objectFieldCount; i++ {
		field := typ.Field(i)

		idString := field.Tag.Get("bsor")
		if len(idString) == 0 {
			return errors.Wrap(ErrInvalidID, "missing \"bsor\" tag")
		}

		if idString == "-" {
			continue // this field was explicitly excluded from BSOR data
		}

		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			return errors.Wrapf(ErrInvalidID, "bsor tag invalid integer: \"%s\"", idString)
		}

		if id == 0 {
			return errors.Wrapf(ErrInvalidID, "bsor tag can't be zero: \"%s\"", idString)
		}

		fixedSizeString := field.Tag.Get("bsor_fixed_size")
		var fixedSize uint
		if len(fixedSizeString) > 0 {
			value, err := strconv.ParseUint(fixedSizeString, 10, 64)
			if err != nil {
				return errors.Wrapf(ErrInvalidID, "bsor_fixed_size tag invalid integer: \"%s\"",
					fixedSizeString)
			}
			fixedSize = uint(value)
		}

		fieldType, err := buildFieldType(field.Type, fixedSize, definitions)
		if err != nil {
			return errors.Wrapf(err, "field %d", i)
		}

		structDefinition.Fields = append(structDefinition.Fields, &FieldDefinition{
			Name: field.Name,
			Type: *fieldType,
			ID:   uint(id),
		})
	}

	*definitions = append(*definitions, structDefinition)

	return nil
}

func buildFieldType(typ reflect.Type, fixedSize uint,
	definitions *Definitions) (*FieldType, error) {

	kind := typ.Kind()

	if kind == reflect.Ptr {
		ptrFieldType, err := buildFieldType(typ.Elem(), fixedSize, definitions)
		if err != nil {
			return nil, errors.Wrap(err, "pointer")
		}

		return &FieldType{
			Name:     ptrFieldType.String(),
			BaseType: BaseTypePointer,
		}, nil
	}

	if typ.Implements(fixedBinaryMarshalerType) {
		v := reflect.New(typ)
		fixedBinaryMarshaler := v.Interface().(FixedBinaryMarshaler)
		fixedSize := uint(fixedBinaryMarshaler.MarshalBinaryFixedSize())
		return &FieldType{
			BaseType: BaseTypeBinary,
			Size:     fixedSize,
		}, nil
	}

	if typ.Implements(binaryMarshalerType) {
		return &FieldType{
			BaseType: BaseTypeBinary,
		}, nil
	}

	switch kind {
	case reflect.Struct:
		name := typ.Name()
		found := false
		for _, definition := range *definitions {
			if definition.Name() == name {
				found = true
				break
			}
		}

		if !found {
			if err := buildStructDefinition(typ, definitions); err != nil {
				return nil, errors.Wrapf(err, "struct %s", name)
			}
		}

		return &FieldType{
			Name:     name,
			BaseType: BaseTypeStruct,
		}, nil

	case reflect.String:
		return &FieldType{
			BaseType: BaseTypeString,
			Size:     fixedSize,
		}, nil

	case reflect.Bool:
		return &FieldType{
			BaseType: BaseTypeBool,
		}, nil

	case reflect.Int:
		return &FieldType{
			BaseType: BaseTypeInt,
		}, nil

	case reflect.Int8:
		return &FieldType{
			BaseType: BaseTypeInt,
			Size:     1,
		}, nil

	case reflect.Int16:
		return &FieldType{
			BaseType: BaseTypeInt,
			Size:     2,
		}, nil

	case reflect.Int32:
		return &FieldType{
			BaseType: BaseTypeInt,
			Size:     4,
		}, nil

	case reflect.Int64:
		return &FieldType{
			BaseType: BaseTypeInt,
			Size:     8,
		}, nil

	case reflect.Uint:
		return &FieldType{
			BaseType: BaseTypeUint,
		}, nil

	case reflect.Uint8:
		return &FieldType{
			BaseType: BaseTypeUint,
			Size:     1,
		}, nil

	case reflect.Uint16:
		return &FieldType{
			BaseType: BaseTypeUint,
			Size:     2,
		}, nil

	case reflect.Uint32:
		return &FieldType{
			BaseType: BaseTypeUint,
			Size:     4,
		}, nil

	case reflect.Uint64:
		return &FieldType{
			BaseType: BaseTypeUint,
			Size:     8,
		}, nil

	case reflect.Float32:
		return &FieldType{
			BaseType: BaseTypeFloat,
			Size:     4,
		}, nil

	case reflect.Float64:
		return &FieldType{
			BaseType: BaseTypeFloat,
			Size:     8,
		}, nil

	case reflect.Array:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			return &FieldType{
				BaseType: BaseTypeBinary,
				Size:     uint(typ.Len()),
			}, nil
		}

		arrayFieldType, err := buildFieldType(elem, fixedSize, definitions)
		if err != nil {
			return nil, errors.Wrap(err, "array")
		}

		return &FieldType{
			Name:     arrayFieldType.String(),
			BaseType: BaseTypeArray,
			Size:     uint(typ.Len()),
		}, nil

	case reflect.Slice:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			return &FieldType{
				BaseType: BaseTypeBinary,
			}, nil
		}

		sliceFieldType, err := buildFieldType(elem, fixedSize, definitions)
		if err != nil {
			return nil, errors.Wrap(err, "slice")
		}

		return &FieldType{
			Name:     sliceFieldType.String(),
			BaseType: BaseTypeArray,
		}, nil

	default:
		return nil, errors.Wrapf(ErrValueConversion, "unknown type: %s", typeName(typ))
	}
}

func (os Definitions) Write(w io.Writer) error {
	for i, o := range os {
		if i > 0 {
			if _, err := w.Write([]byte("\n")); err != nil {
				return errors.Wrap(err, "line breaks")
			}
		}

		if err := o.Write(w); err != nil {
			return errors.Wrapf(err, "object %d", i)
		}
	}

	return nil
}

func (o StructDefinition) Name() string {
	return o.name
}

func (o StructDefinition) Write(w io.Writer) error {
	if _, err := w.Write([]byte(o.name)); err != nil {
		return errors.Wrap(err, "name")
	}

	if _, err := w.Write([]byte(" {\n")); err != nil {
		return errors.Wrap(err, "open bracket")
	}

	var longestName, longestType, longestID int
	for _, field := range o.Fields {
		if len(field.Name) > longestName {
			longestName = len(field.Name)
		}

		typeString := field.Type.String()
		if len(typeString) > longestType {
			longestType = len(typeString)
		}

		idString := fmt.Sprintf("%d", field.ID)
		if len(idString) > longestID {
			longestID = len(idString)
		}
	}

	for _, field := range o.Fields {
		if _, err := w.Write([]byte("\t")); err != nil {
			return errors.Wrap(err, "tab")
		}

		if err := field.WritePadded(w, longestName, longestType, longestID); err != nil {
			return errors.Wrapf(err, "field %s", field.Name)
		}

		if _, err := w.Write([]byte("\n")); err != nil {
			return errors.Wrap(err, "line break")
		}
	}

	if _, err := w.Write([]byte("}\n")); err != nil {
		return errors.Wrap(err, "close bracket")
	}

	return nil
}

func (b BaseDefinition) Name() string {
	return b.name
}

func (b BaseDefinition) Write(w io.Writer) error {
	if _, err := w.Write([]byte(b.name)); err != nil {
		return errors.Wrap(err, "name")
	}

	if _, err := w.Write([]byte(" ")); err != nil {
		return errors.Wrap(err, "space")
	}
	if _, err := w.Write([]byte(b.Type.String())); err != nil {
		return errors.Wrap(err, "type")
	}

	if _, err := w.Write([]byte(";\n")); err != nil {
		return errors.Wrap(err, "semi-colon")
	}

	return nil
}

func (f FieldDefinition) Write(w io.Writer) error {
	if _, err := w.Write([]byte(f.Name)); err != nil {
		return errors.Wrap(err, "name")
	}

	if _, err := w.Write([]byte(" ")); err != nil {
		return errors.Wrap(err, "space")
	}
	if _, err := w.Write([]byte(f.Type.String())); err != nil {
		return errors.Wrap(err, "type")
	}

	if _, err := w.Write([]byte(" ")); err != nil {
		return errors.Wrap(err, "space")
	}
	if _, err := w.Write([]byte(fmt.Sprintf("%d", f.ID))); err != nil {
		return errors.Wrap(err, "id")
	}

	if _, err := w.Write([]byte(";")); err != nil {
		return errors.Wrap(err, "semi-colon")
	}

	return nil
}

func (f FieldDefinition) WritePadded(w io.Writer, nameWidth, typeWidth, idWidth int) error {
	if _, err := w.Write([]byte(rightPad(f.Name, nameWidth))); err != nil {
		return errors.Wrap(err, "name")
	}

	if _, err := w.Write([]byte(" ")); err != nil {
		return errors.Wrap(err, "space")
	}
	if _, err := w.Write([]byte(rightPad(f.Type.String(), typeWidth))); err != nil {
		return errors.Wrap(err, "type")
	}

	if _, err := w.Write([]byte(" ")); err != nil {
		return errors.Wrap(err, "space")
	}
	if _, err := w.Write([]byte(leftPad(fmt.Sprintf("%d", f.ID), idWidth))); err != nil {
		return errors.Wrap(err, "id")
	}

	if _, err := w.Write([]byte(";")); err != nil {
		return errors.Wrap(err, "semi-colon")
	}

	return nil
}

func (t FieldType) String() string {
	switch t.BaseType {
	case BaseTypeStruct:
		return t.Name
	case BaseTypeArray:
		if t.Size > 0 {
			return fmt.Sprintf("[%d]%s", t.Size, t.Name)
		}
		return fmt.Sprintf("[]%s", t.Name)
	case BaseTypePointer:
		return fmt.Sprintf("*%s", t.Name)
	case BaseTypeString:
		if t.Size > 0 {
			return fmt.Sprintf("string(%d)", t.Size)
		}
		return "string"
	case BaseTypeBinary:
		if t.Size > 0 {
			return fmt.Sprintf("binary(%d)", t.Size)
		}
		return "binary"
	case BaseTypeBool:
		return "bool"
	case BaseTypeInt:
		return "int" + intSizeString(t.Size)
	case BaseTypeUint:
		return "uint" + intSizeString(t.Size)
	case BaseTypeFloat:
		return "float" + intSizeString(t.Size)
	default:
		return "invalid"
	}
}

func intSizeString(v uint) string {
	switch v {
	case 0:
		return ""
	case 1:
		return "8"
	case 2:
		return "16"
	case 4:
		return "32"
	case 8:
		return "64"
	default:
		return ""
	}
}

func (v *BaseType) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("Too short for BaseType : %d", len(data))
	}

	return v.SetString(string(data[1 : len(data)-1]))
}

func (v BaseType) MarshalJSON() ([]byte, error) {
	s := v.String()
	if len(s) == 0 {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf("\"%s\"", s)), nil
}

func (v BaseType) MarshalText() ([]byte, error) {
	s := v.String()
	if len(s) == 0 {
		return nil, fmt.Errorf("Unknown BaseType value \"%d\"", uint8(v))
	}

	return []byte(s), nil
}

func (v *BaseType) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *BaseType) SetString(s string) error {
	switch s {
	case "struct":
		*v = BaseTypeStruct
	case "array":
		*v = BaseTypeArray
	case "pointer":
		*v = BaseTypePointer
	case "string":
		*v = BaseTypeString
	case "binary":
		*v = BaseTypeBinary
	case "bool":
		*v = BaseTypeBool
	case "int":
		*v = BaseTypeInt
	case "uint":
		*v = BaseTypeUint
	case "float":
		*v = BaseTypeFloat
	default:
		*v = BaseTypeInvalid
		return fmt.Errorf("Unknown BaseType value \"%s\"", s)
	}

	return nil
}

func (v BaseType) String() string {
	switch v {
	case BaseTypeStruct:
		return "struct"
	case BaseTypeArray:
		return "array"
	case BaseTypePointer:
		return "pointer"
	case BaseTypeString:
		return "string"
	case BaseTypeBinary:
		return "binary"
	case BaseTypeBool:
		return "bool"
	case BaseTypeInt:
		return "int"
	case BaseTypeUint:
		return "uint"
	case BaseTypeFloat:
		return "float"
	default:
		return "invalid"
	}
}

func leftPad(s string, width int) string {
	formatter := fmt.Sprintf("%%%ds", width)
	return fmt.Sprintf(formatter, s)
}

func rightPad(s string, width int) string {
	formatter := fmt.Sprintf("%%-%ds", width)
	return fmt.Sprintf(formatter, s)
}

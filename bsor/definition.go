package bsor

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	BaseTypeInvalid = BaseType(0)
	BaseTypeStruct  = BaseType(1)
	BaseTypeString  = BaseType(2)
	BaseTypeBinary  = BaseType(3)
	BaseTypeBool    = BaseType(4)
	BaseTypeInt8    = BaseType(5)
	BaseTypeInt16   = BaseType(6)
	BaseTypeInt32   = BaseType(7)
	BaseTypeInt64   = BaseType(8)
	BaseTypeUint8   = BaseType(9)
	BaseTypeUint16  = BaseType(10)
	BaseTypeUint32  = BaseType(11)
	BaseTypeUint64  = BaseType(12)
	BaseTypeFloat32 = BaseType(13)
	BaseTypeFloat64 = BaseType(14)
)

var (
	binaryMarshalerType      = reflect.TypeOf(new(BinaryMarshaler)).Elem()
	fixedBinaryMarshalerType = reflect.TypeOf(new(FixedBinaryMarshaler)).Elem()
)

type Definitions struct {
	Version     uint                  `json:"version"`
	Definitions map[string]Definition `json:"definitions"`
}

type StructDefinition struct {
	Fields []*Field `json:"fields"`
}

type Field struct {
	Name string `json:"name,omitempty"`
	ID   uint   `json:"id,omitempty"`
	Type Type   `json:"type"`
}

type BaseDefinition struct {
	Type Type `json:"type"`
}

type Definition interface {
	String() string
}

type Type struct {
	Type      BaseType `json:"base_type"`
	TypeName  string   `json:"type_name,omitempty"`
	IsArray   bool     `json:"is_array,omitempty"`
	IsPointer bool     `json:"is_pointer,omitempty"`
	FixedSize uint     `json:"fixed_size,omitempty"`
}

type BaseType uint8

// BuildDefinitions uses golang reflection to build object definitions from a type.
func BuildDefinitions(types ...reflect.Type) (Definitions, error) {
	definitions := Definitions{
		Version:     0,
		Definitions: make(map[string]Definition),
	}

	for i, typ := range types {
		if err := buildDefinition(typ, &definitions); err != nil {
			return definitions, errors.Wrapf(err, "type %d", i)
		}
	}

	return definitions, nil
}

func buildDefinition(typ reflect.Type, definitions *Definitions) error {
	kind := typ.Kind()

	if kind == reflect.Struct {
		return buildStructDefinition(typ, definitions)
	}

	fieldType, err := buildType(typ, 0, definitions)
	if err != nil {
		return errors.Wrap(err, "field definition")
	}

	definitions.Definitions[typ.Name()] = &BaseDefinition{
		Type: *fieldType,
	}
	return nil
}

func buildStructDefinition(typ reflect.Type, definitions *Definitions) error {
	kind := typ.Kind()

	if kind != reflect.Struct {
		return errors.New("Not a struct")
	}

	name := typ.Name()
	definition := &StructDefinition{}
	definitions.Definitions[name] = definition

	objectFieldCount := typ.NumField()
	for i := 0; i < objectFieldCount; i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue // not exported, "private" lower case field name
		}

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

		fieldType, err := buildType(field.Type, fixedSize, definitions)
		if err != nil {
			return errors.Wrapf(err, "field %d", i)
		}

		definition.Fields = append(definition.Fields, &Field{
			Name: field.Name,
			ID:   uint(id),
			Type: *fieldType,
		})
	}

	return nil
}

func (definitions *Definitions) FindType(typ reflect.Type) *Type {
	for {
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
			continue
		}
		if typ.Kind() == reflect.Array {
			typ = typ.Elem()
			continue
		}
		if typ.Kind() == reflect.Slice {
			typ = typ.Elem()
			continue
		}
		break
	}

	name := typ.Name()
	for n, typ := range definitions.Definitions {
		if name == n {
			if b, ok := typ.(*BaseDefinition); ok {
				return &b.Type
			}
		}
	}

	return nil
}

func buildType(typ reflect.Type, fixedSize uint,
	definitions *Definitions) (*Type, error) {

	if t := definitions.FindType(typ); t != nil {
		return t, nil
	}

	kind := typ.Kind()

	if kind == reflect.Ptr {
		ptrType, err := buildType(typ.Elem(), fixedSize, definitions)
		if err != nil {
			return nil, errors.Wrap(err, "pointer")
		}

		ptrType.IsPointer = true
		return ptrType, nil
	}

	if typ.Implements(fixedBinaryMarshalerType) {
		v := reflect.New(typ)
		fixedBinaryMarshaler := v.Interface().(FixedBinaryMarshaler)
		fixedSize := uint(fixedBinaryMarshaler.MarshalBinaryFixedSize())
		return &Type{
			Type:      BaseTypeBinary,
			FixedSize: fixedSize,
		}, nil
	}

	if typ.Implements(binaryMarshalerType) {
		return &Type{
			Type: BaseTypeBinary,
		}, nil
	}

	switch kind {
	case reflect.Struct:
		name := typ.Name()
		found := false
		for definitionName := range definitions.Definitions {
			if definitionName == name {
				found = true
				break
			}
		}

		if !found {
			if err := buildStructDefinition(typ, definitions); err != nil {
				return nil, errors.Wrapf(err, "struct %s", name)
			}
		}

		return &Type{
			Type:     BaseTypeStruct,
			TypeName: name,
		}, nil

	case reflect.String:
		return &Type{
			Type:      BaseTypeString,
			FixedSize: fixedSize,
		}, nil

	case reflect.Bool:
		return &Type{
			Type: BaseTypeBool,
		}, nil

	case reflect.Int:
		return &Type{
			Type: BaseTypeInt64,
		}, nil

	case reflect.Int8:
		return &Type{
			Type: BaseTypeInt8,
		}, nil

	case reflect.Int16:
		return &Type{
			Type: BaseTypeInt16,
		}, nil

	case reflect.Int32:
		return &Type{
			Type: BaseTypeInt32,
		}, nil

	case reflect.Int64:
		return &Type{
			Type: BaseTypeInt64,
		}, nil

	case reflect.Uint:
		return &Type{
			Type: BaseTypeUint64,
		}, nil

	case reflect.Uint8:
		return &Type{
			Type: BaseTypeUint8,
		}, nil

	case reflect.Uint16:
		return &Type{
			Type: BaseTypeUint16,
		}, nil

	case reflect.Uint32:
		return &Type{
			Type: BaseTypeUint32,
		}, nil

	case reflect.Uint64:
		return &Type{
			Type: BaseTypeUint64,
		}, nil

	case reflect.Float32:
		return &Type{
			Type: BaseTypeFloat32,
		}, nil

	case reflect.Float64:
		return &Type{
			Type: BaseTypeFloat64,
		}, nil

	case reflect.Array:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			return &Type{
				Type:      BaseTypeBinary,
				FixedSize: uint(typ.Len()),
			}, nil
		}

		arrayType, err := buildType(elem, fixedSize, definitions)
		if err != nil {
			return nil, errors.Wrap(err, "array")
		}

		arrayType.IsArray = true
		arrayType.FixedSize = uint(typ.Len())
		return arrayType, nil

	case reflect.Slice:
		elem := typ.Elem()
		switch elem.Kind() {
		case reflect.Uint8: // byte array (Binary Data)
			return &Type{
				Type: BaseTypeBinary,
			}, nil
		}

		sliceType, err := buildType(elem, fixedSize, definitions)
		if err != nil {
			return nil, errors.Wrap(err, "slice")
		}

		sliceType.IsArray = true
		return sliceType, nil

	default:
		return nil, errors.Wrapf(ErrValueConversion, "unknown type: %s", typeName(typ))
	}
}

func (v Definitions) String() string {
	buf := &bytes.Buffer{}

	buf.Write([]byte(fmt.Sprintf("version %d\n\n", v.Version)))

	for name, def := range v.Definitions {
		b := def.String()

		buf.Write([]byte(fmt.Sprintf("%s ", name)))

		buf.Write(append([]byte(b), []byte("\n")...))
	}

	return string(buf.Bytes())
}

func (v StructDefinition) String() string {
	buf := &bytes.Buffer{}
	var nameLength, idLength int
	for _, field := range v.Fields {
		if len(field.Name) > nameLength {
			nameLength = len(field.Name)
		}

		id := fmt.Sprintf("%d", field.ID)
		if len(id) > idLength {
			idLength = len(id)
		}
	}

	for i, field := range v.Fields {
		if i > 0 {
			buf.Write([]byte("\n"))
		}
		buf.Write([]byte(field.PaddedString(nameLength, idLength)))
	}

	return "{\n" + Indent(string(buf.Bytes()), 2) + "\n}\n"
}

func (v BaseDefinition) String() string {
	return v.Type.String() + "\n"
}

func (v Field) String() string {
	return fmt.Sprintf("%d %s %s", v.ID, v.Name, v.Type.String())
}

func (v Field) PaddedString(nameLength, idLength int) string {
	formatter := fmt.Sprintf("%%%dd %%-%ds %%s", idLength, nameLength)
	return fmt.Sprintf(formatter, v.ID, v.Name, v.Type.String())
}

func (v Type) String() string {
	var result string
	if len(v.TypeName) > 0 {
		result = v.TypeName
	} else {
		result = v.Type.String()
	}

	if v.FixedSize > 0 && (v.Type == BaseTypeBinary || v.Type == BaseTypeString) {
		result = fmt.Sprintf("%s(%d)", v.Type, v.FixedSize)
	}

	if v.IsPointer {
		result = "*" + result
	}

	if v.IsArray {
		if v.FixedSize > 0 {
			return fmt.Sprintf("[%d]%s", v.FixedSize, result)
		}
		return fmt.Sprintf("[]%s", result)
	}

	return result
}

func Indent(s string, count int) string {
	formatter := fmt.Sprintf("%%%ds", count)
	pad := fmt.Sprintf(formatter, "")

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}

	return strings.Join(lines, "\n")
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
	case "string":
		*v = BaseTypeString
	case "binary":
		*v = BaseTypeBinary
	case "bool":
		*v = BaseTypeBool
	case "int8":
		*v = BaseTypeInt8
	case "int16":
		*v = BaseTypeInt16
	case "int32":
		*v = BaseTypeInt32
	case "int64":
		*v = BaseTypeInt64
	case "uint8":
		*v = BaseTypeUint8
	case "uint16":
		*v = BaseTypeUint16
	case "uint32":
		*v = BaseTypeUint32
	case "uint64":
		*v = BaseTypeUint64
	case "float32":
		*v = BaseTypeFloat32
	case "float64":
		*v = BaseTypeFloat64
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
	case BaseTypeString:
		return "string"
	case BaseTypeBinary:
		return "binary"
	case BaseTypeBool:
		return "bool"
	case BaseTypeInt8:
		return "int8"
	case BaseTypeInt16:
		return "int16"
	case BaseTypeInt32:
		return "int32"
	case BaseTypeInt64:
		return "int64"
	case BaseTypeUint8:
		return "uint8"
	case BaseTypeUint16:
		return "uint16"
	case BaseTypeUint32:
		return "uint32"
	case BaseTypeUint64:
		return "uint64"
	case BaseTypeFloat32:
		return "float32"
	case BaseTypeFloat64:
		return "float64"
	default:
		return "invalid"
	}
}

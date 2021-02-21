package logger

import (
	"fmt"
	"strconv"

	"github.com/tokenized/pkg/json"
)

type Field interface {
	Name() string
	ValueJSON() string
}

type StringField struct {
	name  string
	value string
}

func (f StringField) Name() string {
	return f.name
}

func (f StringField) ValueJSON() string {
	return strconv.Quote(f.value)
}

func String(name string, value string) *StringField {
	return &StringField{
		name:  name,
		value: value,
	}
}

type JSONField struct {
	name  string
	value json.Marshaler
}

func (f JSONField) Name() string {
	return f.name
}

func (f JSONField) ValueJSON() string {
	b, err := f.value.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("\"JSON Convert Failed : %s\"", err)
	}
	return string(b)
}

func Marshaler(name string, value json.Marshaler) *JSONField {
	return &JSONField{
		name:  name,
		value: value,
	}
}

type StringerField struct {
	name  string
	value fmt.Stringer
}

func (f StringerField) Name() string {
	return f.name
}

func (f StringerField) ValueJSON() string {
	return strconv.Quote(f.value.String())
}

func Stringer(name string, value fmt.Stringer) *StringerField {
	return &StringerField{
		name:  name,
		value: value,
	}
}

type IntField struct {
	name  string
	value int64
}

func (f IntField) Name() string {
	return f.name
}

func (f IntField) ValueJSON() string {
	return fmt.Sprintf("%d", f.value)
}

func Int(name string, value int) *IntField {
	return &IntField{
		name:  name,
		value: int64(value),
	}
}

func Int8(name string, value int8) *IntField {
	return &IntField{
		name:  name,
		value: int64(value),
	}
}

func Int16(name string, value int16) *IntField {
	return &IntField{
		name:  name,
		value: int64(value),
	}
}

func Int32(name string, value int32) *IntField {
	return &IntField{
		name:  name,
		value: int64(value),
	}
}

func Int64(name string, value int64) *IntField {
	return &IntField{
		name:  name,
		value: value,
	}
}

type UintField struct {
	name  string
	value uint64
}

func (f UintField) Name() string {
	return f.name
}

func (f UintField) ValueJSON() string {
	return fmt.Sprintf("%d", f.value)
}

func Uint(name string, value uint) *UintField {
	return &UintField{
		name:  name,
		value: uint64(value),
	}
}

func Uint8(name string, value uint8) *UintField {
	return &UintField{
		name:  name,
		value: uint64(value),
	}
}

func Uint16(name string, value uint16) *UintField {
	return &UintField{
		name:  name,
		value: uint64(value),
	}
}

func Uint32(name string, value uint32) *UintField {
	return &UintField{
		name:  name,
		value: uint64(value),
	}
}

func Uint64(name string, value uint64) *UintField {
	return &UintField{
		name:  name,
		value: value,
	}
}

type BoolField struct {
	name  string
	value bool
}

func (f BoolField) Name() string {
	return f.name
}

func (f BoolField) ValueJSON() string {
	return fmt.Sprintf("%t", f.value)
}

func Bool(name string, value bool) *BoolField {
	return &BoolField{
		name:  name,
		value: value,
	}
}

type Float32Field struct {
	name  string
	value float32
}

func (f Float32Field) Name() string {
	return f.name
}

func (f Float32Field) ValueJSON() string {
	return fmt.Sprintf("%f", f.value)
}

func Float32(name string, value float32) *Float32Field {
	return &Float32Field{
		name:  name,
		value: value,
	}
}

type Float64Field struct {
	name  string
	value float64
}

func (f Float64Field) Name() string {
	return f.name
}

func (f Float64Field) ValueJSON() string {
	return fmt.Sprintf("%f", f.value)
}

func Float64(name string, value float64) *Float64Field {
	return &Float64Field{
		name:  name,
		value: value,
	}
}

type FormatterField struct {
	name   string
	format string
	values []interface{}
}

func (f FormatterField) Name() string {
	return f.name
}

func (f FormatterField) ValueJSON() string {
	return strconv.Quote(fmt.Sprintf(f.format, f.values...))
}

func Formatter(name string, format string, values ...interface{}) *FormatterField {
	return &FormatterField{
		name:   name,
		format: format,
		values: values,
	}
}

type UintListField struct {
	name   string
	values []uint64
}

func (f UintListField) Name() string {
	return f.name
}

func (f UintListField) ValueJSON() string {
	result := "["
	for i, v := range f.values {
		if i != 0 {
			result += ","
		}
		result += strconv.FormatUint(v, 10)
	}
	result += "]"

	return result
}

func Uints(name string, values []uint) *UintListField {
	result := &UintListField{
		name: name,
	}

	for _, value := range values {
		result.values = append(result.values, uint64(value))
	}

	return result
}

func Uint8s(name string, values []uint8) *UintListField {
	result := &UintListField{
		name: name,
	}

	for _, value := range values {
		result.values = append(result.values, uint64(value))
	}

	return result
}

func Uint16s(name string, values []uint16) *UintListField {
	result := &UintListField{
		name: name,
	}

	for _, value := range values {
		result.values = append(result.values, uint64(value))
	}

	return result
}

func Uint32s(name string, values []uint32) *UintListField {
	result := &UintListField{
		name: name,
	}

	for _, value := range values {
		result.values = append(result.values, uint64(value))
	}

	return result
}

func Uint64s(name string, values []uint64) *UintListField {
	return &UintListField{
		name:   name,
		values: values,
	}
}

type FloatListField struct {
	name   string
	values []float64
}

func (f FloatListField) Name() string {
	return f.name
}

func (f FloatListField) ValueJSON() string {
	result := "["
	for i, v := range f.values {
		if i != 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", v)
	}
	result += "]"

	return result
}

func Float32s(name string, values []float32) *FloatListField {
	result := &FloatListField{
		name: name,
	}

	for _, value := range values {
		result.values = append(result.values, float64(value))
	}

	return result
}

func Float64s(name string, values []float64) *FloatListField {
	return &FloatListField{
		name:   name,
		values: values,
	}
}

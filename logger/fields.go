package logger

import (
	"encoding/hex"
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

type JSONMarshallerField struct {
	name  string
	value json.Marshaler
}

func (f JSONMarshallerField) Name() string {
	return f.name
}

func (f JSONMarshallerField) ValueJSON() string {
	b, err := f.value.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("\"JSON Convert Failed : %s\"", err)
	}
	return string(b)
}

func Marshaler(name string, value json.Marshaler) *JSONMarshallerField {
	return &JSONMarshallerField{
		name:  name,
		value: value,
	}
}

type JSONField struct {
	name  string
	value interface{}
}

func (f JSONField) Name() string {
	return f.name
}

func (f JSONField) ValueJSON() string {
	b, err := json.Marshal(f.value)
	if err != nil {
		return fmt.Sprintf("\"JSON Convert Failed : %s\"", err)
	}
	return string(b)
}

func JSON(name string, value interface{}) *JSONField {
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

type StringersField struct {
	name   string
	values []fmt.Stringer
}

func (f StringersField) Name() string {
	return f.name
}

func (f StringersField) ValueJSON() string {
	result := "["
	for i, v := range f.values {
		if i != 0 {
			result += ","
		}
		result += strconv.Quote(v.String())
	}
	result += "]"

	return result
}

func Stringers(name string, values []fmt.Stringer) *StringersField {
	return &StringersField{
		name:   name,
		values: values,
	}
}

type StringsField struct {
	name   string
	values []string
}

func (f StringsField) Name() string {
	return f.name
}

func (f StringsField) ValueJSON() string {
	result := "["
	for i, v := range f.values {
		if i != 0 {
			result += ","
		}
		result += strconv.Quote(v)
	}
	result += "]"

	return result
}

func Strings(name string, values []string) *StringsField {
	return &StringsField{
		name:   name,
		values: values,
	}
}

type JSONMarshallersField struct {
	name   string
	values []json.Marshaler
}

func (f JSONMarshallersField) Name() string {
	return f.name
}

func (f JSONMarshallersField) ValueJSON() string {
	result := "["
	for i, v := range f.values {
		if i != 0 {
			result += ","
		}

		b, err := v.MarshalJSON()
		if err != nil {
			return fmt.Sprintf("\"JSON Convert Failed : %s\"", err)
		}

		result += string(b)
	}
	result += "]"

	return result
}

func Marshalers(name string, values []json.Marshaler) *JSONMarshallersField {
	return &JSONMarshallersField{
		name:   name,
		values: values,
	}
}

type JSONsField struct {
	name   string
	values []interface{}
}

func (f JSONsField) Name() string {
	return f.name
}

func (f JSONsField) ValueJSON() string {
	result := "["
	for i, v := range f.values {
		if i != 0 {
			result += ","
		}

		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("\"JSON Convert Failed : %s\"", err)
		}
		result += string(b)
	}
	result += "]"

	return result
}

func JSONs(name string, values []interface{}) *JSONsField {
	return &JSONsField{
		name:   name,
		values: values,
	}
}

type HexField struct {
	name  string
	value []byte
}

func (f HexField) Name() string {
	return f.name
}

func (f HexField) ValueJSON() string {
	return strconv.Quote(hex.EncodeToString(f.value))
}

func Hex(name string, value []byte) *HexField {
	return &HexField{
		name:  name,
		value: value,
	}
}

type MillisecondsField struct {
	name  string
	value float64
}

func (f MillisecondsField) Name() string {
	return f.name
}

func (f MillisecondsField) ValueJSON() string {
	return fmt.Sprintf("%06f", f.value)
}

func MillisecondsFromNano(name string, value int64) *MillisecondsField {
	return &MillisecondsField{
		name:  name,
		value: float64(value) / 1e6,
	}
}

func Milliseconds(name string, value float64) *MillisecondsField {
	return &MillisecondsField{
		name:  name,
		value: value,
	}
}

type TimestampField struct {
	name  string
	value float64 // seconds since epoch
}

func (f TimestampField) Name() string {
	return f.name
}

func (f TimestampField) ValueJSON() string {
	return fmt.Sprintf("%06f", f.value)
}

func Timestamp(name string, nanosecondsSinceEpoch int64) *TimestampField {
	return &TimestampField{
		name:  name,
		value: float64(nanosecondsSinceEpoch) / 1e9,
	}
}

package bsor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

type TestStruct1 struct {
	IntField                 int               `bsor:"1"`
	StringField              string            `bsor:"2"`
	SubStruct                TestSubStruct1    `bsor:"3"`
	SubStructPtr             *TestSubStruct1   `bsor:"4"`
	BinaryField              []byte            `bsor:"5"`
	FixedBinaryField         [4]byte           `bsor:"6"`
	PointerField             *string           `bsor:"7"`
	ArrayPrimitiveField      []string          `bsor:"8"`
	FixedArrayPrimitiveField [2]int            `bsor:"9"`
	ArrayObjectField         []TestSubStruct1  `bsor:"10"`
	FixedArrayObjectField    [2]TestSubStruct1 `bsor:"11"`
	ArrayObjectPtrField      []*TestSubStruct1 `bsor:"12"`
	ArrayStringPtrField      []*string         `bsor:"13"`
}

type TestSubStruct1 struct {
	SubIntField    int    `bsor:"1"`
	SubStringField string `bsor:"2"`
}

func Test_Marshal_TestStruct1(t *testing.T) {
	stringValue := "string value"

	tests := []struct {
		value TestStruct1
	}{
		{
			value: TestStruct1{
				IntField:    10,
				StringField: "test",
				SubStruct: TestSubStruct1{
					SubIntField:    20,
					SubStringField: "sub_string",
				},
				SubStructPtr: &TestSubStruct1{
					SubIntField: 21,
				},
				BinaryField:              []byte{0x45, 0xcd},
				FixedBinaryField:         [4]byte{0x10, 0x11, 0x12, 0x13},
				PointerField:             &stringValue,
				ArrayPrimitiveField:      []string{"string1", "string2"},
				FixedArrayPrimitiveField: [2]int{4, 5},
				ArrayObjectField: []TestSubStruct1{
					{
						SubIntField:    22,
						SubStringField: "sub_string_array",
					},
				},
				FixedArrayObjectField: [2]TestSubStruct1{
					{
						SubIntField:    23,
						SubStringField: "sub_string_array2",
					},
					{
						SubIntField:    24,
						SubStringField: "sub_string_array3",
					},
				},
				ArrayObjectPtrField: []*TestSubStruct1{
					nil,
					{
						SubIntField:    25,
						SubStringField: "sub_string_array4",
					},
				},
				ArrayStringPtrField: []*string{
					nil,
					&stringValue,
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			js, _ := json.MarshalIndent(tt.value, "", "  ")
			t.Logf("Struct : %s", js)

			script, err := Marshal(tt.value)
			if err != nil {
				t.Fatalf("Failed to marshal struct : %s", err)
			}

			t.Logf("Script : %s", script)

			read := &TestStruct1{}
			if err := Unmarshal(script, read); err != nil {
				t.Fatalf("Failed to unmarshal script : %s", err)
			}

			js, _ = json.MarshalIndent(read, "", "  ")
			t.Logf("Unmarshalled Struct : %s", js)

			if !reflect.DeepEqual(tt.value, *read) {
				t.Errorf("Unmarshalled value not equal")
			}
		})
	}
}

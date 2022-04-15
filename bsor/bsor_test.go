package bsor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

type TestStruct1 struct {
	IntField        int             `bsor:"1"`
	StringField     string          `bsor:"2"`
	SubStruct       TestSubStruct1  `bsor:"3"`
	SubStructPtr    *TestSubStruct1 `bsor:"4"`
	BytesField      []byte          `bsor:"5"`
	FixedBytesField [4]byte         `bsor:"6"`
	PointerField    *string         `bsor:"7"`
	SliceField      []string        `bsor:"8"`
	ArrayField      [2]int          `bsor:"9"`
}

type TestSubStruct1 struct {
	SubIntField    int    `bsor:"1"`
	SubStringField string `bsor:"2"`
}

func Test_Marshal_TestStruct1(t *testing.T) {
	stringValue := "string"

	tests := []struct {
		value TestStruct1
	}{
		{
			value: TestStruct1{
				IntField:    10,
				StringField: "test",
				SubStruct: TestSubStruct1{
					SubIntField:    8,
					SubStringField: "sub_string",
				},
				SubStructPtr: &TestSubStruct1{
					SubIntField: 9,
				},
				BytesField:      []byte{0x45, 0xcd},
				FixedBytesField: [4]byte{0x10, 0x11, 0x12, 0x13},
				PointerField:    &stringValue,
				SliceField:      []string{"string1", "string2"},
				ArrayField:      [2]int{4, 5},
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

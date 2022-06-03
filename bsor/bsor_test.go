package bsor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/tokenized/pkg/bitcoin"

	"github.com/go-test/deep"
)

type TestStruct struct {
	IntField                    int                   `bsor:"1"`
	StringField                 string                `bsor:"2"`
	SubStruct                   TestSubStruct         `bsor:"3"`
	SubStructPtr                *TestSubStruct        `bsor:"4"`
	BinaryField                 []byte                `bsor:"5"`
	FixedBinaryField            [4]byte               `bsor:"6"`
	PointerField                *string               `bsor:"7"`
	ArrayPrimitiveField         []string              `bsor:"8"`
	FixedArrayPrimitiveField    [2]int                `bsor:"9"`
	ArrayObjectField            []TestSubStruct       `bsor:"10"`
	FixedArrayObjectField       [2]TestSubStruct      `bsor:"11"`
	ArrayObjectPtrField         []*TestSubStruct      `bsor:"12"`
	ArrayStringPtrField         []*string             `bsor:"13"`
	PublicKeyField              bitcoin.PublicKey     `bsor:"14"`
	PublicKeyPtrField           *bitcoin.PublicKey    `bsor:"15"`
	PublicKeyPtrField2          *bitcoin.PublicKey    `bsor:"16"`
	PublicKeyArrayField         []bitcoin.PublicKey   `bsor:"17"`
	PublicKeyFixedArrayField    [2]bitcoin.PublicKey  `bsor:"18"`
	PublicKeyPtrArrayField      []*bitcoin.PublicKey  `bsor:"19"`
	PublicKeyPtrFixedArrayField [2]*bitcoin.PublicKey `bsor:"20"`
	IntPtrField                 *int                  `bsor:"21"`
	IntPtrNilField              *int                  `bsor:"22"`
	IntPtrZeroField             *int                  `bsor:"23"`
	FixedStringField            string                `bsor:"24" bsor_fixed_size:"5"`
}

type TestSubStruct struct {
	SubIntField    int    `bsor:"1"`
	SubStringField string `bsor:"2"`
}

func Test_Marshal_TestStruct1(t *testing.T) {
	stringValue := "string value"
	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	pubKey := key.PublicKey()
	key2, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	pubKey2 := key2.PublicKey()
	intValue := 500
	intZeroValue := 0

	tests := []struct {
		value TestStruct
	}{
		{
			value: TestStruct{
				IntField:    10,
				StringField: "test",
				SubStruct: TestSubStruct{
					SubIntField:    20,
					SubStringField: "sub_string",
				},
				SubStructPtr: &TestSubStruct{
					SubIntField: 21,
				},
				BinaryField:              []byte{0x45, 0xcd},
				FixedBinaryField:         [4]byte{0x10, 0x11, 0x12, 0x13},
				PointerField:             &stringValue,
				ArrayPrimitiveField:      []string{"string1", "string2"},
				FixedArrayPrimitiveField: [2]int{4, 5},
				ArrayObjectField: []TestSubStruct{
					{
						SubIntField:    22,
						SubStringField: "sub_string_array",
					},
				},
				FixedArrayObjectField: [2]TestSubStruct{
					{
						SubIntField:    23,
						SubStringField: "sub_string_array2",
					},
					{
						SubIntField:    24,
						SubStringField: "sub_string_array3",
					},
				},
				ArrayObjectPtrField: []*TestSubStruct{
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
				PublicKeyField:    key.PublicKey(),
				PublicKeyPtrField: &pubKey,
				PublicKeyArrayField: []bitcoin.PublicKey{
					key.PublicKey(),
					key2.PublicKey(),
				},
				PublicKeyFixedArrayField: [2]bitcoin.PublicKey{
					key.PublicKey(),
					key2.PublicKey(),
				},
				PublicKeyPtrArrayField: []*bitcoin.PublicKey{
					nil,
					&pubKey2,
				},
				PublicKeyPtrFixedArrayField: [2]*bitcoin.PublicKey{
					nil,
					&pubKey2,
				},
				IntPtrField:      &intValue,
				IntPtrZeroField:  &intZeroValue,
				FixedStringField: "12345",
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			js, _ := json.MarshalIndent(tt.value, "", "  ")
			t.Logf("Struct : %s", js)

			scriptItems, err := Marshal(tt.value)
			if err != nil {
				t.Fatalf("Failed to marshal struct : %s", err)
			}

			script, err := scriptItems.Script()
			if err != nil {
				t.Fatalf("Failed to create script : %s", err)
			}

			t.Logf("Script (%d push ops): %s", len(scriptItems), script)
			t.Logf("Script raw (%d bytes): %x", len(script), []byte(script))

			read := &TestStruct{}
			scriptItems, err = Unmarshal(scriptItems, read)
			if err != nil {
				t.Fatalf("Failed to unmarshal script : %s", err)
			}

			if len(scriptItems) != 0 {
				t.Errorf("No script items should be remaining : %d", len(scriptItems))
			}

			js, _ = json.MarshalIndent(read, "", "  ")
			t.Logf("Unmarshalled Struct : %s", js)

			if !reflect.DeepEqual(tt.value, *read) {
				t.Errorf("Unmarshalled value not equal : %v", deep.Equal(*read, tt.value))
			}
		})
	}
}

type TestStructSimple struct {
	IntField            int               `bsor:"1"`
	StringField         string            `bsor:"2"`
	IntZeroField        int               `bsor:"3"`
	SubStruct           TestSubStruct     `bsor:"4"`
	BinaryField         []byte            `bsor:"5"`
	IntPointerField1    *int              `bsor:"6"`
	IntPointerField2    *int              `bsor:"7"`
	PublicKeyField      bitcoin.PublicKey `bsor:"8"`
	ArrayStringPtrField []*string         `bsor:"25"`
}

func Test_Marshal_TestStructSimple(t *testing.T) {
	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	intValue := 102
	stringValue := "string value"

	tests := []struct {
		value TestStructSimple
	}{
		{
			value: TestStructSimple{
				IntField:    100,
				StringField: "test string",
				SubStruct: TestSubStruct{
					SubIntField:    101,
					SubStringField: "sub_string",
				},
				BinaryField:      []byte{0xab, 0xcd, 0xef},
				IntPointerField1: &intValue,
				PublicKeyField:   key.PublicKey(),
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

			scriptItems, err := Marshal(tt.value)
			if err != nil {
				t.Fatalf("Failed to marshal struct : %s", err)
			}

			script, err := scriptItems.Script()
			if err != nil {
				t.Fatalf("Failed to create script : %s", err)
			}

			t.Logf("Script (%d push ops): %s", len(scriptItems), script)
			t.Logf("Script raw (%d bytes): %x", len(script), []byte(script))

			read := &TestStructSimple{}
			scriptItems, err = Unmarshal(scriptItems, read)
			if err != nil {
				t.Fatalf("Failed to unmarshal script : %s", err)
			}

			if len(scriptItems) != 0 {
				t.Errorf("No script items should be remaining : %d", len(scriptItems))
			}

			js, _ = json.MarshalIndent(read, "", "  ")
			t.Logf("Unmarshalled Struct : %s", js)

			if !reflect.DeepEqual(tt.value, *read) {
				t.Errorf("Unmarshalled value not equal : %v", deep.Equal(*read, tt.value))
			}
		})
	}
}

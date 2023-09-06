package bsor

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func Test_Definition_TestStruct(t *testing.T) {
	definitions, err := BuildDefinitions(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Fatalf("Failed to build definitions : %s", err)
	}

	js, err := json.MarshalIndent(definitions, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal definitions : %s", err)
	}

	t.Logf("Definitions : \n%s", js)

	s := definitions.String()
	t.Logf("User Definitions : \n%s", s)
}

func Test_CreateDefinitions(t *testing.T) {
	defs, err := BuildDefinitions(
		reflect.TypeOf(TestStruct{}),
		reflect.TypeOf(TestStructSimple{}),
	)
	if err != nil {
		fmt.Printf("Failed to create definitions : %s\n", err)
		return
	}

	file, err := os.Create("test_files/definitions.bsor")
	if err != nil {
		fmt.Printf("Failed to create file : %s", err)
		return
	}

	if _, err := file.Write([]byte(defs.String() + "\n")); err != nil {
		fmt.Printf("Failed to write file : %s", err)
		return
	}
}

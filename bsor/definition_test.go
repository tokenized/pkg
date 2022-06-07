package bsor

import (
	"encoding/json"
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

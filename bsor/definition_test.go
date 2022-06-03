package bsor

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_Definition_TestStruct(t *testing.T) {
	definitions, err := BuildDefinitions(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Fatalf("Failed to build definitions : %s", err)
	}

	buf := &bytes.Buffer{}
	if err := definitions.Write(buf); err != nil {
		t.Fatalf("Failed to write definitions : %s", err)
	}

	t.Logf("Definitions : \n%s", buf.Bytes())
}

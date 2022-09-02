package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/tokenized/pkg/bsor"
	"github.com/tokenized/pkg/expanded_tx"
)

func main() {
	defs, err := bsor.BuildDefinitions(
		reflect.TypeOf(expanded_tx.ExpandedTx{}),
	)
	if err != nil {
		fmt.Printf("Failed to create definitions : %s\n", err)
		return
	}

	file, err := os.Create("expanded_tx.bsor")
	if err != nil {
		fmt.Printf("Failed to create file : %s", err)
		return
	}

	if _, err := file.Write([]byte(defs.String() + "\n")); err != nil {
		fmt.Printf("Failed to write file : %s", err)
		return
	}
}

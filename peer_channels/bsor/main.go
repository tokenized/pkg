package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/tokenized/pkg/bsor"
	"github.com/tokenized/pkg/peer_channels"
)

func main() {
	defs, err := bsor.BuildDefinitions(
		reflect.TypeOf(peer_channels.Channel{}),
		reflect.TypeOf(peer_channels.ChannelList{}),
		reflect.TypeOf(peer_channels.Message{}),
		reflect.TypeOf(peer_channels.Messages{}),
		reflect.TypeOf(peer_channels.MessageNotification{}),
	)
	if err != nil {
		fmt.Printf("Failed to create definitions : %s\n", err)
		return
	}

	file, err := os.Create("peer_channels.bsor")
	if err != nil {
		fmt.Printf("Failed to create file : %s", err)
		return
	}

	if _, err := file.Write([]byte(defs.String() + "\n")); err != nil {
		fmt.Printf("Failed to write file : %s", err)
		return
	}
}

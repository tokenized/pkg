package peer_channels

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

const testBaseURL = "http://localhost:8080"

func Test_Interface(t *testing.T) {
	// Verify that both client implementations fully implement the Client interface. This will not
	// compile if they don't.
	checkInterface := func(c Client) {}

	checkInterface(NewHTTPClient(testBaseURL))
	checkInterface(NewMockClient())
}

func Test_CreateAccount(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	accountID, accountToken, err := client.CreateAccount(ctx, "<uuid>")
	if err != nil {
		t.Fatalf("Failed to create account : %s", err)
	}

	t.Logf("Account ID : %s", *accountID)
	t.Logf("Account Token : %s", *accountToken)
}

func Test_CreateChannel(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	channel, err := client.CreateChannel(ctx, "d695a715-e6c6-4ea1-a501-3eadc83055e0",
		"87b587e9-98cd-43d2-9e8d-fc85ee714177")
	if err != nil {
		t.Fatalf("Failed to create channel : %s", err)
	}

	js, err := json.MarshalIndent(channel, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal channel : %s", err)
	}

	t.Logf("Channel : %s", js)
}

func Test_PostJSONMessage(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	type MessageData struct {
		Data string `json:"data"`
	}

	messageData := MessageData{Data: "Some test data"}

	message, err := client.PostJSONMessage(ctx, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"480bc60b-c779-450a-8c0b-854cbe856a92", messageData)
	if err != nil {
		t.Fatalf("Failed to post message : %s", err)
	}

	js, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal message : %s", err)
	}

	t.Logf("Message : %s", js)
}

func Test_GetMessage(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	messages, err := client.GetMessages(ctx, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd", true)
	if err != nil {
		t.Fatalf("Failed to get messages : %s", err)
	}

	js, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal messages : %s", err)
	}

	t.Logf("Messages : %s", js)
}

func Test_GetMaxMessageSequence(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	max, err := client.GetMaxMessageSequence(ctx, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd")
	if err != nil {
		t.Fatalf("Failed to get max message sequence : %s", err)
	}

	t.Logf("Max Sequence : %d", max)
}

func Test_MarkMessages(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	if err := client.MarkMessages(ctx, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd", 5, false, true); err != nil {
		t.Fatalf("Failed to mark messages : %s", err)
	}
}

func Test_ChannelListen(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create peer channel client : %s", err)
	}

	incoming := make(chan Message)
	interrupt := make(chan interface{})
	complete := make(chan interface{})

	// Receive messages on incoming
	go func() {
		for msg := range incoming {
			js, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				t.Fatalf("Failed to marshal message : %s", err)
			}

			t.Logf("Received : %s", js)
		}
	}()

	// Wait 12 seconds, then send interrupt from another thread
	go func() {
		time.Sleep(12 * time.Second)
		t.Logf("Sending interrupt")
		interrupt <- true
	}()

	// Send a message every second.
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			t.Logf("Sending message %d", i)
			_, err := client.PostJSONMessage(ctx, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
				"480bc60b-c779-450a-8c0b-854cbe856a92", fmt.Sprintf("Test message %d", i))
			if err != nil {
				t.Fatalf("Failed to post message : %s", err)
			}
		}
	}()

	if err := client.ChannelListen(ctx, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd", true, incoming, interrupt); err != nil {
		t.Fatalf("Failed to notify messages : %s", err)
	}
	close(incoming)

	select {
	case <-complete:
	case <-time.After(time.Second):
		t.Fatalf("Shut down timed out")
	}

	t.Logf("Finished Listen")
}

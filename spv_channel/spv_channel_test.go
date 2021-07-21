package spv_channel

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

const rootURL = "http://localhost:8080"

func TestCreateAccount(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	accountID, accountToken, err := CreateAccount(ctx, rootURL, "<uuid>")
	if err != nil {
		t.Fatalf("Failed to create account : %s", err)
	}

	t.Logf("Account ID : %s", *accountID)
	t.Logf("Account Token : %s", *accountToken)
}

func TestCreateChannel(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	channel, err := CreateChannel(ctx, rootURL, "d695a715-e6c6-4ea1-a501-3eadc83055e0",
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

func TestPostMessage(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	type MessageData struct {
		Data string `json:"data"`
	}

	messageData := MessageData{Data: "Some test data"}

	message, err := PostMessage(ctx, rootURL, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
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

func TestGetMessage(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	messages, err := GetMessages(ctx, rootURL, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
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

func TestGetMaxMessageSequence(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	max, err := GetMaxMessageSequence(ctx, rootURL, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd")
	if err != nil {
		t.Fatalf("Failed to get max message sequence : %s", err)
	}

	t.Logf("Max Sequence : %d", max)
}

func TestMarkMessages(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	if err := MarkMessages(ctx, rootURL, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd", 5, false, true); err != nil {
		t.Fatalf("Failed to mark messages : %s", err)
	}
}

func TestListen(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	incoming := make(chan SPVMessage)
	interrupt := make(chan interface{})

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
			_, err := PostMessage(ctx, rootURL, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
				"480bc60b-c779-450a-8c0b-854cbe856a92", fmt.Sprintf("Test message %d", i))
			if err != nil {
				t.Fatalf("Failed to post message : %s", err)
			}
		}
	}()

	if err := Listen(ctx, rootURL, "6f5a5fe3-bf66-4aac-a753-8e33bb77ee99",
		"d4deef7c-cc6d-4e0a-9f1f-e7e6687d8bfd", incoming, interrupt); err != nil {
		t.Fatalf("Failed to notify messages : %s", err)
	}

	t.Logf("Finished Listen")
}

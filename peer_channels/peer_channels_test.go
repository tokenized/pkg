package peer_channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

const testBaseURL = "http://localhost:8080"
const testMasterToken = ""

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

	accountID, accountToken, err := HTTPCreateAccount(ctx, testBaseURL, testMasterToken)
	if err != nil {
		t.Fatalf("Failed to create account : %s", err)
	}

	t.Logf("Account ID : %s", *accountID)
	t.Logf("Account Token : %s", *accountToken)
}

func Test_CreateChannel(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	accountID, accountToken, err := HTTPCreateAccount(ctx, testBaseURL, testMasterToken)
	if err != nil {
		t.Fatalf("Failed to create account : %s", err)
	}

	accountClient := NewHTTPAccountClient(testBaseURL, *accountID, *accountToken)

	channel, err := accountClient.CreateChannel(ctx)
	if err != nil {
		t.Fatalf("Failed to create channel : %s", err)
	}

	js, err := json.MarshalIndent(channel, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal channel : %s", err)
	}

	t.Logf("Channel : %s", js)
}

func Test_WriteMessage_JSON(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	accountID, accountToken, err := HTTPCreateAccount(ctx, testBaseURL, testMasterToken)
	if err != nil {
		t.Fatalf("Failed to create account : %s", err)
	}

	accountClient := NewHTTPAccountClient(testBaseURL, *accountID, *accountToken)

	channel, err := accountClient.CreateChannel(ctx)
	if err != nil {
		t.Fatalf("Failed to create channel : %s", err)
	}

	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create client : %s", err)
	}

	type MessageData struct {
		Data string `json:"data"`
	}

	messageData := MessageData{Data: "Some test data"}
	js, _ := json.Marshal(messageData)

	if err := client.WriteMessage(ctx, channel.ID, channel.WriteToken, ContentTypeJSON,
		bytes.NewReader(js)); err != nil {
		t.Fatalf("Failed to write message : %s", err)
	}

	msgs, err := client.GetMessages(ctx, channel.ID, channel.ReadToken, true, 10)
	if err != nil {
		t.Fatalf("Failed to get channel messages : %s", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Wrong returned message count : got %d, want %d", len(msgs), 1)
	}

	messageJS, _ := json.MarshalIndent(msgs[0], "", "  ")
	t.Logf("Message : %s", messageJS)

	if !bytes.Equal(js, msgs[0].Payload) {
		t.Errorf("Wrong returned message payload : \ngot  %s\n\nwant %s\n", msgs[0].Payload, js)
	}

	if msgs[0].ContentType != ContentTypeJSON {
		t.Errorf("Wrong returned message content type : got %s, want %s", msgs[0].ContentType,
			ContentTypeJSON)
	}

	maxSequence, err := client.GetMaxMessageSequence(ctx, channel.ID, channel.ReadToken)
	if err != nil {
		t.Fatalf("Failed to get max sequence : %s", err)
	}

	if maxSequence != 1 {
		t.Errorf("Wrong max sequence : got %d, want %d", maxSequence, 1)
	}

	if err := client.MarkMessages(ctx, channel.ID, channel.ReadToken, 0, true, true); err != nil {
		t.Fatalf("Failed to mark message as read : %s", err)
	}

	msgs, err = client.GetMessages(ctx, channel.ID, channel.ReadToken, true, 10)
	if err != nil {
		t.Fatalf("Failed to get channel messages : %s", err)
	}

	if len(msgs) != 0 {
		t.Fatalf("Wrong returned message count (they should all be read) : got %d, want %d",
			len(msgs), 0)
	}

	messageData = MessageData{Data: "Some test data 2"}
	js, _ = json.Marshal(messageData)

	if err := client.WriteMessage(ctx, channel.ID, channel.WriteToken, ContentTypeJSON,
		bytes.NewReader(js)); err != nil {
		t.Fatalf("Failed to write message : %s", err)
	}

	msgs, err = client.GetMessages(ctx, channel.ID, channel.ReadToken, true, 10)
	if err != nil {
		t.Fatalf("Failed to get channel messages : %s", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Wrong returned message count (first should be read) : got %d, want %d",
			len(msgs), 1)
	}

	if err := client.DeleteMessage(ctx, channel.ID, channel.ReadToken, 1, true); err != nil {
		t.Fatalf("Failed to delete message : %s", err)
	}

	msgs, err = client.GetMessages(ctx, channel.ID, channel.ReadToken, false, 10)
	if err != nil {
		t.Fatalf("Failed to get channel messages : %s", err)
	}

	if len(msgs) != 0 {
		t.Fatalf("Wrong returned message count (they should all be deleted) : got %d, want %d",
			len(msgs), 0)
	}
}

func Test_ChannelListen(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	accountID, accountToken, err := HTTPCreateAccount(ctx, testBaseURL, testMasterToken)
	if err != nil {
		t.Fatalf("Failed to create account : %s", err)
	}

	accountClient := NewHTTPAccountClient(testBaseURL, *accountID, *accountToken)

	channel, err := accountClient.CreateChannel(ctx)
	if err != nil {
		t.Fatalf("Failed to create channel : %s", err)
	}

	factory := NewFactory()
	client, err := factory.NewClient(testBaseURL)
	if err != nil {
		t.Fatalf("Failed to create client : %s", err)
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
			msg := fmt.Sprintf("Test message %d", i)
			if err := client.WriteMessage(ctx, channel.ID, channel.WriteToken, ContentTypeText,
				bytes.NewReader([]byte(msg))); err != nil {
				t.Fatalf("Failed to write message : %s", err)
			}
		}
	}()

	if err := client.Listen(ctx, channel.ReadToken, true, incoming, interrupt); err != nil {
		t.Fatalf("Failed to listen : %s", err)
	}
	close(incoming)

	select {
	case <-complete:
	case <-time.After(time.Second):
		t.Fatalf("Shut down timed out")
	}

	t.Logf("Finished Listen")
}

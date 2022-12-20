package peer_channels

import (
	"encoding/json"
	"testing"
)

const testBaseURL = "http://localhost:8080"
const testMasterToken = ""

func Test_Channels_JSON(t *testing.T) {
	tests := []struct {
		json      string
		masked    string
		channelID string
		token     string
		full      string
	}{
		{
			json: `{
				"peer_channel": "mock://mock_peer_channels/api/v1/channel/123456?token=abcdef"
			}`,
			masked:    "mock://mock_peer_channels/api/v1/channel/123456",
			channelID: "123456",
			token:     "abcdef",
			full:      "mock://mock_peer_channels/api/v1/channel/123456?token=abcdef",
		},
		{
			json: `{
				"peer_channel": "mock://mock_peer_channels/prefix/api/v1/channel/123456?token=abcdef"
			}`,
			masked:    "mock://mock_peer_channels/prefix/api/v1/channel/123456",
			channelID: "123456",
			token:     "abcdef",
			full:      "mock://mock_peer_channels/prefix/api/v1/channel/123456?token=abcdef",
		},
		{
			json: `{
				"peer_channel": "mock://mock_peer_channels/api/v1/channel/123456?token=abcdef&other_value=test"
			}`,
			masked:    "mock://mock_peer_channels/api/v1/channel/123456?other_value=test",
			channelID: "123456",
			token:     "abcdef",
			full:      "mock://mock_peer_channels/api/v1/channel/123456?other_value=test&token=abcdef",
		},
	}

	for i, tt := range tests {
		var config struct {
			Channel *Channel `json:"peer_channel"`
		}

		if err := json.Unmarshal([]byte(tt.json), &config); err != nil {
			t.Fatalf("Failed to unmarshal : %s", err)
		}

		if config.Channel == nil {
			t.Fatalf("Peer channel should not be nil")
		}

		t.Logf("Test %d", i)
		t.Logf("Peer channel URL : %s", config.Channel.String())
		t.Logf("Peer channel masked : %s", config.Channel.MaskedString())
		t.Logf("Peer channel token : %s", config.Channel.Token)

		if config.Channel.MaskedString() != tt.masked {
			t.Errorf("Wrong peer channel masked : got %s, want %s", config.Channel.MaskedString(),
				tt.masked)
		}

		if config.Channel.ChannelID != tt.channelID {
			t.Errorf("Wrong peer channel token : got %s, want %s", config.Channel.ChannelID,
				tt.channelID)
		}

		if config.Channel.Token != tt.token {
			t.Errorf("Wrong peer channel token : got %s, want %s", config.Channel.Token,
				tt.token)
		}

		if config.Channel.String() != tt.full {
			t.Errorf("Wrong peer channel URL : got %s, want %s", config.Channel.String(),
				tt.full)
		}
	}
}

func Test_Channels_String(t *testing.T) {
	tests := []struct {
		masked string
		token  string
		full   string
	}{
		{
			masked: "https://mock.tokenized.id" + apiURLChannelPart + "123456",
			token:  "abcedfg",
			full:   "https://mock.tokenized.id" + apiURLChannelPart + "123456?token=abcedfg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.full, func(t *testing.T) {
			var peerChannel Channel
			if err := peerChannel.SetString(tt.full); err != nil {
				t.Fatalf("Failed to set string : %s", err)
			}

			if peerChannel.MaskedString() != tt.masked {
				t.Errorf("Wrong masked : got %s, want %s", peerChannel.MaskedString(), tt.masked)
			}

			if peerChannel.Token != tt.token {
				t.Errorf("Wrong token : got %s, want %s", peerChannel.Token, tt.token)
			}

			full := peerChannel.String()
			t.Logf("Full URL : %s", full)

			if full != tt.full {
				t.Errorf("Wrong full url : got %s, want %s", full, tt.full)
			}
		})
	}
}

func Test_ParseURL(t *testing.T) {
	tests := []struct {
		baseURL   string
		channelID string
		url       string
	}{
		{
			baseURL:   "https://test.com",
			channelID: "123456",
			url:       "https://test.com/api/v1/channel/123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			baseURL, channelID, _, err := parseChannel(tt.url)
			if err != nil {
				t.Errorf("Failed to parse channel URL : %s", err)
			}

			t.Logf("Base URL : %s", baseURL)
			t.Logf("Channel ID : %s", channelID)

			if baseURL != tt.baseURL {
				t.Errorf("Wrong base URL : got %s, want %s", baseURL, tt.baseURL)
			}

			if channelID != tt.channelID {
				t.Errorf("Wrong channel ID : got %s, want %s", channelID, tt.channelID)
			}

			url := ChannelURL(tt.baseURL, tt.channelID)

			t.Logf("URL : %s", url)

			if url != tt.url {
				t.Errorf("Wrong URL : got %s, want %s", url, tt.url)
			}
		})
	}
}

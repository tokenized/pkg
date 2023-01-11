package peer_channels

import (
	"encoding/json"
	"testing"
)

func Test_Accounts_JSON(t *testing.T) {
	tests := []struct {
		json      string
		masked    string
		accountID string
		token     string
		full      string
	}{
		{
			json: `{
				"peer_account": "mock://mock_peer_channels/api/v1/account/123456?token=abcdef"
			}`,
			masked:    "mock://mock_peer_channels/api/v1/account/123456",
			accountID: "123456",
			token:     "abcdef",
			full:      "mock://mock_peer_channels/api/v1/account/123456?token=abcdef",
		},
		{
			json: `{
				"peer_account": "mock://mock_peer_channels/prefix/api/v1/account/123456?token=abcdef"
			}`,
			masked:    "mock://mock_peer_channels/prefix/api/v1/account/123456",
			accountID: "123456",
			token:     "abcdef",
			full:      "mock://mock_peer_channels/prefix/api/v1/account/123456?token=abcdef",
		},
		{
			json: `{
				"peer_account": "mock://mock_peer_channels/api/v1/account/123456?token=abcdef&other_value=test"
			}`,
			masked:    "mock://mock_peer_channels/api/v1/account/123456?other_value=test",
			accountID: "123456",
			token:     "abcdef",
			full:      "mock://mock_peer_channels/api/v1/account/123456?other_value=test&token=abcdef",
		},
	}

	for i, tt := range tests {
		var config struct {
			Account *Account `json:"peer_account"`
		}

		if err := json.Unmarshal([]byte(tt.json), &config); err != nil {
			t.Fatalf("Failed to unmarshal : %s", err)
		}

		if config.Account == nil {
			t.Fatalf("Peer account should not be nil")
		}

		t.Logf("Test %d", i)
		t.Logf("Peer account URL : %s", config.Account.String())
		t.Logf("Peer account masked : %s", config.Account.MaskedString())
		t.Logf("Peer account token : %s", config.Account.Token)

		if config.Account.MaskedString() != tt.masked {
			t.Errorf("Wrong peer account masked : \ngot  %s, \nwant %s", config.Account.MaskedString(),
				tt.masked)
		}

		if config.Account.AccountID != tt.accountID {
			t.Errorf("Wrong peer account token : got %s, want %s", config.Account.AccountID,
				tt.accountID)
		}

		if config.Account.Token != tt.token {
			t.Errorf("Wrong peer account token : got %s, want %s", config.Account.Token,
				tt.token)
		}

		if config.Account.String() != tt.full {
			t.Errorf("Wrong peer account URL : got %s, want %s", config.Account.String(),
				tt.full)
		}
	}
}

package bsvalias

import (
	"fmt"
	"testing"
	"time"
)

func Test_parseCacheControl(t *testing.T) {
	now := time.Now()
	nowUnix := now.Unix()

	tests := []struct {
		text        string
		cacheExpiry *time.Time
	}{
		{
			text:        fmt.Sprintf("public, max-age=%d", nowUnix),
			cacheExpiry: &now,
		},
		{
			text:        fmt.Sprintf("public, max-age=%d, immutable", nowUnix),
			cacheExpiry: &now,
		},
		{
			text:        fmt.Sprintf("max-age=%d", nowUnix),
			cacheExpiry: &now,
		},
		{
			text:        "",
			cacheExpiry: nil,
		},
		{
			text:        "public, immutable",
			cacheExpiry: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			cacheExpiry := parseCacheControl(tt.text)

			if tt.cacheExpiry == nil {
				if cacheExpiry != nil {
					t.Errorf("Cache expiry should be nil : %s", cacheExpiry)
				} else {
					t.Logf("Cache expiry is nil")
				}
			} else {
				if cacheExpiry == nil {
					t.Errorf("Cache expiry should not be nil")
				} else if cacheExpiry.Unix() != tt.cacheExpiry.Unix() {
					t.Errorf("Wrong cache expiry : got %s, want %s", cacheExpiry, tt.cacheExpiry)
				} else {
					t.Logf("Cache expiry is correct : %s", cacheExpiry)
				}
			}
		})
	}
}

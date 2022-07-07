package bsvalias

import (
	"context"
	"testing"

	"github.com/pkg/errors"
)

func TestGetSite(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	tests := []struct {
		name    string
		domain  string
		wantErr error
		wantURL string
	}{
		{
			name:    "moneybutton.com",
			domain:  "moneybutton.com",
			wantURL: "https://moneybutton.com",
		},
		{
			name:    "polynym.io",
			domain:  "polynym.io",
			wantURL: "https://api.polynym.io",
		},
		{
			name:    "handcash.io",
			domain:  "handcash.io",
			wantURL: "https://cloud.handcash.io",
		},
		{
			name:    "tokenized.id",
			domain:  "tokenized.id",
			wantURL: "https://nexus-api.tokenized.com",
		},
		{
			name:    "example.com",
			domain:  "example.com",
			wantErr: ErrNotCapable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site, err := GetSite(ctx, tt.domain)

			if tt.wantErr != nil {
				if errors.Cause(err) != tt.wantErr {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}

				// success
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if site.URL != tt.wantURL {
				t.Errorf("got %q want %q", site.URL, tt.wantURL)
			}
		})
	}
}

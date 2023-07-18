package bsvalias

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pkg/errors"
)

func TestGetSite(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                     string
		domain                   string
		wantErr                  error
		wantURL                  string
		requiresSenderValidation bool
	}{
		// {
		// 	name:    "moneybutton.com",
		// 	domain:  "moneybutton.com",
		// 	wantURL: "https://moneybutton.com",
		// },
		// {
		// 	name:    "polynym.io",
		// 	domain:  "polynym.io",
		// 	wantURL: "https://api.polynym.io",
		// },
		// {
		// 	name:    "handcash.io",
		// 	domain:  "handcash.io",
		// 	wantURL: "https://cloud.handcash.io",
		// },
		{
			name:    "tkz.id",
			domain:  "tkz.id",
			wantURL: "https://nexus-api.tokenized.com",
		},
		// {
		// 	name:    "example.com",
		// 	domain:  "example.com",
		// 	wantErr: ErrNotCapable,
		// },
		// {
		// 	name:                     "centbee.com",
		// 	domain:                   "centbee.com",
		// 	wantURL:                  "https://d1xxc9dtnjao6f.cloudfront.net",
		// 	requiresSenderValidation: true,
		// },
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

			js, _ := json.MarshalIndent(site.Capabilities, "", "  ")
			t.Logf("Capabilities : %s", js)

			if site.Capabilities.RequiresNameSenderValidation() {
				if !tt.requiresSenderValidation {
					t.Errorf("Should not require sender validation")
				} else {
					t.Logf("Requires sender validation")
				}
			} else {
				if tt.requiresSenderValidation {
					t.Errorf("Should require sender validation")
				} else {
					t.Logf("Does not require sender validation")
				}
			}

			if site.URL != tt.wantURL {
				t.Errorf("got %q want %q", site.URL, tt.wantURL)
			}
		})
	}
}

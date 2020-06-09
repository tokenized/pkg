package bsvalias

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
)

const (
	URLNamePKI                = "pki"
	URLNamePaymentDestination = "paymentDestination"
	URLNamePaymentRequest     = "f7ecaab847eb"
)

func GetSite(ctx context.Context, domain string) (Site, error) {
	// Lookup SRV record for possible hosting other than specified domain
	_, records, _ := net.LookupSRV("bsvalias", "tcp", domain)

	var site Site
	if len(records) > 0 {
		// Strip period at end of target if it exists.
		// I am not sure why it is there --ce
		l := len(records[0].Target)
		if records[0].Target[l-1] == '.' {
			records[0].Target = records[0].Target[:l-1]
		}

		url := fmt.Sprintf("https://%s:%d/.well-known/bsvalias", records[0].Target, records[0].Port)
		if err := get(url, &site.Capabilities); err == nil {
			site.URL = fmt.Sprintf("https://%s:%d", records[0].Target, records[0].Port)
			return site, nil
		}
	}

	url := fmt.Sprintf("https://%s/.well-known/bsvalias", domain)
	if err := get(url, &site.Capabilities); err != nil {
		return site, errors.Wrap(err, "http get capabilites")
	}

	site.URL = fmt.Sprintf("https://%s", domain)
	return site, nil
}

// GetURL returns the URL for the specified capability.
func (c Capabilities) GetURL(name string) (string, error) {
	value, exists := c.Capabilities[name]
	if !exists {
		return "", ErrNotCapable
	}

	url, ok := value.(string)
	if !ok || len(url) == 0 {
		return "", ErrNotCapable
	}

	return url, nil
}

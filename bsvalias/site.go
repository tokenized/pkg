package bsvalias

import (
	"context"
	"fmt"
	"net"

	"github.com/tokenized/logger"

	"github.com/pkg/errors"
)

func GetSite(ctx context.Context, domain string) (Site, error) {
	// Lookup SRV record for possible hosting other than specified domain
	_, records, _ := net.LookupSRV("bsvalias", "tcp", domain)

	var site Site

	if len(records) > 0 {
		// Strip period at end of target.
		r := records[0]

		// get the domain name from the SRV record
		l := len(r.Target)
		if r.Target[l-1] == '.' {
			r.Target = r.Target[:l-1]
		}

		// If the port is 443 we can omit it, as we're using https anyway.
		//
		// Adding the port actually causes some webservers to fail (e.g. polynym, requiring the
		// fallback.
		host := fmt.Sprintf("https://%s", r.Target)
		if r.Port != 443 {
			// a port other than 443
			host = fmt.Sprintf("%s:%d", host, r.Port)
		}

		url := fmt.Sprintf("%s/.well-known/bsvalias", host)

		if err := get(ctx, url, &site.Capabilities); err == nil {
			site.URL = host
			return site, nil
		}

		// err was not nil, so we deliberately fall through to the default, below.
		logger.Warn(ctx, "SRV resolution failed for %s, attempting default", domain)
	}

	// use the default well known url, per the spec.
	url := fmt.Sprintf("https://%s/.well-known/bsvalias", domain)

	if err := get(ctx, url, &site.Capabilities); err != nil {
		return site, errors.Wrap(ErrNotCapable, err.Error())
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

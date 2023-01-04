package peer_channels

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

type Channel struct {
	BaseURL   string
	ChannelID string
	Token     string
}

type Channels []*Channel

func NewChannel(baseURL, channelID, token string) (*Channel, error) {
	if _, err := url.Parse(baseURL); err != nil {
		return nil, errors.Wrap(err, "url")
	}

	return &Channel{
		BaseURL:   baseURL,
		ChannelID: channelID,
		Token:     token,
	}, nil
}

func ParseChannel(channelURL string) (*Channel, error) {
	baseURL, channelID, token, err := parseChannel(channelURL)
	if err != nil {
		return nil, err
	}

	return &Channel{
		BaseURL:   baseURL,
		ChannelID: channelID,
		Token:     token,
	}, nil
}

func (v Channel) MarshalText() ([]byte, error) {
	u, err := url.Parse(v.BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	u.Path = appendPath(u.Path, apiURLChannelPart)
	u.Path = appendPath(u.Path, v.ChannelID)

	query := u.Query()
	query.Add("token", url.PathEscape(v.Token))
	u.RawQuery = query.Encode()

	return []byte(u.String()), nil
}

func (v *Channel) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *Channel) SetString(s string) error {
	baseURL, channelID, token, err := parseChannel(s)
	if err != nil {
		return err
	}

	v.BaseURL = baseURL
	v.ChannelID = channelID
	v.Token = token
	return nil
}

func CopyString(s string) string {
	result := make([]byte, len(s))
	copy(result, s)
	return string(result)
}

func (v Channel) Copy() Channel {
	return Channel{
		BaseURL:   CopyString(v.BaseURL),
		ChannelID: CopyString(v.ChannelID),
		Token:     CopyString(v.Token),
	}
}

func (v Channel) String() string {
	b, err := v.MarshalText()
	if err != nil {
		return ""
	}

	return string(b)
}

// MaskedString returns the URL without the token.
func (v Channel) MaskedString() string {
	u, err := url.Parse(v.BaseURL)
	if err != nil {
		return ""
	}

	u.Path = appendPath(u.Path, apiURLChannelPart)
	u.Path = appendPath(u.Path, v.ChannelID)

	return u.String()
}

func (v Channel) MarshalBinary() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *Channel) UnmarshalBinary(data []byte) error {
	return v.SetString(string(data))
}

// Scan converts from a database column.
func (v *Channel) Scan(data interface{}) error {
	s, ok := data.(string)
	if !ok {
		return errors.New("Peer Channel value not string")
	}

	if err := v.SetString(s); err != nil {
		return errors.Wrap(err, "set string")
	}

	return nil
}

func (v Channel) MarshalJSONMasked() ([]byte, error) {
	return []byte("\"Masked:" + v.MaskedString() + "\""), nil
}

// parseChannel parses a channel URL and returns the baseURL, channelID, and token.
func parseChannel(channelURL string) (string, string, string, error) {
	u, err := url.Parse(channelURL)
	if err != nil {
		return "", "", "", errors.Wrap(err, "url")
	}

	query := u.Query()

	token := query.Get("token")
	query.Del("token")

	u.RawQuery = query.Encode()

	parts := strings.Split(u.Path, apiURLChannelPart)
	if len(parts) != 2 {
		return "", "", "", errors.New("Missing api channel part")
	}

	u.Path = parts[0]

	if len(parts[1]) == 0 {
		return "", "", "", errors.New("Missing channel id")
	}
	channelParts := strings.Split(parts[1], "/")
	channelID := channelParts[0]

	return u.String(), channelID, token, nil
}

func appendPath(base, add string) string {
	lb := len(base)
	if lb == 0 {
		return add
	}

	la := len(add)
	if la == 0 {
		return base
	}

	if base[lb-1] == '/' {
		base = base[:lb-1]
	}

	if add[0] == '/' {
		add = add[1:]
	}

	return fmt.Sprintf("%s/%s", base, add)
}

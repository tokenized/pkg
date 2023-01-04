package peer_channels

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

type Account struct {
	BaseURL   string `bsor:"1" json:"base_url"`
	AccountID string `bsor:"2" json:"account_id"`
	Token     string `bsor:"3" json:"token"`
}

func NewAccount(baseURL, accountID, token string) (*Account, error) {
	if _, err := url.Parse(baseURL); err != nil {
		return nil, errors.Wrap(err, "url")
	}

	return &Account{
		BaseURL:   baseURL,
		AccountID: accountID,
		Token:     token,
	}, nil
}

func ParseAccount(accountURL string) (*Account, error) {
	baseURL, accountID, token, err := parseAccount(accountURL)
	if err != nil {
		return nil, err
	}

	return &Account{
		BaseURL:   baseURL,
		AccountID: accountID,
		Token:     token,
	}, nil
}

func (v Account) Copy() Account {
	return Account{
		BaseURL:   CopyString(v.BaseURL),
		AccountID: CopyString(v.AccountID),
		Token:     CopyString(v.Token),
	}
}

func (v Account) MarshalText() ([]byte, error) {
	u, err := url.Parse(v.BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "url")
	}

	u.Path = appendPath(u.Path, apiURLAccountPart)
	u.Path = appendPath(u.Path, v.AccountID)

	query := u.Query()
	query.Add("token", url.PathEscape(v.Token))
	u.RawQuery = query.Encode()

	return []byte(u.String()), nil
}

func (v *Account) UnmarshalText(text []byte) error {
	return v.SetString(string(text))
}

func (v *Account) SetString(s string) error {
	baseURL, accountID, token, err := parseAccount(s)
	if err != nil {
		return err
	}

	v.BaseURL = baseURL
	v.AccountID = accountID
	v.Token = token
	return nil
}

func (v Account) String() string {
	b, err := v.MarshalText()
	if err != nil {
		return ""
	}

	return string(b)
}

// MaskedString returns the URL without the token.
func (v Account) MaskedString() string {
	u, err := url.Parse(v.BaseURL)
	if err != nil {
		return ""
	}

	u.Path = appendPath(u.Path, apiURLAccountPart)
	u.Path = appendPath(u.Path, v.AccountID)

	return u.String()
}

func (v Account) MarshalBinary() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *Account) UnmarshalBinary(data []byte) error {
	return v.SetString(string(data))
}

// Scan converts from a database column.
func (v *Account) Scan(data interface{}) error {
	s, ok := data.(string)
	if !ok {
		return errors.New("Peer Account value not string")
	}

	if err := v.SetString(s); err != nil {
		return errors.Wrap(err, "set string")
	}

	return nil
}

func (v Account) MarshalJSONMasked() ([]byte, error) {
	return []byte("\"Masked:" + v.MaskedString() + "\""), nil
}

// parseAccount parses an account URL and returns the baseURL, accountID, and token.
func parseAccount(accountURL string) (string, string, string, error) {
	u, err := url.Parse(accountURL)
	if err != nil {
		return "", "", "", errors.Wrap(err, "url")
	}

	query := u.Query()

	token := query.Get("token")
	query.Del("token")

	u.RawQuery = query.Encode()

	parts := strings.Split(u.Path, apiURLAccountPart)
	if len(parts) != 2 {
		return "", "", "", errors.New("Missing api account part")
	}

	u.Path = parts[0]

	if len(parts[1]) == 0 {
		return "", "", "", errors.New("Missing account id")
	}
	accountParts := strings.Split(parts[1], "/")
	accountID := accountParts[0]

	return u.String(), accountID, token, nil
}

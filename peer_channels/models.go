package peer_channels

type Channel struct {
	ID           string        `json:"id"`
	Path         string        `json:"href"`
	PublicRead   bool          `json:"public_read"`
	PublicWrite  bool          `json:"public_write"`
	Sequenced    bool          `json:"sequenced"`
	Locked       bool          `json:"locked"`
	Head         int           `json:"head"`
	Retention    Retention     `json:"retention"`
	AccessTokens []AccessToken `json:"access_tokens"`
}

type Retention struct {
	MinAgeDays int  `json:"min_age_days"`
	MaxAgeDays int  `json:"max_age_days"`
	AutoPrune  bool `json:"auto_prune"`
}

type AccessToken struct {
	ID          string `json:"id"`
	Token       string `json:"token"`
	Description string `json:"description"`
	CanRead     bool   `json:"can_read"`
	CanWrite    bool   `json:"can_write"`
}

type ChannelList struct {
	Channels []*Channel `json:"channels"`
}

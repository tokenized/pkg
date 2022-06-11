package peer_channels

type Channel struct {
	ID   string `bsor:"1" json:"id"`
	Path string `bsor:"2" json:"href"`

	// PublicRead specifies that a read token is not needed to read messages from this channel.
	// You still need a read token to mark messages as read and delete messages.
	PublicRead bool `bsor:"3" json:"public_read"`

	// PublicWrite specifies that a write token is not needed to write new messages to this channel.
	PublicWrite bool `bsor:"4" json:"public_write"`

	// Sequenced specifies that all channel messages must be marked as read before a message can be
	// written
	Sequenced bool `bsor:"5" json:"sequenced"`

	// Locked specifies a channel can not have new messages written to it.
	Locked bool `bsor:"6" json:"locked"`

	// Head is the sequence of the next message to be written.
	Head int `bsor:"7" json:"head"`

	Retention    Retention     `bsor:"8" json:"retention"`
	AccessTokens []AccessToken `bsor:"9" json:"access_tokens"`
}

type Retention struct {
	MinAgeDays int  `bsor:"1" json:"min_age_days"`
	MaxAgeDays int  `bsor:"2" json:"max_age_days"`
	AutoPrune  bool `bsor:"3" json:"auto_prune"`
}

type AccessToken struct {
	ID          string `bsor:"1" json:"id"`
	Token       string `bsor:"2" json:"token"`
	Description string `bsor:"3" json:"description"`
	CanRead     bool   `bsor:"4" json:"can_read"`
	CanWrite    bool   `bsor:"5" json:"can_write"`
}

type ChannelList struct {
	Channels []*Channel `bsor:"1" json:"channels"`
}

func (c Channel) GetWriteToken() string {
	for _, token := range c.AccessTokens {
		if token.CanWrite {
			return token.Token
		}
	}

	return ""
}

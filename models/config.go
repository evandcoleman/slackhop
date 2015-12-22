package models

type Config struct {
	Channels []Channel `json:"channels"`
}

type Channel struct {
	YearsAgo              int    `json:"years_ago"`
	SourceChannelId       string `json:"source_channel"`
	TargetChannelId       string `json:"target_channel"`
	SuppressNotifications bool   `json:"suppress_notifications"`
}

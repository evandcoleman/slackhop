package models

import "github.com/nlopes/slack"

type SlackMessage struct {
	Text        string             `json:"text"`
	Name        string             `json:"username,omitempty"`
	AvatarUrl   string             `json:"icon_url,omitempty"`
	Channel     string             `json:"channel,omitempty"`
	Attachments []slack.Attachment `json:"attachments,omitempty"`
}

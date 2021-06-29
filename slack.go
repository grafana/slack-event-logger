package main

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var (
	slackClientInstance *slack.Client
)

func GetSlackClient(config Config) (*slack.Client, error) {
	if slackClientInstance == nil {
		log.Info("Creating slack client...")
		if config.Slack.BotToken == "" {
			return nil, errors.New("please set a Slack bot token")
		}
		if config.Slack.AppToken == "" {
			return nil, errors.New("please set a Slack app token")
		}
		slackClientInstance = slack.New(config.Slack.BotToken, slack.OptionAppLevelToken(config.Slack.AppToken))
	}
	return slackClientInstance, nil
}

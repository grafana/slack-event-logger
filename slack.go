package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

// EnvSlackBotToken is used to do API calls to Slack
const EnvSlackBotToken = "SLACK_BOT_TOKEN"

// EnvSlackAppToken is used to connect to the Slack Socket Mode
const EnvSlackAppToken = "SLACK_APP_TOKEN"

var (
	slackClientInstance *slack.Client
)

func GetSlackClient() (*slack.Client, error) {
	if slackClientInstance == nil {
		log.Println("Creating slack client...")
		slackBotToken, ok := os.LookupEnv(EnvSlackBotToken)
		if !ok {
			return nil, fmt.Errorf("please set a Slack bot token in the %s env variable", EnvSlackBotToken)
		}
		slackAppToken, ok := os.LookupEnv(EnvSlackAppToken)
		if !ok {
			return nil, fmt.Errorf("please set a Slack app token in the %s env variable", EnvSlackAppToken)
		}
		slackClientInstance = slack.New(slackBotToken, slack.OptionAppLevelToken(slackAppToken))
	}
	return slackClientInstance, nil
}

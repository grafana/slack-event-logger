package main

import (
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

func GetSlackClient(config Config) (*slack.Client, error) {
	log.Info("Creating slack client...")
	if config.Slack.BotToken == "" {
		return nil, errors.New("please set a Slack bot token")
	}
	if config.Slack.AppToken == "" {
		return nil, errors.New("please set a Slack app token")
	}
	return slack.New(config.Slack.BotToken, slack.OptionAppLevelToken(config.Slack.AppToken)), nil
}

func GetConversations(client *slack.Client, channelId string, timeRange time.Duration) ([]slack.Message, error) {
	var (
		messages []slack.Message
		response *slack.GetConversationHistoryResponse
		err      error
	)
	params := &slack.GetConversationHistoryParameters{ChannelID: channelId, Inclusive: true, Oldest: fmt.Sprint(time.Now().Add(-timeRange).Unix())}
	for response == nil || response.HasMore {
		if response, err = client.GetConversationHistory(params); err != nil {
			return nil, err
		}
		messages = append(messages, response.Messages...)
		params.Cursor = response.ResponseMetaData.NextCursor
	}
	return messages, nil
}

func GetConversationReplies(client *slack.Client, channelId, threadTs string) ([]slack.Message, error) {
	var (
		messages    []slack.Message
		newMessages []slack.Message
		hasMore     bool = true
		nextCursor  string
		err         error
	)
	params := &slack.GetConversationRepliesParameters{ChannelID: channelId, Timestamp: threadTs, Inclusive: true}
	for hasMore {
		if newMessages, hasMore, nextCursor, err = client.GetConversationReplies(params); err != nil {
			return nil, err
		}
		messages = append(messages, newMessages...)
		params.Cursor = nextCursor
	}
	return messages, nil
}

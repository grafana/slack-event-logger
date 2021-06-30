package main

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/kyokomi/emoji"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

const (
	messageAdded    = "message_added"
	messagedDeleted = "message_deleted"
	messagedEdited  = "message_edited"
)

type SocketMode struct {
	config      Config
	slackClient *slack.Client
	customEmoji map[string]string
}

func NewSocketMode(config Config) (*SocketMode, error) {
	var (
		sc  = &SocketMode{config: config}
		err error
	)
	if sc.slackClient, err = GetSlackClient(config); err != nil {
		return nil, err
	}

	if sc.customEmoji, err = sc.slackClient.GetEmoji(); err != nil {
		return nil, err
	}

	return sc, nil
}

func (sc *SocketMode) Run() error {
	socketClient := socketmode.New(sc.slackClient)

	if err := sc.init(); err != nil {
		return fmt.Errorf("got an error on init: %w", err)
	}

	go startMetrics(sc.config)

	go func() {
		// See here for event examples: https://github.com/slack-go/slack/blob/master/examples/socketmode/socketmode.go
	eventLoop:
		for evt := range socketClient.Events {
			var err error

			switch evt.Type {
			case socketmode.EventTypeHello:
				log.Info("Got Slack Hello")
				continue
			case socketmode.EventTypeConnecting:
				log.Info("Connecting to Slack with Socket Mode...")
				continue
			case socketmode.EventTypeConnectionError:
				log.Error("Connection to Slack failed!")
				break eventLoop
			case socketmode.EventTypeConnected:
				log.Info("Connected to Slack with Socket Mode.")
				continue
			case socketmode.EventTypeSlashCommand:
				continue
			case socketmode.EventTypeInteractive:
				continue
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent := evt.Data.(slackevents.EventsAPIEvent)
				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent

					switch innerEvent.Data.(type) {
					case *slackevents.ReactionAddedEvent:
						reactionAdded := innerEvent.Data.(*slackevents.ReactionAddedEvent)
						err = sc.reactionEvent(reactionAdded.EventTimestamp, reactionAdded.Item.Channel, reactionAdded.User, reactionAdded.Item.Timestamp, reactionAdded.Reaction, true, true)

					case *slackevents.ReactionRemovedEvent:
						reactionRemoved := innerEvent.Data.(*slackevents.ReactionRemovedEvent)
						err = sc.reactionEvent(reactionRemoved.EventTimestamp, reactionRemoved.Item.Channel, reactionRemoved.User, reactionRemoved.Item.Timestamp, reactionRemoved.Reaction, false, true)

					case *slackevents.MessageEvent:
						message := innerEvent.Data.(*slackevents.MessageEvent)

						event := "message_added"
						text := message.Text
						threadTs := message.ThreadTimeStamp
						user := message.User
						messageId := message.ClientMsgID
						if previous := message.PreviousMessage; previous != nil {
							new := message.Message
							user = previous.User
							messageId = previous.ClientMsgID
							if new == nil || new.SubType == "tombstone" { // lol
								event = "message_deleted"
							} else {
								event = "message_edited"
								text = new.Text
							}
						} else if threadTs == "" {
							// If the thread timestamp is empty on a new message, this is a top level message
							// This is essentially a thread without replies (at this point), we can set the threadTs
							threadTs = message.TimeStamp
						}

						sc.messageEvent(event, message.EventTimeStamp.String(), message.Channel, user, messageId, message.TimeStamp, threadTs, text, true)
					default:
						err = fmt.Errorf("unhandled callback event %+v", eventsAPIEvent)
					}
				default:
					err = fmt.Errorf("unhandled events API event %+v", eventsAPIEvent)
				}
			default:
				err = fmt.Errorf("unexpected event type received: %s", evt.Type)
			}

			if evt.Request != nil {
				socketClient.Ack(*evt.Request, nil)
			}

			if err != nil {
				log.Errorf("Got an error while handling events: %v", err)
			}
		}
	}()

	return socketClient.Run()
}

func (sc *SocketMode) init() error {
	backfillRange, channels := sc.config.Slack.BackfillTimeRange, sc.config.Slack.Channels
	if backfillRange == 0 {
		log.Info("Not backfilling Slack, no time range given")
		return nil
	}
	if len(channels) == 0 {
		log.Info("Not backfilling Slack, no channels given")
		return nil
	}

	for _, channel := range sc.config.Slack.Channels {
		messages, err := GetConversations(sc.slackClient, channel, backfillRange)
		if err != nil {
			return err
		}
		for i, topLevelMessage := range messages {
			if i%50 == 0 {
				log.Infof("Backfilling conversations for channel %s (%d/%d)", channel, i, len(messages))
			}
			var threadMessages []slack.Message
			if topLevelMessage.ReplyCount > 0 {
				if threadMessages, err = GetConversationReplies(sc.slackClient, channel, topLevelMessage.ThreadTimestamp); err != nil {
					return err
				}
			} else {
				threadMessages = []slack.Message{topLevelMessage}
			}

			for _, message := range threadMessages {
				threadTs := message.ThreadTimestamp
				if threadTs == "" {
					threadTs = message.Timestamp
				}
				if err = sc.messageEvent(messageAdded, message.Timestamp, channel, message.User, message.ClientMsgID, message.Timestamp, threadTs, message.Text, false); err != nil {
					return err
				}
				for _, reaction := range message.Reactions {
					for _, user := range reaction.Users {
						if err = sc.reactionEvent(message.Timestamp, channel, user, message.Timestamp, reaction.Name, true, false); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func (sc *SocketMode) messageEvent(event, eventTimestamp, channelId, userId, messageId, messageTs, threadTs, text string, logIt bool) error {
	timestamp, err := parseTimestamp(eventTimestamp)
	if err != nil {
		return err
	}
	sc.addChannelToCache(channelId)
	sc.addUserToCache(userId)

	// Register thread durations
	if threadTs != messageTs {
		var start, end time.Time
		if start, err = parseTimestamp(threadTs); err != nil {
			return err
		}
		if end, err = parseTimestamp(messageTs); err != nil {
			return err
		}

		slack_thread_seconds.WithLabelValues(channelId, threadTs).Set(end.Sub(start).Seconds())
	}

	switch event {
	case messagedEdited:
		break
	case messagedDeleted:
		slack_user_message_count.
			WithLabelValues(channelId, userId).Dec()

		slack_thread_message_count.
			WithLabelValues(channelId, threadTs).Dec()
	default:
		slack_user_message_count.
			WithLabelValues(channelId, userId).Inc()

		slack_thread_message_count.
			WithLabelValues(channelId, threadTs).Inc()
	}

	if logIt {
		log.WithTime(timestamp).WithFields(log.Fields{
			"event":     event,
			"messageId": messageId,
			"channel":   channelId,
			"user":      userId,
			"messageTs": messageTs,
			"threadTs":  threadTs,
		}).Info()
	}

	return nil
}

func (sc *SocketMode) reactionEvent(eventTimestamp, channelId, userId, itemTimestamp, reaction string, added bool, logIt bool) error {
	event := "reaction_added"
	var inc float64 = 1
	if !added {
		event = "reaction_removed"
		inc = -1
	}
	timestamp, err := parseTimestamp(eventTimestamp)
	if err != nil {
		return err
	}
	hasEmoji := sc.addReactionToCache(reaction)
	sc.addChannelToCache(channelId)
	sc.addUserToCache(userId)

	if logIt {
		logMessage := log.WithTime(timestamp).WithFields(log.Fields{
			"event":    event,
			"channel":  channelId,
			"user":     userId,
			"itemTs":   itemTimestamp,
			"reaction": reaction,
		})
		if hasEmoji {
			logMessage = logMessage.WithField("emoji", emojiUnicodes[reaction])
		}
		logMessage.Info()
	}

	slack_message_reaction_count.WithLabelValues(reaction, userId, channelId, itemTimestamp).Add(inc)
	return nil
}

func (sc *SocketMode) addChannelToCache(channelId string) {
	if channelId == "" || channelNames[channelId] != "" {
		return
	}

	channel, err := sc.slackClient.GetConversationInfo(channelId, false)
	if err != nil {
		log.Errorf("Failed to get channel info for %s: %v", channelId, err)
	}
	channelNames[channelId] = channel.Name
	slack_channel_info.WithLabelValues(channelId, channel.Name).Set(1)
}

func (sc *SocketMode) addUserToCache(userId string) {
	if userId == "" || userNames[userId] != "" {
		return
	}

	user, err := sc.slackClient.GetUserInfo(userId)
	if err != nil {
		log.Errorf("Failed to get user info for %s: %v", userId, err)
	}
	userNames[userId] = user.Profile.DisplayName
	slack_user_info.
		WithLabelValues(userId, user.Name, user.RealName, user.Profile.DisplayName).Set(1)
}

func (sc *SocketMode) addReactionToCache(reaction string) bool {
	if reaction == "" {
		return false
	}

	if emojiUnicodes[reaction] != "" {
		return true
	}

	r := ":" + reaction + ":"
	// cache standard emojis with unicode ref
	if unicode, ok := emoji.CodeMap()[r]; ok {
		emojiUnicodes[reaction] = unicode
		slack_emoji_info.
			WithLabelValues(reaction, "", unicode).Set(1)
		return true
	}

	// cache custom emojis with URL ref
	if url, ok := sc.customEmoji[reaction]; ok {
		emojiUnicodes[reaction] = url
		slack_emoji_info.
			WithLabelValues(reaction, url, "").Set(1)
		return true
	}
	return false
}

func parseTimestamp(timestamp string) (time.Time, error) {
	timestampFloat, err := strconv.ParseFloat(timestamp, 64)
	if err != nil {
		return time.Time{}, err
	}
	sec, dec := math.Modf(timestampFloat)
	return time.Unix(int64(sec), int64(dec*(1e9))), nil
}

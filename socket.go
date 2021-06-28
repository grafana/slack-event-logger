package main

import (
	"fmt"
	"math"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type SocketMode struct {
	slackClient *slack.Client
}

func NewSocketMode() (*SocketMode, error) {
	var (
		sc  = &SocketMode{}
		err error
	)
	if sc.slackClient, err = GetSlackClient(); err != nil {
		return nil, err
	}
	return sc, nil
}

func (sc *SocketMode) Run() error {
	socketClient := socketmode.New(sc.slackClient)

	go func() {
		// See here for event examples: https://github.com/slack-go/slack/blob/master/examples/socketmode/socketmode.go
		for evt := range socketClient.Events {
			var err error

			switch evt.Type {
			case socketmode.EventTypeHello:
				log.Println("Got Slack Hello")
				continue
			case socketmode.EventTypeConnecting:
				log.Println("Connecting to Slack with Socket Mode...")
				continue
			case socketmode.EventTypeConnectionError:
				log.Fatalln("Connection to Slack failed!")
				continue
			case socketmode.EventTypeConnected:
				log.Println("Connected to Slack with Socket Mode.")
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
					logMessage := log.WithField("event", innerEvent.Type)
					var timestamp time.Time

					switch innerEvent.Data.(type) {
					case *slackevents.ReactionAddedEvent:
						reactionAdded := innerEvent.Data.(*slackevents.ReactionAddedEvent)
						timestamp, err = parseTimestamp(reactionAdded.EventTimestamp)
						if err != nil {
							continue
						}
						logMessage.WithTime(timestamp).WithFields(log.Fields{
							"channel":  reactionAdded.Item.Channel,
							"user":     reactionAdded.User,
							"itemTs":   reactionAdded.Item.Timestamp,
							"reaction": reactionAdded.Reaction,
						}).Info()

					case *slackevents.ReactionRemovedEvent:
						reactionRemoved := innerEvent.Data.(*slackevents.ReactionRemovedEvent)
						timestamp, err = parseTimestamp(reactionRemoved.EventTimestamp)
						if err != nil {
							continue
						}
						logMessage.WithTime(timestamp).WithFields(log.Fields{
							"channel":  reactionRemoved.Item.Channel,
							"user":     reactionRemoved.User,
							"itemTs":   reactionRemoved.Item.Timestamp,
							"reaction": reactionRemoved.Reaction,
						}).Info()

					case *slackevents.MessageEvent:
						message := innerEvent.Data.(*slackevents.MessageEvent)
						timestamp, err = parseTimestamp(string(message.EventTimeStamp))
						if err != nil {
							continue
						}

						text := message.Text

						logMessage = logMessage.WithTime(timestamp).WithFields(log.Fields{
							"messageId":    message.ClientMsgID,
							"channel":      message.Channel,
							"user":         message.User,
							"messageTs":    message.TimeStamp,
							"threadTs":     message.ThreadTimeStamp,
							"messageEvent": "new",
						})

						if previous := message.PreviousMessage; previous != nil {
							new := message.Message
							logMessage = logMessage.WithFields(log.Fields{"user": previous.User, "messageId": previous.ClientMsgID})
							if new == nil || new.SubType == "tombstone" { // lol
								logMessage = logMessage.WithField("messageEvent", "deleted")
							} else {
								logMessage = logMessage.WithField("messageEvent", "edited")
								text = new.Text
							}
						}

						logMessage.Info(text)

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
				logError(err)
			}
		}
	}()

	return socketClient.Run()
}

func parseTimestamp(timestamp string) (time.Time, error) {
	timestampFloat, err := strconv.ParseFloat(timestamp, 64)
	if err != nil {
		return time.Time{}, err
	}
	sec, dec := math.Modf(timestampFloat)
	return time.Unix(int64(sec), int64(dec*(1e9))), nil
}

func logError(err error) {
	if err != nil {
		log.Printf("Got an error while handling events: %v", err)
	}
}

package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type SocketMode struct {
	slackClient *slack.Client
}

var (
	slack_user_message_count = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_user_message_count",
			Help: "Number of messages a single User sent to a channel",
		}, []string{"channel", "user"})

	slack_thread_message_count = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_thread_message_count",
			Help: "Number of messages in a single thread in a channel",
		}, []string{"channel", "threadTs"})

	slack_message_reaction_count = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_message_reaction_count",
			Help: "Number of reactions on a single message",
		}, []string{"channel", "messageTs"})

	slack_thread_seconds = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_thread_seconds",
			Help: "The amount of seconds between the first and last message of a thread",
		}, []string{"threadTs"})

	slack_channel_info = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_channel_info",
			Help: "Information about the channel",
		}, []string{"channel", "name"})

	channelNames map[string]string
)

func NewSocketMode(config Config) (*SocketMode, error) {
	var (
		sc  = &SocketMode{}
		err error
	)
	if sc.slackClient, err = GetSlackClient(config); err != nil {
		return nil, err
	}
	return sc, nil
}

func (sc *SocketMode) Run() error {
	socketClient := socketmode.New(sc.slackClient)

	go startMetrics()

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
					logMessage := log.WithField("event", innerEvent.Type)
					var timestamp time.Time

					switch innerEvent.Data.(type) {
					case *slackevents.ReactionAddedEvent:
						reactionAdded := innerEvent.Data.(*slackevents.ReactionAddedEvent)
						timestamp, err = parseTimestamp(reactionAdded.EventTimestamp)
						if err != nil {
							break
						}

						slack_message_reaction_count.
							WithLabelValues(reactionAdded.Item.Channel, reactionAdded.Item.Timestamp).Inc()

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
							break
						}
						logMessage.WithTime(timestamp).WithFields(log.Fields{
							"channel":  reactionRemoved.Item.Channel,
							"user":     reactionRemoved.User,
							"itemTs":   reactionRemoved.Item.Timestamp,
							"reaction": reactionRemoved.Reaction,
						}).Info()

						slack_message_reaction_count.
							WithLabelValues(reactionRemoved.Item.Channel, reactionRemoved.Item.Timestamp).Dec()

					case *slackevents.MessageEvent:
						message := innerEvent.Data.(*slackevents.MessageEvent)
						timestamp, err = parseTimestamp(string(message.EventTimeStamp))
						if err != nil {
							break
						}

						if channelNames[message.Channel] == "" {
							channel, err := sc.slackClient.GetConversationInfo(message.Channel, false)
							if err != nil {
								log.Errorln("Failed to get channel info", err)
							}
							channelNames[message.Channel] = channel.Name
							slack_channel_info.
								WithLabelValues(message.Channel, channel.Name).Set(1)
						}

						text := message.Text

						logMessage = logMessage.WithTime(timestamp).WithFields(log.Fields{
							"messageId": message.ClientMsgID,
							"channel":   message.Channel,
							"user":      message.User,
							"messageTs": message.TimeStamp,
						})

						messageEvent := "new"
						threadTs := message.ThreadTimeStamp
						if previous := message.PreviousMessage; previous != nil {
							new := message.Message
							logMessage = logMessage.WithFields(log.Fields{"user": previous.User, "messageId": previous.ClientMsgID})
							if new == nil || new.SubType == "tombstone" { // lol
								messageEvent = "deleted"
							} else {
								messageEvent = "edited"
								text = new.Text
							}
						} else {
							// If the thread timestamp is empty on a new message, this is a top level message
							// This is essentially a thread without replies (at this point), we can set the threadTs
							if threadTs == "" {
								threadTs = message.TimeStamp
							} else {
								var start, end time.Time
								start, err = parseTimestamp(threadTs)
								if err != nil {
									break
								}
								end, err = parseTimestamp(message.TimeStamp)
								if err != nil {
									break
								}

								slack_thread_seconds.WithLabelValues(threadTs).Set(end.Sub(start).Seconds())
							}
						}
						logMessage = logMessage.WithField("messageEvent", messageEvent)
						logMessage = logMessage.WithField("threadTs", threadTs)

						switch messageEvent {
						case "edited":
							continue
						case "deleted":
							slack_user_message_count.
								WithLabelValues(message.Channel, message.User).Dec()

							slack_thread_message_count.
								WithLabelValues(message.Channel, threadTs).Dec()
						default:
							slack_user_message_count.
								WithLabelValues(message.Channel, message.User).Inc()

							slack_thread_message_count.
								WithLabelValues(message.Channel, threadTs).Inc()
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
				log.Errorf("Got an error while handling events: %v", err)
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

func startMetrics() {
	log.Println("Exposing metrics on :8080")
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("http server stopped: %s", err)
	}
}

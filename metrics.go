package main

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
		}, []string{"reaction", "user", "channel", "messageTs"})

	slack_thread_seconds = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_thread_seconds",
			Help: "The amount of seconds between the first and last message of a thread",
		}, []string{"channel", "threadTs"})

	slack_channel_info = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_channel_info",
			Help: "Information about the channel",
		}, []string{"channel", "name"})

	slack_user_info = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_user_info",
			Help: "Information about the channel",
		}, []string{"user", "name", "realname", "displayname"})

	slack_emoji_info = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slack_emoji_info",
			Help: "Information about emoji",
		}, []string{"reaction", "url", "unicode"})

	channelNames  map[string]string = map[string]string{}
	userNames     map[string]string = map[string]string{}
	emojiUnicodes map[string]string = map[string]string{}
)

func startMetrics(config Config) {
	port := fmt.Sprintf(":%d", config.Metrics.Port)

	log.Infof("Exposing metrics on %s", port)
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("http server stopped: %s", err)
	}
}

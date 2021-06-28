package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/grafana/slack-event-logger/internal/socket"
)

func main() {
	socket, err := socket.NewSocketMode()
	if err != nil {
		log.Fatalf("Failed to init: %v", err)
	}
	if err := socket.Run(); err != nil {
		log.Fatalf("Failed to run: %v", err)
	}
}

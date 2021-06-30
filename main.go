package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.WithError(err).Fatalf("Failed to load config: %v", err)
	}
	if config.LoggingFormat == "logfmt" {
		log.SetFormatter(&log.TextFormatter{})
	} else if config.LoggingFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.WithError(err).Fatalf("Invalid log format: %s", config.LoggingFormat)
	}

	socket, err := NewSocketMode(config)
	if err != nil {
		log.WithError(err).Fatalf("Failed to init: %v", err)
	}
	if err := socket.Run(); err != nil {
		log.WithError(err).Fatalf("Failed to run: %v", err)
	}
}

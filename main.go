package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

const EnvLoggingFormat = "LOGGING_FORMAT"

func main() {
	if logFormat := os.Getenv(EnvLoggingFormat); logFormat == "logfmt" || logFormat == "" {
		log.SetFormatter(&log.TextFormatter{})
	} else if logFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.Fatalf("Invalid log format: %s", logFormat)
	}

	socket, err := NewSocketMode()
	if err != nil {
		log.Fatalf("Failed to init: %v", err)
	}
	if err := socket.Run(); err != nil {
		log.Fatalf("Failed to run: %v", err)
	}
}

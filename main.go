package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	socket, err := NewSocketMode()
	if err != nil {
		log.Fatalf("Failed to init: %v", err)
	}
	if err := socket.Run(); err != nil {
		log.Fatalf("Failed to run: %v", err)
	}
}

package main

import (
	"github.com/kelseyhightower/envconfig"
)

type (
	Config struct {
		LoggingFormat string `envconfig:"LOGGING_FORMAT" default:"logfmt"`
		Metrics       struct {
			Port int `envconfig:"METRICS_PORT" default:"8080"`
		}
		Slack struct {
			AppToken string `envconfig:"SLACK_APP_TOKEN"`
			BotToken string `envconfig:"SLACK_BOT_TOKEN"`
		}
	}
)

// LoadConfig loads the configuration from the environment.
func LoadConfig() (Config, error) {
	config := Config{}
	if err := envconfig.Process("", &config); err != nil {
		return config, err
	}
	return config, nil
}

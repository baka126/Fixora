package config

import (
	"os"
)

type OperatingMode string

const (
	DryRun     OperatingMode = "dry-run"
	ClickToFix OperatingMode = "click-to-fix"
	AutoFix    OperatingMode = "auto-fix"
)

type Config struct {
	SlackToken   string
	SlackChannel string
	Mode         OperatingMode
}

func Load() *Config {
	mode := OperatingMode(os.Getenv("FIXORA_MODE"))
	if mode == "" {
		mode = AutoFix
	}

	return &Config{
		SlackToken:   os.Getenv("SLACK_TOKEN"),
		SlackChannel: os.Getenv("SLACK_CHANNEL"),
		Mode:         mode,
	}
}

package config

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrMissingPulseURL indicates Pulse API URL is missing.
	ErrMissingPulseURL = errors.New("bridge config: PulseBaseURL must be provided")
	// ErrMissingAppID indicates AppID is missing.
	ErrMissingAppID = errors.New("bridge config: PulseAppID must be provided")
	// ErrMissingAppSecret indicates AppSecret is missing.
	ErrMissingAppSecret = errors.New("bridge config: PulseAppSecret must be at least 16 characters")
)

// BridgeConfig specifies runtime credentials for Pulse and connected external platforms.
type BridgeConfig struct {
	PulseBaseURL     string
	PulseAppID       string
	PulseAppSecret   string
	ListenPort       int
	TelegramBotToken string
	SlackBotToken    string
	DiscordBotToken  string
	Timeout          time.Duration
}

// Validate verifies that core connection invariants are satisfied with zero fallback defaults.
func (c *BridgeConfig) Validate() error {
	if strings.TrimSpace(c.PulseBaseURL) == "" {
		return ErrMissingPulseURL
	}
	if strings.TrimSpace(c.PulseAppID) == "" {
		return ErrMissingAppID
	}
	if len(strings.TrimSpace(c.PulseAppSecret)) < 16 {
		return ErrMissingAppSecret
	}
	if c.ListenPort <= 0 {
		c.ListenPort = 32080
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	return nil
}


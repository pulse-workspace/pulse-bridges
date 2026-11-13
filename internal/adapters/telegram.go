package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pulse-platform/pulse-bridges/internal/bridge"
)

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
		} `json:"from"`
		Chat struct {
			ID    int64  `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title,omitempty"`
		} `json:"chat"`
		Date int64  `json:"date"`
		Text string `json:"text,omitempty"`
	} `json:"message"`
}

// TelegramAdapter bridges messages between Telegram Bot API and Pulse.
type TelegramAdapter struct {
	botToken   string
	apiBaseURL string
	client     *http.Client
}

// NewTelegramAdapter instantiates a TelegramAdapter with bot credentials.
func NewTelegramAdapter(botToken string) *TelegramAdapter {
	return &TelegramAdapter{
		botToken:   botToken,
		apiBaseURL: "https://api.telegram.org",
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Platform returns bridge.PlatformTelegram.
func (a *TelegramAdapter) Platform() bridge.Platform {
	return bridge.PlatformTelegram
}

// TranslateInbound decodes raw Telegram webhook updates into normalized InboundEvents.
func (a *TelegramAdapter) TranslateInbound(ctx context.Context, payload []byte, headers map[string]string) (*bridge.InboundEvent, error) {
	var update telegramUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, fmt.Errorf("telegram: invalid update payload: %w", err)
	}

	if update.Message == nil || update.Message.Text == "" {
		return nil, nil // Ignore non-text updates or service notifications
	}

	senderName := update.Message.From.FirstName
	if update.Message.From.LastName != "" {
		senderName += " " + update.Message.From.LastName
	}
	if senderName == "" {
		senderName = update.Message.From.Username
	}

	return &bridge.InboundEvent{
		Platform:       bridge.PlatformTelegram,
		ExternalChatID: strconv.FormatInt(update.Message.Chat.ID, 10),
		ExternalUserID: strconv.FormatInt(update.Message.From.ID, 10),
		SenderName:     senderName,
		Body:           update.Message.Text,
		Timestamp:      time.Unix(update.Message.Date, 0),
	}, nil
}

// DeliverOutbound posts a Pulse message to the target Telegram chat room.
func (a *TelegramAdapter) DeliverOutbound(ctx context.Context, event *bridge.OutboundEvent) error {
	chatID, err := strconv.ParseInt(event.ExternalChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %s: %w", event.ExternalChatID, err)
	}

	text := fmt.Sprintf("*%s*: %s", event.SenderName, event.Body)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	})

	url := fmt.Sprintf("%s/bot%s/sendMessage", a.apiBaseURL, a.botToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: dispatch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: API error status %d", resp.StatusCode)
	}

	return nil
}


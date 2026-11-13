package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pulse-platform/pulse-bridges/internal/bridge"
)

type slackEventPayload struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge,omitempty"`
	Event     *struct {
		Type    string `json:"type"`
		User    string `json:"user"`
		Text    string `json:"text"`
		Channel string `json:"channel"`
		Ts      string `json:"ts"`
		BotID   string `json:"bot_id,omitempty"`
	} `json:"event,omitempty"`
}

// SlackAdapter connects Pulse with Slack workspaces via Events API and Web API.
type SlackAdapter struct {
	botToken   string
	apiBaseURL string
	client     *http.Client
}

// NewSlackAdapter instantiates a SlackAdapter with bot credentials.
func NewSlackAdapter(botToken string) *SlackAdapter {
	return &SlackAdapter{
		botToken:   botToken,
		apiBaseURL: "https://slack.com/api",
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Platform returns bridge.PlatformSlack.
func (a *SlackAdapter) Platform() bridge.Platform {
	return bridge.PlatformSlack
}

// TranslateInbound translates Slack event callbacks into normalized InboundEvents.
func (a *SlackAdapter) TranslateInbound(ctx context.Context, payload []byte, headers map[string]string) (*bridge.InboundEvent, error) {
	var body slackEventPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("slack: invalid event payload: %w", err)
	}

	// Ignore bot echoes to prevent infinite messaging loops
	if body.Event == nil || body.Event.Type != "message" || body.Event.BotID != "" {
		return nil, nil
	}

	return &bridge.InboundEvent{
		Platform:       bridge.PlatformSlack,
		ExternalChatID: body.Event.Channel,
		ExternalUserID: body.Event.User,
		SenderName:     body.Event.User,
		Body:           body.Event.Text,
		Timestamp:      time.Now(),
	}, nil
}

// DeliverOutbound transmits a Pulse message to a Slack channel via chat.postMessage.
func (a *SlackAdapter) DeliverOutbound(ctx context.Context, event *bridge.OutboundEvent) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"channel":  event.ExternalChatID,
		"text":     fmt.Sprintf("*%s*: %s", event.SenderName, event.Body),
		"username": event.SenderName,
	})

	url := fmt.Sprintf("%s/chat.postMessage", a.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.botToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: postMessage failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: API error status %d", resp.StatusCode)
	}

	return nil
}


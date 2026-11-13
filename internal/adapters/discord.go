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

type discordWebhookPayload struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

// DiscordAdapter connects Pulse channels with Discord servers and webhook integrations.
type DiscordAdapter struct {
	botToken   string
	apiBaseURL string
	client     *http.Client
}

// NewDiscordAdapter instantiates a DiscordAdapter with bot credentials.
func NewDiscordAdapter(botToken string) *DiscordAdapter {
	return &DiscordAdapter{
		botToken:   botToken,
		apiBaseURL: "https://discord.com/api/v10",
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Platform returns bridge.PlatformDiscord.
func (a *DiscordAdapter) Platform() bridge.Platform {
	return bridge.PlatformDiscord
}

// TranslateInbound decodes Discord webhook payloads into normalized InboundEvents.
func (a *DiscordAdapter) TranslateInbound(ctx context.Context, payload []byte, headers map[string]string) (*bridge.InboundEvent, error) {
	var body discordWebhookPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("discord: invalid webhook payload: %w", err)
	}

	if body.Author.Bot {
		return nil, nil // Prevent bot loops
	}

	return &bridge.InboundEvent{
		Platform:       bridge.PlatformDiscord,
		ExternalChatID: body.ChannelID,
		ExternalUserID: body.Author.ID,
		SenderName:     body.Author.Username,
		Body:           body.Content,
		Timestamp:      time.Now(),
	}, nil
}

// DeliverOutbound posts a Pulse message to a Discord channel.
func (a *DiscordAdapter) DeliverOutbound(ctx context.Context, event *bridge.OutboundEvent) error {
	content := fmt.Sprintf("**%s**: %s", event.SenderName, event.Body)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"content": content,
	})

	url := fmt.Sprintf("%s/channels/%s/messages", a.apiBaseURL, event.ExternalChatID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+a.botToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: delivery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: API error status %d", resp.StatusCode)
	}

	return nil
}


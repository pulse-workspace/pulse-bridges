package bridge

import (
	"context"
	"time"
)

// Platform denotes supported third-party chat platforms.
type Platform string

const (
	PlatformTelegram Platform = "telegram"
	PlatformSlack    Platform = "slack"
	PlatformDiscord  Platform = "discord"
	PlatformMatrix   Platform = "matrix"
	PlatformWhatsApp Platform = "whatsapp"
)

// InboundEvent represents a normalized message event originating from an external platform.
type InboundEvent struct {
	Platform       Platform               `json:"platform"`
	ExternalChatID string                 `json:"externalChatId"`
	ExternalUserID string                 `json:"externalUserId"`
	SenderName     string                 `json:"senderName"`
	Body           string                 `json:"body"`
	MediaURL       string                 `json:"mediaUrl,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Raw            map[string]interface{} `json:"raw,omitempty"`
}

// OutboundEvent represents a normalized message originating from Pulse destined for an external platform.
type OutboundEvent struct {
	Platform       Platform               `json:"platform"`
	PulseConvID    string                 `json:"pulseConvId"`
	ExternalChatID string                 `json:"externalChatId"`
	SenderID       string                 `json:"senderId"`
	SenderName     string                 `json:"senderName"`
	Body           string                 `json:"body"`
	MediaURL       string                 `json:"mediaUrl,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// IdentityMapping associates an external platform chat/user with a Pulse conversation/user.
type IdentityMapping struct {
	Platform       Platform  `json:"platform"`
	ExternalChatID string    `json:"externalChatId"`
	PulseConvID    string    `json:"pulseConvId"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Adapter defines the contract for bidirectional translation with external messaging platforms.
type Adapter interface {
	// Platform returns the unique platform identifier.
	Platform() Platform
	// TranslateInbound translates raw webhook bytes into a normalized InboundEvent.
	TranslateInbound(ctx context.Context, payload []byte, headers map[string]string) (*InboundEvent, error)
	// DeliverOutbound transmits a Pulse-originated message to the external chat room.
	DeliverOutbound(ctx context.Context, event *OutboundEvent) error
}


package bridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/pulse-platform/pulse-server-sdk-go"
)

// Router orchestrates bidirectional message flows and maintains identity mappings.
type Router struct {
	mu           sync.RWMutex
	adapters     map[Platform]Adapter
	chatMappings map[string]string // Key: "platform:externalChatID", Value: PulseConvID
	pulseClient  *pulse.Client
}

// NewRouter instantiates the central bridge routing engine.
func NewRouter(pulseClient *pulse.Client) *Router {
	return &Router{
		adapters:     make(map[Platform]Adapter),
		chatMappings: make(map[string]string),
		pulseClient:  pulseClient,
	}
}

// RegisterAdapter mounts a platform adapter to the router.
func (r *Router) RegisterAdapter(adapter Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Platform()] = adapter
}

// BindChat links an external chat room with a Pulse conversation ID.
func (r *Router) BindChat(platform Platform, externalChatID, pulseConvID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", platform, externalChatID)
	r.chatMappings[key] = pulseConvID
}

// GetBoundPulseConvID resolves the Pulse conversation linked to an external chat.
func (r *Router) GetBoundPulseConvID(platform Platform, externalChatID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", platform, externalChatID)
	convID, ok := r.chatMappings[key]
	return convID, ok
}

// HandleInbound processes incoming webhook payloads from external platforms and routes them to Pulse.
func (r *Router) HandleInbound(ctx context.Context, platform Platform, payload []byte, headers map[string]string) error {
	r.mu.RLock()
	adapter, exists := r.adapters[platform]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("router: no adapter registered for platform %s", platform)
	}

	event, err := adapter.TranslateInbound(ctx, payload, headers)
	if err != nil {
		return fmt.Errorf("router: inbound translation error: %w", err)
	}
	if event == nil {
		return nil // Ignored event (echo or non-text)
	}

	pulseConvID, bound := r.GetBoundPulseConvID(platform, event.ExternalChatID)
	if !bound {
		// Auto-bind or create conversation if not present
		pulseConvID = fmt.Sprintf("conv_bridge_%s_%s", platform, event.ExternalChatID)
		r.BindChat(platform, event.ExternalChatID, pulseConvID)
	}

	// Dispatch message into Pulse via Server SDK
	_, err = r.pulseClient.Messages.SendSystemMessage(ctx, pulse.SystemMessageRequest{
		ConvID: pulseConvID,
		Body:   fmt.Sprintf("[%s] %s: %s", platform, event.SenderName, event.Body),
		Metadata: map[string]interface{}{
			"bridgePlatform": string(platform),
			"externalUserId": event.ExternalUserID,
			"externalChatId": event.ExternalChatID,
		},
	})
	if err != nil {
		return fmt.Errorf("router: failed to publish to Pulse: %w", err)
	}

	return nil
}

// HandleOutbound receives a message from Pulse and forwards it to the target external platform.
func (r *Router) HandleOutbound(ctx context.Context, event *OutboundEvent) error {
	r.mu.RLock()
	adapter, exists := r.adapters[event.Platform]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("router: no adapter registered for platform %s", event.Platform)
	}

	return adapter.DeliverOutbound(ctx, event)
}


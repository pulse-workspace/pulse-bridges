package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pulse-platform/pulse-bridges/internal/adapters"
	"github.com/pulse-platform/pulse-bridges/internal/bridge"
	"github.com/pulse-platform/pulse-server-sdk-go"
)

type mockAdapter struct {
	platform   bridge.Platform
	lastInbound *bridge.InboundEvent
	outboundErr error
}

func (m *mockAdapter) Platform() bridge.Platform {
	return m.platform
}

func (m *mockAdapter) TranslateInbound(ctx context.Context, payload []byte, headers map[string]string) (*bridge.InboundEvent, error) {
	ev := &bridge.InboundEvent{
		Platform:       m.platform,
		ExternalChatID: "chat_123",
		ExternalUserID: "user_456",
		SenderName:     "Test User",
		Body:           string(payload),
		Timestamp:      time.Now(),
	}
	m.lastInbound = ev
	return ev, nil
}

func (m *mockAdapter) DeliverOutbound(ctx context.Context, event *bridge.OutboundEvent) error {
	return m.outboundErr
}

func TestRouter_InboundAndOutbound(t *testing.T) {
	// Mock Pulse backend HTTP server
	pulseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"msgId":"msg_bridge_01","convId":"conv_123","convSeq":1,"senderId":"system","body":"test","createdAt":"2026-11-13T10:00:00Z"}`))
	}))
	defer pulseServer.Close()

	pulseClient, err := pulse.NewClient(pulse.Config{
		BaseURL:   pulseServer.URL,
		AppID:     "app_test",
		AppSecret: "0123456789abcdef0123456789abcdef",
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to init pulse client: %v", err)
	}

	router := bridge.NewRouter(pulseClient)
	mock := &mockAdapter{platform: bridge.PlatformTelegram}
	router.RegisterAdapter(mock)

	t.Run("bind chat and route inbound", func(t *testing.T) {
		router.BindChat(bridge.PlatformTelegram, "chat_123", "conv_pulse_123")
		convID, ok := router.GetBoundPulseConvID(bridge.PlatformTelegram, "chat_123")
		if !ok || convID != "conv_pulse_123" {
			t.Errorf("expected bound convID conv_pulse_123, got %s", convID)
		}

		err := router.HandleInbound(context.Background(), bridge.PlatformTelegram, []byte("hello from telegram"), nil)
		if err != nil {
			t.Fatalf("failed to handle inbound: %v", err)
		}

		if mock.lastInbound == nil || mock.lastInbound.Body != "hello from telegram" {
			t.Errorf("expected inbound body to be recorded")
		}
	})

	t.Run("route outbound message", func(t *testing.T) {
		outbound := &bridge.OutboundEvent{
			Platform:       bridge.PlatformTelegram,
			PulseConvID:    "conv_pulse_123",
			ExternalChatID: "chat_123",
			SenderName:     "Alice",
			Body:           "reply from pulse",
			Timestamp:      time.Now(),
		}

		err := router.HandleOutbound(context.Background(), outbound)
		if err != nil {
			t.Fatalf("failed to handle outbound: %v", err)
		}
	})
}

func TestTelegramAdapter_Translate(t *testing.T) {
	adapter := adapters.NewTelegramAdapter("mock_bot_token")
	payload := []byte(`{
		"update_id": 1001,
		"message": {
			"message_id": 42,
			"from": { "id": 999, "first_name": "Bob", "username": "bobsmith" },
			"chat": { "id": -100123456, "type": "supergroup", "title": "Dev Ops" },
			"date": 1700000000,
			"text": "Status report: all systems normal."
		}
	}`)

	event, err := adapter.TranslateInbound(context.Background(), payload, nil)
	if err != nil {
		t.Fatalf("unexpected translation error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event not to be nil")
	}

	if event.SenderName != "Bob" {
		t.Errorf("expected sender Bob, got %s", event.SenderName)
	}
	if event.Body != "Status report: all systems normal." {
		t.Errorf("expected message body, got %s", event.Body)
	}
	if event.ExternalChatID != "-100123456" {
		t.Errorf("expected externalChatId -100123456, got %s", event.ExternalChatID)
	}
}


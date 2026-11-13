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

type matrixTransaction struct {
	Events []struct {
		Type    string `json:"type"`
		Sender  string `json:"sender"`
		RoomID  string `json:"room_id"`
		EventID string `json:"event_id"`
		Content struct {
			Body    string `json:"body"`
			MsgType string `json:"msgtype"`
		} `json:"content"`
	} `json:"events"`
}

// MatrixAdapter connects Pulse with Matrix homeserver federations via AS API.
type MatrixAdapter struct {
	homeserverURL string
	asToken       string
	client        *http.Client
}

// NewMatrixAdapter instantiates a MatrixAdapter.
func NewMatrixAdapter(homeserverURL, asToken string) *MatrixAdapter {
	return &MatrixAdapter{
		homeserverURL: homeserverURL,
		asToken:       asToken,
		client:        &http.Client{Timeout: 10 * time.Second},
	}
}

// Platform returns bridge.PlatformMatrix.
func (a *MatrixAdapter) Platform() bridge.Platform {
	return bridge.PlatformMatrix
}

// TranslateInbound translates Matrix event transactions into InboundEvents.
func (a *MatrixAdapter) TranslateInbound(ctx context.Context, payload []byte, headers map[string]string) (*bridge.InboundEvent, error) {
	var txn matrixTransaction
	if err := json.Unmarshal(payload, &txn); err != nil {
		return nil, fmt.Errorf("matrix: invalid transaction: %w", err)
	}

	for _, ev := range txn.Events {
		if ev.Type == "m.room.message" && ev.Content.MsgType == "m.text" {
			return &bridge.InboundEvent{
				Platform:       bridge.PlatformMatrix,
				ExternalChatID: ev.RoomID,
				ExternalUserID: ev.Sender,
				SenderName:     ev.Sender,
				Body:           ev.Content.Body,
				Timestamp:      time.Now(),
			}, nil
		}
	}

	return nil, nil
}

// DeliverOutbound transmits a Pulse message to a Matrix room.
func (a *MatrixAdapter) DeliverOutbound(ctx context.Context, event *bridge.OutboundEvent) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"msgtype": "m.text",
		"body":    fmt.Sprintf("[%s]: %s", event.SenderName, event.Body),
	})

	txnID := fmt.Sprintf("pulse_%d", time.Now().UnixNano())
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s?access_token=%s",
		a.homeserverURL, event.ExternalChatID, txnID, a.asToken)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("matrix: delivery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("matrix: API error status %d", resp.StatusCode)
	}

	return nil
}


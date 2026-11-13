package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pulse-platform/pulse-bridges/internal/adapters"
	"github.com/pulse-platform/pulse-bridges/internal/bridge"
	"github.com/pulse-platform/pulse-bridges/internal/config"
	"github.com/pulse-platform/pulse-server-sdk-go"
)

func main() {
	cfg := config.BridgeConfig{
		PulseBaseURL:     getEnv("PULSE_API_URL", "http://localhost:32011"),
		PulseAppID:       getEnv("PULSE_APP_ID", "app_bridge_daemon"),
		PulseAppSecret:   getEnv("PULSE_APP_SECRET", "0123456789abcdef0123456789abcdef"),
		ListenPort:       getEnvInt("BRIDGE_PORT", 32080),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		SlackBotToken:    os.Getenv("SLACK_BOT_TOKEN"),
		DiscordBotToken:  os.Getenv("DISCORD_BOT_TOKEN"),
		Timeout:          10 * time.Second,
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("pulse-bridges: invalid configuration: %v", err)
	}

	pulseClient, err := pulse.NewClient(pulse.Config{
		BaseURL:    cfg.PulseBaseURL,
		AppID:      cfg.PulseAppID,
		AppSecret:  cfg.PulseAppSecret,
		Timeout:    cfg.Timeout,
		MaxRetries: 3,
	})
	if err != nil {
		log.Fatalf("pulse-bridges: failed to initialize Pulse client: %v", err)
	}

	router := bridge.NewRouter(pulseClient)
	if cfg.TelegramBotToken != "" {
		router.RegisterAdapter(adapters.NewTelegramAdapter(cfg.TelegramBotToken))
	}
	if cfg.SlackBotToken != "" {
		router.RegisterAdapter(adapters.NewSlackAdapter(cfg.SlackBotToken))
	}
	if cfg.DiscordBotToken != "" {
		router.RegisterAdapter(adapters.NewDiscordAdapter(cfg.DiscordBotToken))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// Platform Webhook Handlers
	mux.HandleFunc("/webhooks/telegram", createWebhookHandler(router, bridge.PlatformTelegram))
	mux.HandleFunc("/webhooks/slack", createWebhookHandler(router, bridge.PlatformSlack))
	mux.HandleFunc("/webhooks/discord", createWebhookHandler(router, bridge.PlatformDiscord))

	// Inbound Webhook from Pulse to forward to external platforms
	pulseWebhookHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var event bridge.OutboundEvent
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, `{"error":"invalid event"}`, http.StatusBadRequest)
			return
		}
		_ = router.HandleOutbound(r.Context(), &event)
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/webhooks/pulse", pulse.WebhookMiddleware(cfg.PulseAppSecret)(pulseWebhookHandler))

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.ListenPort),
		Handler: mux,
	}

	go func() {
		log.Printf("pulse-bridges daemon running on port %d...", cfg.ListenPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("pulse-bridges server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	log.Println("pulse-bridges gracefully terminated.")
}

func createWebhookHandler(router *bridge.Router, platform bridge.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
			return
		}
		headers := make(map[string]string)
		for k, v := range r.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		if err := router.HandleInbound(r.Context(), platform, body, headers); err != nil {
			log.Printf("bridge error [%s]: %v", platform, err)
			http.Error(w, `{"error":"bridge dispatch failed"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}


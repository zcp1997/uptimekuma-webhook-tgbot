package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	maxPayloadBytes       = 1 << 20 // 1 MiB
	defaultTelegramAPIURL = "https://api.telegram.org"
	defaultListenAddr     = ":8080"
)

var defaultRequestTimeout = 10 * time.Second

type config struct {
	listenAddr       string
	webhookToken     string
	telegramBotToken string
	telegramChatID   string
	telegramBaseURL  string
	requestTimeout   time.Duration
}

type telegramClient struct {
	baseURL        string
	botToken       string
	chatID         string
	httpClient     *http.Client
	requestTimeout time.Duration
}

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("warning: %v", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	client := &telegramClient{
		baseURL:        strings.TrimSuffix(cfg.telegramBaseURL, "/"),
		botToken:       cfg.telegramBotToken,
		chatID:         cfg.telegramChatID,
		requestTimeout: cfg.requestTimeout,
		httpClient:     &http.Client{Timeout: cfg.requestTimeout},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/uptimekuma-webhook", webhookHandler(cfg, client))

	server := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", cfg.listenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func loadConfig() (config, error) {
	cfg := config{
		listenAddr:      getEnv("LISTEN_ADDR", defaultListenAddr),
		telegramBaseURL: getEnv("TELEGRAM_API_BASE_URL", defaultTelegramAPIURL),
		requestTimeout:  defaultRequestTimeout,
	}

	cfg.webhookToken = strings.TrimSpace(os.Getenv("WEBHOOK_AUTH_TOKEN"))
	cfg.telegramBotToken = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	cfg.telegramChatID = strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))

	if cfg.webhookToken == "" {
		return config{}, errors.New("WEBHOOK_AUTH_TOKEN is required")
	}
	if cfg.telegramBotToken == "" {
		return config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.telegramChatID == "" {
		return config{}, errors.New("TELEGRAM_CHAT_ID is required")
	}

	if timeoutStr := strings.TrimSpace(os.Getenv("REQUEST_TIMEOUT")); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return config{}, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
		}
		if timeout <= 0 {
			return config{}, errors.New("REQUEST_TIMEOUT must be positive")
		}
		cfg.requestTimeout = timeout
	}

	return cfg, nil
}

func webhookHandler(cfg config, client *telegramClient) http.HandlerFunc {
	expectedAuthHeader := "Bearer " + cfg.webhookToken

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("Authorization") != expectedAuthHeader {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadBytes))
		if err != nil {
			log.Printf("failed to read request body: %v", err)
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if len(body) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}

		payload := map[string]any{}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			log.Printf("invalid JSON payload: %v", err)
		}

		message := buildTelegramMessage(payload, body)
		ctx, cancel := context.WithTimeout(r.Context(), client.requestTimeout)
		defer cancel()

		if err := client.sendMessage(ctx, message); err != nil {
			log.Printf("failed to send telegram message: %v", err)
			http.Error(w, "failed to forward notification", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

func buildTelegramMessage(payload map[string]any, raw []byte) string {
	var builder strings.Builder

	monitorName := nestedString(payload, "monitor", "name")
	if monitorName != "" {
		builder.WriteString("Monitor: ")
		builder.WriteString(monitorName)
		builder.WriteByte('\n')
	}

	status := stringFromMap(payload, "status")
	if status != "" {
		builder.WriteString("Status: ")
		builder.WriteString(strings.ToUpper(status))
		builder.WriteByte('\n')
	}

	reason := stringFromMap(payload, "incidentReason")
	if reason == "" {
		reason = stringFromMap(payload, "msg")
	}
	if reason != "" {
		builder.WriteString("Reason: ")
		builder.WriteString(reason)
		builder.WriteByte('\n')
	}

	monitorURL := nestedString(payload, "monitor", "url")
	if monitorURL != "" {
		builder.WriteString("URL: ")
		builder.WriteString(monitorURL)
		builder.WriteByte('\n')
	}

	timestamp := stringFromMap(payload, "time")
	if timestamp != "" {
		builder.WriteString("Time: ")
		builder.WriteString(timestamp)
		builder.WriteByte('\n')
	}

	ping := stringFromMap(payload, "ping")
	if ping != "" {
		builder.WriteString("Ping: ")
		builder.WriteString(ping)
		builder.WriteByte('\n')
	}

	text := strings.TrimSpace(builder.String())
	if text == "" {
		return fallbackRaw(raw)
	}

	rawText := fallbackRaw(raw)
	if rawText != "" {
		text = text + "\n---\n" + rawText
	}

	return text
}

func fallbackRaw(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	const maxRaw = 3900
	if len(trimmed) > maxRaw {
		return trimmed[:maxRaw] + "..."
	}
	return trimmed
}

func nestedString(payload map[string]any, keys ...string) string {
	current := any(payload)
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[key]
		if !ok {
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", v))
	default:
		return ""
	}
}

func stringFromMap(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", v))
	default:
		return ""
	}
}

func (c *telegramClient) sendMessage(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("telegram message is empty")
	}

	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.botToken)
	payload := map[string]any{
		"chat_id":                  c.chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}
	if !response.OK {
		if response.Description == "" {
			response.Description = "unknown error"
		}
		return fmt.Errorf("telegram API error: %s", response.Description)
	}

	return nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[len("export "):])
		}

		sep := strings.Index(line, "=")
		if sep < 0 {
			continue
		}

		key := strings.TrimSpace(line[:sep])
		if key == "" {
			continue
		}

		value := strings.TrimSpace(line[sep+1:])
		if len(value) > 1 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				if unquoted, err := strconv.Unquote(value); err == nil {
					value = unquoted
				}
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return nil
}
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return fallback
}

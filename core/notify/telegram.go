package notify

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TelegramBot sends messages via the Telegram Bot API.
// All sends are non-blocking: messages are pushed to a buffered channel
// and processed by a background worker goroutine.
type TelegramBot struct {
	queue  chan telegramMsg
	client *http.Client
}

type telegramMsg struct {
	token  string
	chatID string
	text   string
}

const (
	queueSize  = 100
	maxRetries = 3
	retryDelay = 1 * time.Second
)

// NewTelegramBot starts the background worker.
func NewTelegramBot() *TelegramBot {
	bot := &TelegramBot{
		queue: make(chan telegramMsg, queueSize),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	go bot.worker()
	return bot
}

// Send pushes a message to the queue. Never blocks the caller.
func (b *TelegramBot) Send(token, chatID, text string) {
	select {
	case b.queue <- telegramMsg{token: token, chatID: chatID, text: text}:
		slog.Info("telegram queued", "chat_id", chatID)
	default:
		slog.Warn("telegram queue full, message dropped", "chat_id", chatID)
	}
}

// SendAll sends a message to multiple comma-separated chat IDs.
func (b *TelegramBot) SendAll(token, chatIDs, text string) {
	for _, id := range strings.Split(chatIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			b.Send(token, id, text)
		}
	}
}

func (b *TelegramBot) worker() {
	for msg := range b.queue {
		b.sendWithRetry(msg)
	}
}

func (b *TelegramBot) sendWithRetry(msg telegramMsg) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := b.doSend(msg)
		if err == nil {
			slog.Info("telegram sent", "chat_id", msg.chatID, "attempt", attempt)
			return
		}
		lastErr = err
		slog.Warn("telegram send failed", "chat_id", msg.chatID, "attempt", attempt, "error", err)
		if attempt < maxRetries {
			time.Sleep(retryDelay * time.Duration(attempt))
		}
	}
	slog.Error("telegram send failed after retries", "chat_id", msg.chatID, "error", lastErr)
}

func (b *TelegramBot) doSend(msg telegramMsg) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", msg.token)
	data := url.Values{
		"chat_id":    {msg.chatID},
		"text":       {msg.text},
		"parse_mode": {"HTML"},
	}

	resp, err := b.client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// EscapeHTML escapes special chars for Telegram HTML mode.
func EscapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

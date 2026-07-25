package whatsappweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/abettucci/group-split-bot/internal/telegram"
)

// Client habla con el sidecar Node (whatsapp-web.js) por HTTP.
// Implementa bot.Messenger.
type Client struct {
	httpClient   *http.Client
	sidecarURL   string
	sharedSecret string
}

// NewClient crea un cliente que postea outbound al sidecar.
// Si las env vars no están seteadas, el cliente igual se construye pero los Send fallarán.
func NewClient() *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		sidecarURL:   strings.TrimRight(os.Getenv("WAWEB_SIDECAR_URL"), "/"),
		sharedSecret: os.Getenv("WAWEB_SHARED_SECRET"),
	}
}

type sendRequest struct {
	ChatID   int64  `json:"chat_id"`
	ChatType string `json:"chat_type"`
	Text     string `json:"text"`
}

// SendMessage envía un mensaje individual (privado). Para grupos usar SendMessageWithChatType.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.sendInternal(ctx, chatID, "private", text)
}

// SendMessageWithOptions ignora ReplyMarkup (WhatsApp Web no tiene inline keyboards).
func (c *Client) SendMessageWithOptions(ctx context.Context, req *telegram.SendMessageRequest) error {
	return c.SendMessage(ctx, req.ChatID, req.Text)
}

// EditMessageText: WhatsApp no permite editar mensajes a través de la API web pública.
// Reenviamos como mensaje nuevo (mismo trato que el cliente Cloud).
func (c *Client) EditMessageText(ctx context.Context, chatID int64, _ int64, text string) error {
	return c.SendMessage(ctx, chatID, text)
}

// AnswerCallbackQuery: no aplica (no hay callbacks en WhatsApp Web).
func (c *Client) AnswerCallbackQuery(_ context.Context, _ string, _ string) error {
	return nil
}

func (c *Client) sendInternal(ctx context.Context, chatID int64, chatType, text string) error {
	if c.sidecarURL == "" {
		return fmt.Errorf("waweb: WAWEB_SIDECAR_URL not configured")
	}
	if c.sharedSecret == "" {
		return fmt.Errorf("waweb: WAWEB_SHARED_SECRET not configured")
	}

	body, err := json.Marshal(sendRequest{ChatID: chatID, ChatType: chatType, Text: text})
	if err != nil {
		return fmt.Errorf("waweb: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.sidecarURL+"/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("waweb: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.sharedSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("waweb: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("waweb: sidecar returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// InboundPayload representa el body que envía el sidecar a /wa-web/inbound.
type InboundPayload struct {
	ChatID    int64  `json:"chat_id"`
	RawJID    string `json:"raw_jid"`
	ChatType  string `json:"chat_type"` // "private" | "group"
	FromName  string `json:"from_name"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
	Timestamp int64  `json:"timestamp"`
}

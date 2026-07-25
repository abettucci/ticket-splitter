package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/abettucci/group-split-bot/internal/telegram"
)

const apiBase = "https://graph.facebook.com/v25.0"

// Client cliente para WhatsApp Cloud API
type Client struct {
	httpClient    *http.Client
	token         string
	phoneNumberID string
}

// NewClient crea un nuevo cliente de WhatsApp
func NewClient() *Client {
	return &Client{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		token:         os.Getenv("WHATSAPP_TOKEN"),
		phoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"),
	}
}

// SendMessage envía un mensaje de texto a un número de WhatsApp
func (c *Client) SendMessage(ctx context.Context, to int64, text string) error {
	toStr := fmt.Sprintf("%d", to)
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                toStr,
		"type":              "text",
		"text": map[string]interface{}{
			"preview_url": false,
			"body":        text,
		},
	}
	return c.post(ctx, "messages", payload)
}

// SendMessageWithOptions envía un mensaje — en WhatsApp ignoramos ReplyMarkup (no hay inline keyboards)
func (c *Client) SendMessageWithOptions(ctx context.Context, req *telegram.SendMessageRequest) error {
	return c.SendMessage(ctx, req.ChatID, req.Text)
}

// EditMessageText en WhatsApp no existe edición — enviamos un nuevo mensaje
func (c *Client) EditMessageText(ctx context.Context, to int64, _ int64, text string) error {
	return c.SendMessage(ctx, to, text)
}

// AnswerCallbackQuery no aplica en WhatsApp — no-op
func (c *Client) AnswerCallbackQuery(_ context.Context, _ string, _ string) error {
	return nil
}

func (c *Client) post(ctx context.Context, endpoint string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("whatsapp: marshal error: %w", err)
	}

	url := fmt.Sprintf("%s/%s/%s", apiBase, c.phoneNumberID, endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("whatsapp: request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("whatsapp: API error status %d", resp.StatusCode)
	}
	return nil
}

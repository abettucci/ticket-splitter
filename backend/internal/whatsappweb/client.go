package whatsappweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abettucci/group-split-bot/internal/telegram"
)

// Client habla con el sidecar Node (whatsapp-web.js) por HTTP.
// Implementa bot.Messenger.
type Client struct {
	httpClient   *http.Client
	sidecarURL   string
	sharedSecret string
	chatTypes    sync.Map // map[int64]string; response route learned from inbound WhatsApp messages
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
	// Keep chat IDs as JSON strings: group JIDs can exceed JavaScript's safe
	// integer range, and the Node sidecar must preserve every digit.
	ChatID      string                  `json:"chat_id"`
	ChatType    string                  `json:"chat_type"`
	Text        string                  `json:"text"`
	Interactive *interactiveListRequest `json:"interactive,omitempty"`
}

type interactiveListRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type interactiveListRequest struct {
	Type       string               `json:"type"`
	Title      string               `json:"title,omitempty"`
	ButtonText string               `json:"button_text"`
	Footer     string               `json:"footer,omitempty"`
	Rows       []interactiveListRow `json:"rows"`
}

// RememberChatType stores the route for a chat that has just sent an inbound message.
// Group replies must go to @g.us, while debt reminders always use a private route.
func (c *Client) RememberChatType(chatID int64, chatType string) {
	if chatType == "group" || chatType == "private" {
		c.chatTypes.Store(chatID, chatType)
	}
}

// SendMessage replies to the inbound chat type when known, otherwise defaults to private.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	chatType := "private"
	if value, ok := c.chatTypes.Load(chatID); ok {
		chatType, _ = value.(string)
	}
	return c.sendInternal(ctx, chatID, chatType, text)
}

// SendMessageWithOptions turns Telegram inline keyboards into a WhatsApp list
// when possible, while preserving a numbered-text fallback for every client.
func (c *Client) SendMessageWithOptions(ctx context.Context, req *telegram.SendMessageRequest) error {
	keyboard, ok := req.ReplyMarkup.(telegram.InlineKeyboardMarkup)
	if !ok {
		return c.SendMessage(ctx, req.ChatID, req.Text)
	}

	rows := make([]interactiveListRow, 0, 10)
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == "" || len(rows) == 10 {
				continue
			}
			rows = append(rows, interactiveListRow{ID: button.CallbackData, Title: button.Text})
		}
	}
	if len(rows) == 0 {
		return c.SendMessage(ctx, req.ChatID, req.Text)
	}

	fallback := formatInteractiveFallback(req.Text, rows)
	interactive := &interactiveListRequest{
		Type:       "list",
		Title:      "Splitter",
		ButtonText: "Ver opciones",
		Footer:     "También podés responder con el número o escribir lo que necesitás.",
		Rows:       rows,
	}
	return c.sendInternalWithInteractive(ctx, req.ChatID, c.chatTypeFor(req.ChatID), fallback, interactive)
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
	return c.sendInternalWithInteractive(ctx, chatID, chatType, text, nil)
}

func (c *Client) chatTypeFor(chatID int64) string {
	chatType := "private"
	if value, ok := c.chatTypes.Load(chatID); ok {
		chatType, _ = value.(string)
	}
	return chatType
}

func (c *Client) sendInternalWithInteractive(ctx context.Context, chatID int64, chatType, text string, interactive *interactiveListRequest) error {
	if c.sidecarURL == "" {
		return fmt.Errorf("waweb: WAWEB_SIDECAR_URL not configured")
	}
	if c.sharedSecret == "" {
		return fmt.Errorf("waweb: WAWEB_SHARED_SECRET not configured")
	}

	body, err := json.Marshal(sendRequest{ChatID: strconv.FormatInt(chatID, 10), ChatType: chatType, Text: text, Interactive: interactive})
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

func formatInteractiveFallback(text string, rows []interactiveListRow) string {
	var builder strings.Builder
	builder.WriteString(text)
	builder.WriteString("\n\nRespondé con:\n")
	for index, row := range rows {
		fmt.Fprintf(&builder, "%d. %s\n", index+1, row.Title)
	}
	return strings.TrimSpace(builder.String())
}

// InboundPayload representa el body que envía el sidecar a /wa-web/inbound.
type InboundPayload struct {
	ChatID        NumericID `json:"chat_id"`
	RawJID        string    `json:"raw_jid"`
	ChatType      string    `json:"chat_type"` // "private" | "group"
	ChatName      string    `json:"chat_name"`
	SenderID      NumericID `json:"sender_id"`
	SenderJID     string    `json:"sender_jid"`
	InteractiveID string    `json:"interactive_id"`
	IsMentioned   bool      `json:"is_mentioned"`
	FromName      string    `json:"from_name"`
	Text          string    `json:"text"`
	MessageID     string    `json:"message_id"`
	Timestamp     int64     `json:"timestamp"`
}

// NumericID accepts both the older numeric payload and the string payload
// used now. Strings are mandatory for large WhatsApp group IDs because JSON
// numbers are parsed as JavaScript Number values in the sidecar.
type NumericID string

func (id *NumericID) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if len(raw) > 1 && raw[0] == '"' {
		var decoded string
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		*id = NumericID(decoded)
		return nil
	}
	*id = NumericID(raw)
	return nil
}

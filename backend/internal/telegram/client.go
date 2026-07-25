package telegram

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// TelegramAPIBase URL base de la API de Telegram
	TelegramAPIBase = "https://api.telegram.org/bot"

	// RequestTimeout timeout para requests a Telegram
	RequestTimeout = 10 * time.Second

	// MaxRetries número máximo de reintentos
	MaxRetries = 3
)

// Client cliente para interactuar con Telegram API
type Client struct {
	httpClient *http.Client
	botToken   string
}

// Update representa un update de Telegram
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// Message representa un mensaje de Telegram
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      *Chat  `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text,omitempty"`
}

// User representa un usuario de Telegram
type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

// Chat representa un chat de Telegram
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"` // "private", "group", "supergroup", "channel"
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// CallbackQuery representa un callback de inline keyboard
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// SendMessageRequest request para enviar mensaje
type SendMessageRequest struct {
	ChatID                int64       `json:"chat_id"`
	Text                  string      `json:"text"`
	ParseMode             string      `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool        `json:"disable_web_page_preview,omitempty"`
	ReplyMarkup           interface{} `json:"reply_markup,omitempty"`
}

// InlineKeyboardMarkup teclado inline
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton botón del teclado inline
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// APIResponse respuesta genérica de Telegram
type APIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

// NewClient crea un nuevo cliente de Telegram
func NewClient() *Client {
	// Configurar HTTP client con TLS seguro
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   RequestTimeout,
		},
		botToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
	}
}

// SendMessage envía un mensaje de texto
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.SendMessageWithOptions(ctx, &SendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	})
}

// EscapeHTML escapa caracteres especiales para HTML
func EscapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// FormatMoney formatea un monto sin decimales y con separador de miles (punto)
func FormatMoney(amount float64) string {
	// Redondear al entero más cercano
	intAmount := int64(amount + 0.5)

	// Convertir a string
	str := fmt.Sprintf("%d", intAmount)

	// Agregar separadores de miles
	n := len(str)
	if n <= 3 {
		return "$" + str
	}

	// Calcular cuántos grupos de 3 dígitos hay
	var result strings.Builder
	result.WriteString("$")

	// Posición del primer grupo (puede ser 1, 2 o 3 dígitos)
	firstGroupLen := n % 3
	if firstGroupLen == 0 {
		firstGroupLen = 3
	}

	result.WriteString(str[:firstGroupLen])

	for i := firstGroupLen; i < n; i += 3 {
		result.WriteString(".")
		result.WriteString(str[i : i+3])
	}

	return result.String()
}

// SendMessageWithOptions envía un mensaje con opciones adicionales
func (c *Client) SendMessageWithOptions(ctx context.Context, req *SendMessageRequest) error {
	// Validar longitud del mensaje (Telegram limita a 4096)
	if len(req.Text) > 4096 {
		req.Text = req.Text[:4093] + "..."
	}

	return c.makeRequest(ctx, "sendMessage", req)
}

// AnswerCallbackQuery responde a un callback query
func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error {
	return c.makeRequest(ctx, "answerCallbackQuery", map[string]interface{}{
		"callback_query_id": callbackID,
		"text":              text,
	})
}

// EditMessageText edita el texto de un mensaje existente
func (c *Client) EditMessageText(ctx context.Context, chatID, messageID int64, text string) error {
	// Validar longitud del mensaje (Telegram limita a 4096)
	if len(text) > 4096 {
		text = text[:4093] + "..."
	}

	return c.makeRequest(ctx, "editMessageText", map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "HTML",
	})
}

// makeRequest realiza una request a la API de Telegram con reintentos
func (c *Client) makeRequest(ctx context.Context, method string, payload interface{}) error {
	url := fmt.Sprintf("%s%s/%s", TelegramAPIBase, c.botToken, method)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "GroupSplitBot/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Max 1MB response
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		var apiResp APIResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		if !apiResp.OK {
			// Si es rate limit (429), esperar más
			if apiResp.ErrorCode == 429 {
				time.Sleep(time.Second * time.Duration(attempt+1))
				lastErr = fmt.Errorf("rate limited: %s", apiResp.Description)
				continue
			}
			return fmt.Errorf("telegram API error: %s (code: %d)", apiResp.Description, apiResp.ErrorCode)
		}

		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetWebhook configura el webhook del bot
func (c *Client) SetWebhook(ctx context.Context, webhookURL, secretToken string) error {
	payload := map[string]interface{}{
		"url":                  webhookURL,
		"secret_token":         secretToken,
		"allowed_updates":      []string{"message", "callback_query"},
		"drop_pending_updates": true,
		"max_connections":      100,
	}

	return c.makeRequest(ctx, "setWebhook", payload)
}

// GetMe obtiene información del bot
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	url := fmt.Sprintf("%s%s/getMe", TelegramAPIBase, c.botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result User `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API error")
	}

	return &result.Result, nil
}

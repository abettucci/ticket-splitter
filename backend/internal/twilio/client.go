package twilio

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/abettucci/group-split-bot/internal/telegram"
)

const apiBase = "https://api.twilio.com/2010-04-01"

// Client implementa la interfaz Messenger usando Twilio para WhatsApp
type Client struct {
	httpClient *http.Client
	accountSid string
	authToken  string
	fromNumber string // formato: "whatsapp:+14155238886"
}

// NewClient crea un cliente Twilio con credenciales desde variables de entorno
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		accountSid: os.Getenv("TWILIO_ACCOUNT_SID"),
		authToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		fromNumber: os.Getenv("TWILIO_WHATSAPP_FROM"), // ej: "whatsapp:+14155238886"
	}
}

// SendMessage envía un mensaje de texto via Twilio WhatsApp
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	to := fmt.Sprintf("whatsapp:+%d", chatID)
	return c.sendRaw(ctx, to, StripHTML(text))
}

// SendMessageWithOptions envía un mensaje con opciones — ignora ReplyMarkup (no aplica en WhatsApp)
func (c *Client) SendMessageWithOptions(ctx context.Context, req *telegram.SendMessageRequest) error {
	return c.SendMessage(ctx, req.ChatID, req.Text)
}

// EditMessageText en WhatsApp no existe edición — envía un nuevo mensaje
func (c *Client) EditMessageText(ctx context.Context, chatID, _ int64, text string) error {
	return c.SendMessage(ctx, chatID, text)
}

// AnswerCallbackQuery no aplica en WhatsApp
func (c *Client) AnswerCallbackQuery(_ context.Context, _ string, _ string) error {
	return nil
}

// ValidateSignature verifica la firma HMAC-SHA1 que Twilio envía en X-Twilio-Signature.
// webhookURL debe ser la URL completa del endpoint (ej: https://api.tu-dominio.com/twilio/inbound).
// formBody es el body raw de la request (application/x-www-form-urlencoded).
func ValidateSignature(authToken, webhookURL, formBody, signature string) bool {
	params, err := url.ParseQuery(formBody)
	if err != nil {
		return false
	}

	// Algoritmo Twilio: URL + params ordenados por clave (clave+valor sin separador)
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(webhookURL)
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params.Get(k))
	}

	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(sb.String()))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

func (c *Client) sendRaw(ctx context.Context, to, text string) error {
	endpoint := fmt.Sprintf("%s/Accounts/%s/Messages.json", apiBase, c.accountSid)

	data := url.Values{}
	data.Set("From", c.fromNumber)
	data.Set("To", to)
	data.Set("Body", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: request error: %w", err)
	}
	req.SetBasicAuth(c.accountSid, c.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio: API error status %d", resp.StatusCode)
	}
	return nil
}

// StripHTML convierte el HTML del bot a formato WhatsApp (*bold*, _italic_, `code`)
func StripHTML(text string) string {
	replacements := []struct{ old, new string }{
		{"<b>", "*"}, {"</b>", "*"},
		{"<i>", "_"}, {"</i>", "_"},
		{"<code>", "`"}, {"</code>", "`"},
		{"<pre>", "```"}, {"</pre>", "```"},
		{"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"},
	}
	for _, r := range replacements {
		text = strings.ReplaceAll(text, r.old, r.new)
	}
	// Eliminar cualquier tag HTML restante
	for strings.Contains(text, "<") {
		start := strings.Index(text, "<")
		end := strings.Index(text[start:], ">")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+end+1:]
	}
	return text
}

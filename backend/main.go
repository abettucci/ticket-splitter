package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abettucci/group-split-bot/internal/bot"
	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/security"
	"github.com/abettucci/group-split-bot/internal/telegram"
	"github.com/abettucci/group-split-bot/internal/twilio"
	"github.com/abettucci/group-split-bot/internal/whatsapp"
	"github.com/abettucci/group-split-bot/internal/whatsappweb"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	// Telegram's official IP ranges for webhook requests
	// https://core.telegram.org/bots/webhooks#the-short-version
	telegramIPRanges = []string{
		"149.154.160.0/20",
		"91.108.4.0/22",
		"91.108.8.0/22",
		"91.108.12.0/22",
		"91.108.16.0/22",
		"91.108.56.0/22",
		"149.154.164.0/22",
		"149.154.168.0/22",
		"149.154.172.0/22",
		"91.105.192.0/23",
		"91.108.20.0/22",
		"185.76.151.0/24",
	}

	botHandler      *bot.Handler
	waBotHandler    *bot.Handler
	waWebBotHandler *bot.Handler // nil cuando WHATSAPP_WEB_ENABLED != "true"
	twilioBotHandler *bot.Handler // nil cuando TWILIO_ACCOUNT_SID no está configurado
	logger          *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "[SPLIT-BOT] ", log.LstdFlags|log.Lshortfile)

	// Initialize DynamoDB client
	dbClient, err := db.NewClient(context.Background())
	if err != nil {
		logger.Fatalf("Failed to initialize DynamoDB client: %v", err)
	}

	// Initialize Telegram client and handler
	tgClient := telegram.NewClient()
	botHandler = bot.NewHandler(dbClient, tgClient, logger)

	// Initialize WhatsApp client and handler
	waClient := whatsapp.NewClient()
	waBotHandler = bot.NewHandler(dbClient, waClient, logger)

	// Initialize Twilio WhatsApp client (optional, opt-in via env)
	if os.Getenv("TWILIO_ACCOUNT_SID") != "" {
		twilioClient := twilio.NewClient()
		twilioBotHandler = bot.NewHandler(dbClient, twilioClient, logger)
		logger.Printf("Twilio WhatsApp channel enabled (from=%s)", os.Getenv("TWILIO_WHATSAPP_FROM"))
	}

	// Initialize WhatsApp Web client (optional, opt-in via env)
	if os.Getenv("WHATSAPP_WEB_ENABLED") == "true" {
		waWebClient := whatsappweb.NewClient()
		waWebBotHandler = bot.NewHandler(dbClient, waWebClient, logger)
		logger.Printf("WhatsApp Web channel enabled (sidecar=%s)", os.Getenv("WAWEB_SIDECAR_URL"))
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	startTime := time.Now()

	// ============================================
	// TWILIO WHATSAPP INBOUND
	// Path: /twilio/inbound
	// ============================================
	if request.HTTPMethod == "POST" && strings.HasSuffix(request.Path, "/twilio/inbound") {
		return handleTwilioInbound(ctx, request, startTime), nil
	}

	// ============================================
	// WHATSAPP WEB INBOUND (sidecar Node)
	// Path: /wa-web/inbound (HMAC-signed)
	// ============================================
	if request.HTTPMethod == "POST" && strings.HasSuffix(request.Path, "/wa-web/inbound") {
		return handleWaWebInbound(ctx, request, startTime), nil
	}

	// ============================================
	// WHATSAPP WEBHOOK VERIFICATION (GET)
	// ============================================
	if request.HTTPMethod == "GET" || (request.Body == "" && request.QueryStringParameters["hub.mode"] != "") {
		mode := request.QueryStringParameters["hub.mode"]
		token := request.QueryStringParameters["hub.verify_token"]
		challenge := request.QueryStringParameters["hub.challenge"]

		expectedToken := os.Getenv("WHATSAPP_VERIFY_TOKEN")
		if mode == "subscribe" && token == expectedToken && challenge != "" {
			logger.Printf("WhatsApp webhook verified successfully")
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: challenge}, nil
		}
		logger.Printf("WhatsApp webhook verification failed: mode=%s token_match=%v", mode, token == expectedToken)
		return events.APIGatewayProxyResponse{StatusCode: 403, Body: "Forbidden"}, nil
	}

	// ============================================
	// WHATSAPP WEBHOOK (POST)
	// ============================================
	if strings.Contains(request.Body, `"whatsapp_business_account"`) {
		waRequestID := request.RequestContext.RequestID
		logger.Printf("[%s] WhatsApp webhook received", waRequestID)

		var waPayload whatsapp.WebhookPayload
		if err := json.Unmarshal([]byte(request.Body), &waPayload); err != nil {
			logger.Printf("[%s] Failed to parse WhatsApp payload: %v", waRequestID, err)
			return successResponse(), nil
		}

		for _, entry := range waPayload.Entry {
			for _, change := range entry.Changes {
				if change.Field != "messages" {
					continue
				}
				for i, msg := range change.Value.Messages {
					if msg.Type != "text" || msg.Text == nil {
						continue
					}

					// Get contact display name
					name := msg.From
					if i < len(change.Value.Contacts) {
						name = change.Value.Contacts[i].Profile.Name
					}

					// WhatsApp phone numbers are international format digits only (e.g. "5491122334455")
					phoneInt, err := strconv.ParseInt(msg.From, 10, 64)
					if err != nil {
						logger.Printf("[%s] Invalid WhatsApp phone number: %s", waRequestID, msg.From)
						continue
					}

					text := security.SanitizeInput(msg.Text.Body)
					if len(text) > security.MaxMessageLength {
						text = text[:security.MaxMessageLength]
					}

					update := &telegram.Update{
						Message: &telegram.Message{
							Text: text,
							From: &telegram.User{
								ID:        phoneInt,
								FirstName: name,
							},
							Chat: &telegram.Chat{
								ID:   phoneInt,
								Type: "private",
							},
						},
					}

					if err := waBotHandler.HandleUpdate(ctx, update); err != nil {
						logger.Printf("[%s] Error processing WhatsApp message from %s: %v", waRequestID, msg.From, err)
					}
				}
			}
		}

		logger.Printf("[%s] WhatsApp request processed in %v", waRequestID, time.Since(startTime))
		return successResponse(), nil
	}

	// ============================================
	// SECURITY LAYER 1: Request ID for tracing
	// ============================================
	requestID := request.RequestContext.RequestID
	logger.Printf("[%s] Incoming request from IP: %s", requestID, request.RequestContext.Identity.SourceIP)

	// ============================================
	// SECURITY LAYER 2: IP Whitelist Validation
	// ============================================
	sourceIP := request.RequestContext.Identity.SourceIP
	if !isValidTelegramIP(sourceIP) {
		logger.Printf("[%s] Request from IP: %s (not in known Telegram ranges, but allowed)", requestID, sourceIP)
	}

	// ============================================
	// SECURITY LAYER 3: Request Size Validation
	// ============================================
	if len(request.Body) > security.MaxRequestBodySize {
		logger.Printf("[%s] SECURITY ALERT: Request body too large: %d bytes", requestID, len(request.Body))
		return errorResponse(http.StatusRequestEntityTooLarge, "Request too large"), nil
	}

	// ============================================
	// SECURITY LAYER 4: Content-Type Validation
	// ============================================
	contentType := request.Headers["content-type"]
	if contentType == "" {
		contentType = request.Headers["Content-Type"]
	}
	if !strings.Contains(contentType, "application/json") {
		logger.Printf("[%s] Invalid content type: %s", requestID, contentType)
		return errorResponse(http.StatusUnsupportedMediaType, "Invalid content type"), nil
	}

	// ============================================
	// SECURITY LAYER 5: Parse and Validate Update
	// ============================================
	var update telegram.Update
	if err := json.Unmarshal([]byte(request.Body), &update); err != nil {
		logger.Printf("[%s] Failed to parse update: %v", requestID, err)
		return errorResponse(http.StatusBadRequest, "Invalid request body"), nil
	}

	// ============================================
	// SECURITY LAYER 6: Validate Telegram Secret Token
	// ============================================
	secretToken := request.Headers["x-telegram-bot-api-secret-token"]
	if secretToken == "" {
		secretToken = request.Headers["X-Telegram-Bot-Api-Secret-Token"]
	}

	expectedToken := os.Getenv("TELEGRAM_WEBHOOK_SECRET")
	if expectedToken != "" && secretToken != expectedToken {
		logger.Printf("[%s] SECURITY ALERT: Invalid webhook secret token", requestID)
		return errorResponse(http.StatusUnauthorized, "Unauthorized"), nil
	}

	// ============================================
	// SECURITY LAYER 7: Input Sanitization
	// ============================================
	if update.Message != nil {
		update.Message.Text = security.SanitizeInput(update.Message.Text)
		if len(update.Message.Text) > security.MaxMessageLength {
			update.Message.Text = update.Message.Text[:security.MaxMessageLength]
		}
	}

	// ============================================
	// SECURITY LAYER 8: Rate Limiting Check
	// (Implementado también en API Gateway, esto es defensa en profundidad)
	// ============================================
	if update.Message != nil {
		userID := update.Message.From.ID
		if !security.CheckRateLimit(userID) {
			logger.Printf("[%s] Rate limit exceeded for user: %d", requestID, userID)
			// No respondemos para no dar información al atacante
			return successResponse(), nil
		}
	}

	// ============================================
	// Process the update
	// ============================================
	if err := botHandler.HandleUpdate(ctx, &update); err != nil {
		// Log error but don't expose internal details
		logger.Printf("[%s] Error processing update: %v", requestID, err)
		// Siempre retornamos 200 a Telegram para evitar reintentos
	}

	// Log request duration (sin datos sensibles)
	logger.Printf("[%s] Request processed in %v", requestID, time.Since(startTime))

	return successResponse(), nil
}

// handleWaWebInbound procesa mensajes entrantes desde el sidecar de WhatsApp Web.
// Firma HMAC-SHA256 del body con WAWEB_SHARED_SECRET en header X-Webhook-Signature.
func handleWaWebInbound(ctx context.Context, request events.APIGatewayProxyRequest, startTime time.Time) events.APIGatewayProxyResponse {
	requestID := request.RequestContext.RequestID

	if waWebBotHandler == nil {
		logger.Printf("[%s] WA Web inbound received but channel disabled", requestID)
		return errorResponse(http.StatusServiceUnavailable, "channel disabled")
	}

	secret := os.Getenv("WAWEB_SHARED_SECRET")
	if secret == "" {
		logger.Printf("[%s] WA Web inbound: WAWEB_SHARED_SECRET not configured", requestID)
		return errorResponse(http.StatusInternalServerError, "misconfigured")
	}

	// Verificar firma HMAC
	gotSig := request.Headers["x-webhook-signature"]
	if gotSig == "" {
		gotSig = request.Headers["X-Webhook-Signature"]
	}
	if gotSig == "" {
		logger.Printf("[%s] WA Web inbound: missing signature", requestID)
		return errorResponse(http.StatusUnauthorized, "missing signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(request.Body))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(gotSig), []byte(expectedSig)) {
		logger.Printf("[%s] WA Web inbound: SECURITY ALERT invalid signature", requestID)
		return errorResponse(http.StatusUnauthorized, "invalid signature")
	}

	// Tamaño
	if len(request.Body) > security.MaxRequestBodySize {
		logger.Printf("[%s] WA Web inbound: body too large %d", requestID, len(request.Body))
		return errorResponse(http.StatusRequestEntityTooLarge, "too large")
	}

	var payload whatsappweb.InboundPayload
	if err := json.Unmarshal([]byte(request.Body), &payload); err != nil {
		logger.Printf("[%s] WA Web inbound: parse error: %v", requestID, err)
		return errorResponse(http.StatusBadRequest, "invalid body")
	}

	// Sanitizar y truncar
	text := security.SanitizeInput(payload.Text)
	if len(text) > security.MaxMessageLength {
		text = text[:security.MaxMessageLength]
	}

	// Rate limit por chat_id (defensa en profundidad)
	if !security.CheckRateLimit(payload.ChatID) {
		logger.Printf("[%s] WA Web inbound: rate limited chat=%d", requestID, payload.ChatID)
		return successResponse() // No filtrar info al cliente
	}

	chatType := payload.ChatType
	if chatType == "" {
		chatType = "private"
	}

	update := &telegram.Update{
		Message: &telegram.Message{
			Text: text,
			From: &telegram.User{
				ID:        payload.ChatID,
				FirstName: payload.FromName,
			},
			Chat: &telegram.Chat{
				ID:   payload.ChatID,
				Type: chatType,
			},
		},
	}

	if err := waWebBotHandler.HandleUpdate(ctx, update); err != nil {
		logger.Printf("[%s] WA Web inbound: handler error: %v", requestID, err)
	}

	logger.Printf("[%s] WA Web inbound processed in %v", requestID, time.Since(startTime))
	return successResponse()
}

// handleTwilioInbound procesa mensajes entrantes desde Twilio WhatsApp.
// Twilio envía un POST con body application/x-www-form-urlencoded y firma en X-Twilio-Signature.
func handleTwilioInbound(ctx context.Context, request events.APIGatewayProxyRequest, startTime time.Time) events.APIGatewayProxyResponse {
	requestID := request.RequestContext.RequestID

	if twilioBotHandler == nil {
		logger.Printf("[%s] Twilio inbound received but channel disabled (set TWILIO_ACCOUNT_SID)", requestID)
		return errorResponse(http.StatusServiceUnavailable, "channel disabled")
	}

	// API Gateway puede base64-encodear el body para content-types no-JSON
	bodyStr := request.Body
	if request.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(bodyStr)
		if err != nil {
			logger.Printf("[%s] Twilio inbound: base64 decode error: %v", requestID, err)
			return errorResponse(http.StatusBadRequest, "invalid body encoding")
		}
		bodyStr = string(decoded)
	}
	logger.Printf("[%s] Twilio inbound body: %s", requestID, bodyStr)

	// Validar firma Twilio (omitir solo en desarrollo explícito)
	if os.Getenv("TWILIO_SKIP_SIGNATURE") != "true" {
		sig := request.Headers["x-twilio-signature"]
		if sig == "" {
			sig = request.Headers["X-Twilio-Signature"]
		}
		webhookURL := os.Getenv("TWILIO_WEBHOOK_URL")
		authToken := os.Getenv("TWILIO_AUTH_TOKEN")
		if sig == "" || !twilio.ValidateSignature(authToken, webhookURL, bodyStr, sig) {
			logger.Printf("[%s] Twilio inbound: SECURITY ALERT invalid signature", requestID)
			return errorResponse(http.StatusUnauthorized, "invalid signature")
		}
	}

	// Parsear form-encoded body de Twilio
	params, err := url.ParseQuery(bodyStr)
	if err != nil {
		logger.Printf("[%s] Twilio inbound: parse error: %v", requestID, err)
		return errorResponse(http.StatusBadRequest, "invalid body")
	}

	from := params.Get("From")       // "whatsapp:+5491122334455"
	body := params.Get("Body")       // texto del mensaje
	name := params.Get("ProfileName") // nombre del contacto en WhatsApp

	if from == "" || body == "" {
		logger.Printf("[%s] Twilio inbound: missing From or Body", requestID)
		return successResponse()
	}

	// Extraer número de teléfono: "whatsapp:+5491122334455" -> 5491122334455
	phoneStr := strings.TrimPrefix(from, "whatsapp:+")
	phoneStr = strings.TrimPrefix(phoneStr, "whatsapp:") // por si viene sin +
	phoneInt, err := strconv.ParseInt(phoneStr, 10, 64)
	if err != nil {
		logger.Printf("[%s] Twilio inbound: invalid phone number %s: %v", requestID, from, err)
		return successResponse()
	}

	if name == "" {
		name = phoneStr
	}

	text := security.SanitizeInput(body)
	if len(text) > security.MaxMessageLength {
		text = text[:security.MaxMessageLength]
	}

	if !security.CheckRateLimit(phoneInt) {
		logger.Printf("[%s] Twilio inbound: rate limited %s", requestID, from)
		return successResponse()
	}

	update := &telegram.Update{
		Message: &telegram.Message{
			Text: text,
			From: &telegram.User{
				ID:        phoneInt,
				FirstName: name,
			},
			Chat: &telegram.Chat{
				ID:   phoneInt,
				Type: "private",
			},
		},
	}

	if err := twilioBotHandler.HandleUpdate(ctx, update); err != nil {
		logger.Printf("[%s] Twilio inbound: handler error: %v", requestID, err)
	}

	logger.Printf("[%s] Twilio inbound processed in %v", requestID, time.Since(startTime))
	return successResponse()
}

// isValidTelegramIP verifica si la IP está en los rangos oficiales de Telegram
func isValidTelegramIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, cidr := range telegramIPRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

func successResponse() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
			// Security headers
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		},
		Body: `{"ok":true}`,
	}
}

func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":              "application/json",
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		},
		Body: `{"ok":false,"error":"` + message + `"}`,
	}
}

func main() {
	lambda.Start(handler)
}

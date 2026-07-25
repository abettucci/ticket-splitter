package security

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// MaxRequestBodySize límite de tamaño del body (64KB)
	MaxRequestBodySize = 64 * 1024

	// MaxMessageLength longitud máxima de mensaje permitida
	MaxMessageLength = 4096

	// MaxCommandLength longitud máxima de un comando
	MaxCommandLength = 256

	// MaxDescriptionLength longitud máxima de descripción de gasto
	MaxDescriptionLength = 200

	// MaxAmountValue monto máximo permitido (previene overflow)
	MaxAmountValue = 999999999.99

	// MinAmountValue monto mínimo permitido
	MinAmountValue = 0.01

	// RateLimitWindow ventana de tiempo para rate limiting
	RateLimitWindow = time.Minute

	// RateLimitMaxRequests máximo de requests por ventana
	RateLimitMaxRequests = 30
)

var (
	// Patrones peligrosos que deben ser bloqueados
	dangerousPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script[^>]*>`),                   // XSS script tags
		regexp.MustCompile(`(?i)javascript:`),                     // JavaScript protocol
		regexp.MustCompile(`(?i)on\w+\s*=`),                       // Event handlers
		regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|create|alter)\s+`), // SQL injection
		regexp.MustCompile(`(?i)(\$\{|\$\(|` + "`" + `)`),         // Template injection
		regexp.MustCompile(`(?i)(\.\.\/|\.\.\\)`),                 // Path traversal
		regexp.MustCompile(`(?i)(<iframe|<object|<embed)`),        // Dangerous HTML
	}

	// Caracteres permitidos en descripciones (whitelist approach)
	allowedDescriptionChars = regexp.MustCompile(`^[\p{L}\p{N}\s\-_.,!?()áéíóúÁÉÍÓÚñÑüÜ]+$`)

	// Rate limiter storage (en producción usar Redis/DynamoDB)
	rateLimiter = &RateLimiter{
		requests: make(map[int64]*userRequests),
	}
)

// RateLimiter implementa rate limiting por usuario
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[int64]*userRequests
}

type userRequests struct {
	count    int
	windowStart time.Time
}

// CheckRateLimit verifica si el usuario excedió el rate limit
func CheckRateLimit(userID int64) bool {
	rateLimiter.mu.Lock()
	defer rateLimiter.mu.Unlock()

	now := time.Now()
	
	req, exists := rateLimiter.requests[userID]
	if !exists {
		rateLimiter.requests[userID] = &userRequests{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Si la ventana expiró, resetear
	if now.Sub(req.windowStart) > RateLimitWindow {
		req.count = 1
		req.windowStart = now
		return true
	}

	// Incrementar contador
	req.count++
	
	return req.count <= RateLimitMaxRequests
}

// SanitizeInput limpia el input de caracteres/patrones peligrosos
func SanitizeInput(input string) string {
	if input == "" {
		return ""
	}

	// Remover caracteres de control excepto newlines y tabs
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, input)

	// Trim espacios
	sanitized = strings.TrimSpace(sanitized)

	// Verificar patrones peligrosos
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(sanitized) {
			// Remover el patrón peligroso
			sanitized = pattern.ReplaceAllString(sanitized, "")
		}
	}

	return sanitized
}

// ValidateDescription valida una descripción de gasto
func ValidateDescription(desc string) (string, bool) {
	desc = SanitizeInput(desc)
	
	if len(desc) == 0 {
		return "", false
	}
	
	if len(desc) > MaxDescriptionLength {
		desc = desc[:MaxDescriptionLength]
	}

	// Verificar que solo contenga caracteres permitidos
	if !allowedDescriptionChars.MatchString(desc) {
		// Filtrar caracteres no permitidos
		var filtered strings.Builder
		for _, r := range desc {
			if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) ||
				r == '-' || r == '_' || r == '.' || r == ',' || r == '!' || 
				r == '?' || r == '(' || r == ')' {
				filtered.WriteRune(r)
			}
		}
		desc = filtered.String()
	}

	return desc, len(desc) > 0
}

// ValidateAmount valida un monto numérico
func ValidateAmount(amount float64) bool {
	return amount >= MinAmountValue && amount <= MaxAmountValue
}

// ValidateCommand valida que un comando sea válido
func ValidateCommand(cmd string) bool {
	if len(cmd) > MaxCommandLength {
		return false
	}

	// Comandos válidos
	validCommands := []string{
		"/start", "/help", "/nuevo_gasto", "/ver_gastos",
		"/dividir", "/mis_deudas", "/pagar", "/agregar_miembro",
		"/eliminar_miembro", "/balance", "/historial",
	}

	cmdLower := strings.ToLower(cmd)
	for _, valid := range validCommands {
		if strings.HasPrefix(cmdLower, valid) {
			return true
		}
	}

	return false
}

// MaskSensitiveData enmascara datos sensibles para logs
func MaskSensitiveData(data string) string {
	if len(data) <= 4 {
		return "****"
	}
	return data[:2] + strings.Repeat("*", len(data)-4) + data[len(data)-2:]
}

// ValidatePhoneNumber valida formato de número de teléfono
func ValidatePhoneNumber(phone string) bool {
	// Remover espacios y caracteres no numéricos excepto +
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || r == '+' {
			return r
		}
		return -1
	}, phone)

	// Debe tener entre 8 y 15 dígitos
	digits := strings.TrimPrefix(cleaned, "+")
	return len(digits) >= 8 && len(digits) <= 15
}

// GenerateSecureToken genera un token seguro para webhooks
// Nota: En producción usar crypto/rand
func GenerateSecureToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// En producción, usar crypto/rand en lugar de esto
	// Este es solo un placeholder
	return strings.Repeat("x", length)
}


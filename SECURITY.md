# 🔐 Documentación de Seguridad - SplitBot

Este documento detalla todas las medidas de seguridad implementadas en SplitBot.

## Índice

1. [Arquitectura de Seguridad](#arquitectura-de-seguridad)
2. [Protección contra DDoS](#protección-contra-ddos)
3. [Autenticación y Autorización](#autenticación-y-autorización)
4. [Validación de Input](#validación-de-input)
5. [Encriptación](#encriptación)
6. [Gestión de Secretos](#gestión-de-secretos)
7. [Logging y Auditoría](#logging-y-auditoría)
8. [Políticas IAM](#políticas-iam)
9. [Mejores Prácticas](#mejores-prácticas)
10. [Checklist de Seguridad](#checklist-de-seguridad)

---

## Arquitectura de Seguridad

```
Internet
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│                    AWS WAF                                   │
│  - Rate Limiting (1000 req/5min/IP)                         │
│  - SQL Injection Protection (AWSManagedRulesSQLiRuleSet)    │
│  - Common Attack Protection (AWSManagedRulesCommonRuleSet)  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│                 API Gateway (HTTP API)                       │
│  - Throttling: 50 req/seg, burst 100                        │
│  - TLS 1.2+ obligatorio                                     │
│  - Request validation                                        │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│                   AWS Lambda (Go)                            │
│  - Validación de IP origen                                   │
│  - Validación de secret token                                │
│  - Sanitización de input                                     │
│  - Rate limiting por usuario                                 │
│  - Request size validation                                   │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│                    DynamoDB                                  │
│  - Encriptación at rest (KMS)                               │
│  - Point-in-time recovery                                    │
│  - IAM-based access control                                  │
└─────────────────────────────────────────────────────────────┘
```

---

## Protección contra DDoS

### Capa 1: AWS Shield (Automático)
- Protección contra ataques DDoS de capas 3 y 4
- Incluido sin costo adicional con API Gateway

### Capa 2: AWS WAF
```hcl
# Rate limiting por IP
rate_based_statement {
  limit              = 1000  # requests por 5 minutos
  aggregate_key_type = "IP"
}
```

### Capa 3: API Gateway Throttling
```hcl
default_route_settings {
  throttling_burst_limit = 100  # máximo simultáneo
  throttling_rate_limit  = 50   # por segundo
}
```

### Capa 4: Rate Limiting en Aplicación
```go
// Por usuario: 30 requests por minuto
const RateLimitMaxRequests = 30
const RateLimitWindow = time.Minute
```

---

## Autenticación y Autorización

### Validación de Webhook Secret
Telegram envía un header `X-Telegram-Bot-Api-Secret-Token` que validamos:

```go
secretToken := request.Headers["x-telegram-bot-api-secret-token"]
expectedToken := os.Getenv("TELEGRAM_WEBHOOK_SECRET")

if secretToken != expectedToken {
    return errorResponse(http.StatusUnauthorized, "Unauthorized"), nil
}
```

### Validación de IP de Origen
Solo aceptamos requests de los rangos de IP oficiales de Telegram:

```go
var telegramIPRanges = []string{
    "149.154.160.0/20",
    "91.108.4.0/22",
}
```

---

## Validación de Input

### Límites de Tamaño
```go
const MaxRequestBodySize = 64 * 1024     // 64KB
const MaxMessageLength = 4096            // Telegram limit
const MaxDescriptionLength = 200
const MaxAmountValue = 999999999.99
```

### Sanitización
Removemos patrones peligrosos:

```go
var dangerousPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)<script[^>]*>`),           // XSS
    regexp.MustCompile(`(?i)javascript:`),             // JS protocol
    regexp.MustCompile(`(?i)on\w+\s*=`),               // Event handlers
    regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop)`), // SQLi
    regexp.MustCompile(`(?i)(\$\{|\$\()`),             // Template injection
    regexp.MustCompile(`(?i)(\.\.\/|\.\.\\)`),         // Path traversal
}
```

### Whitelist de Caracteres
Solo permitimos caracteres seguros en descripciones:

```go
var allowedDescriptionChars = regexp.MustCompile(
    `^[\p{L}\p{N}\s\-_.,!?()áéíóúÁÉÍÓÚñÑüÜ]+$`
)
```

---

## Encriptación

### En Tránsito
- **TLS 1.2+** obligatorio en API Gateway
- **TLS 1.2+** para conexiones a Telegram API
- **TLS 1.2+** para conexiones a DynamoDB

### En Reposo
- **DynamoDB**: Encriptación con KMS administrado por AWS
- **CloudWatch Logs**: Encriptación por defecto
- **Secrets Manager**: Encriptación con KMS

```hcl
resource "aws_dynamodb_table" "main" {
  server_side_encryption {
    enabled = true
  }
}
```

---

## Gestión de Secretos

### AWS Secrets Manager
Almacenamos credenciales sensibles en Secrets Manager:

```hcl
resource "aws_secretsmanager_secret_version" "bot_secrets" {
  secret_string = jsonencode({
    TELEGRAM_BOT_TOKEN      = var.telegram_bot_token
    TELEGRAM_WEBHOOK_SECRET = random_password.webhook_secret.result
  })
}
```

### Variables de Entorno
- Nunca hardcodeamos secretos
- Los tokens se pasan via variables de entorno cifradas
- El webhook secret se genera automáticamente

---

## Logging y Auditoría

### CloudWatch Logs
```go
logger.Printf("[%s] Incoming request from IP: %s", requestID, sourceIP)
logger.Printf("[%s] SECURITY ALERT: Invalid webhook secret", requestID)
```

### Audit Trail en DynamoDB
Todos los comandos se registran:

```go
type BotCommand struct {
    ChatID    int64
    UserID    int64
    Command   string
    CreatedAt time.Time
}
```

### Alertas de Seguridad
```hcl
resource "aws_cloudwatch_metric_alarm" "waf_blocked_requests" {
  alarm_name  = "waf-blocked"
  threshold   = 100
  metric_name = "BlockedRequests"
}
```

---

## Políticas IAM

### Principio de Menor Privilegio

La función Lambda SOLO puede:

1. **CloudWatch Logs**: Escribir logs
```json
{
  "Action": ["logs:CreateLogStream", "logs:PutLogEvents"],
  "Resource": "arn:aws:logs:*:*:log-group:/aws/lambda/splitbot:*"
}
```

2. **DynamoDB**: CRUD en su tabla específica
```json
{
  "Action": ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:Query"],
  "Resource": ["arn:aws:dynamodb:*:*:table/splitbot-*"]
}
```

3. **Secrets Manager**: Leer sus secretos
```json
{
  "Action": ["secretsmanager:GetSecretValue"],
  "Resource": "arn:aws:secretsmanager:*:*:secret:splitbot-*"
}
```

---

## Mejores Prácticas

### Headers de Seguridad
Incluimos headers de seguridad en todas las respuestas:

```go
headers := map[string]string{
    "X-Content-Type-Options":    "nosniff",
    "X-Frame-Options":           "DENY",
    "X-XSS-Protection":          "1; mode=block",
    "Strict-Transport-Security": "max-age=31536000; includeSubDomains",
}
```

### Error Handling
- Nunca exponemos errores internos al usuario
- Logueamos errores detallados internamente
- Siempre retornamos 200 a Telegram (evita reintentos)

### Retry Logic
```go
const MaxRetries = 3
// Exponential backoff
time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
```

---

## Checklist de Seguridad

### Pre-Deployment
- [ ] Variables sensibles en Secrets Manager
- [ ] `terraform.tfvars` en `.gitignore`
- [ ] `govulncheck ./...` ejecutado
- [ ] Tests de seguridad pasados
- [ ] Revisión de código

### Post-Deployment
- [ ] Webhook configurado con secret
- [ ] WAF activo y monitoreado
- [ ] Alertas configuradas
- [ ] Logs funcionando
- [ ] Backup de DynamoDB activo

### Mantenimiento Regular
- [ ] Rotar token del bot cada 90 días
- [ ] Revisar logs de WAF semanalmente
- [ ] Actualizar dependencias mensualmente
- [ ] Ejecutar `make security` antes de cada deploy
- [ ] Revisar políticas IAM trimestralmente

---

## Reporte de Vulnerabilidades

Si encuentras una vulnerabilidad de seguridad, por favor:

1. **NO** la publiques públicamente
2. Envía un email a: security@tudominio.com
3. Incluye:
   - Descripción de la vulnerabilidad
   - Pasos para reproducirla
   - Impacto potencial

Responderemos dentro de 48 horas.

---

## Recursos Adicionales

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [AWS Security Best Practices](https://docs.aws.amazon.com/wellarchitected/latest/security-pillar/)
- [Telegram Bot Security](https://core.telegram.org/bots#6-botfather)
- [Go Security Checklist](https://github.com/Checkmarx/Go-SCP)


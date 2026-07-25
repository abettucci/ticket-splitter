# SplitBot Backend - AWS Lambda + Go

Bot de Telegram para dividir gastos grupales, construido con Go y desplegado en AWS Lambda.

## 🏗️ Arquitectura

```
Telegram → API Gateway → WAF → Lambda (Go) → DynamoDB
                ↓
         Secrets Manager
                ↓
          CloudWatch
```

## 🔐 Medidas de Seguridad Implementadas

### 1. Protección contra DDoS
- **AWS WAF**: Rate limiting (1000 req/5min por IP)
- **API Gateway Throttling**: 50 req/seg, burst de 100
- **Rate Limiting en código**: 30 req/min por usuario

### 2. Validación de Requests
- Verificación de IP de origen (rangos de Telegram)
- Validación de `X-Telegram-Bot-Api-Secret-Token`
- Límite de tamaño de request (64KB)
- Validación de Content-Type

### 3. Sanitización de Input
- Filtrado de caracteres de control
- Detección y bloqueo de patrones peligrosos:
  - SQL Injection
  - XSS (script tags, event handlers)
  - Template Injection
  - Path Traversal
- Límites de longitud en todos los campos

### 4. Encriptación
- **En tránsito**: TLS 1.2+ obligatorio
- **En reposo**: DynamoDB con KMS
- **Secrets**: AWS Secrets Manager

### 5. IAM - Principio de Menor Privilegio
- Lambda solo puede:
  - Escribir logs a CloudWatch
  - CRUD en su tabla DynamoDB específica
  - Leer secrets específicos

### 6. Logging y Auditoría
- Todos los comandos se registran
- Logs de API Gateway
- Métricas de WAF
- Alertas de CloudWatch

## 📦 Requisitos

- Go 1.21+
- AWS CLI configurado
- Terraform 1.0+
- Make

## 🚀 Deployment

### 1. Crear el bot en Telegram

```bash
# Habla con @BotFather en Telegram
# /newbot
# Sigue las instrucciones y guarda el token
```

### 2. Configurar variables

```bash
cd infrastructure
cp terraform.tfvars.example terraform.tfvars
# Edita terraform.tfvars con tu token
```

### 3. Build y Deploy

```bash
# Desde el directorio backend
cd backend

# Instalar dependencias
make deps

# Build para Lambda (ARM64 - más económico)
make build-arm

# Deploy
cd ../infrastructure
terraform init
terraform plan
terraform apply
```

### 4. Configurar Webhook

El output de Terraform incluye el comando curl necesario para configurar el webhook.

```bash
# Ver outputs
terraform output -json

# Configurar webhook (usar el comando del output)
curl -X POST "https://api.telegram.org/bot<TOKEN>/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "<WEBHOOK_URL>",
    "secret_token": "<SECRET>",
    "allowed_updates": ["message", "callback_query"]
  }'
```

## 🧪 Testing Local

```bash
# Ejecutar tests
make test

# Verificar vulnerabilidades
make security

# Linter
make lint
```

## 📊 Monitoreo

```bash
# Ver logs en tiempo real
aws logs tail /aws/lambda/splitbot-prod --follow

# Ver métricas de WAF
aws cloudwatch get-metric-statistics \
  --namespace AWS/WAFV2 \
  --metric-name BlockedRequests \
  --dimensions Name=WebACL,Value=splitbot-prod-waf \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum
```

## 💰 Costos Estimados

| Servicio | Free Tier | Después |
|----------|-----------|---------|
| Lambda | 1M req/mes | $0.20/1M req |
| API Gateway | 1M req/mes (12 meses) | $1/1M req |
| DynamoDB | 25GB + 25 WCU/RCU | Pay per request |
| WAF | - | $5/mes + $0.60/1M req |
| Secrets Manager | - | $0.40/secret/mes |
| CloudWatch Logs | 5GB | $0.50/GB |

**Estimación para bajo tráfico**: $5-10/mes

**Sin WAF (menos seguro pero más barato)**: ~$0-2/mes

## 🔧 Comandos del Bot

| Comando | Descripción |
|---------|-------------|
| `/start` | Bienvenida |
| `/help` | Ayuda |
| `/nuevo_gasto [desc] [monto]` | Crear gasto |
| `/ver_gastos` | Listar gastos |
| `/dividir [id]` | Dividir gasto |
| `/mis_deudas` | Ver deudas |
| `/pagar [id]` | Marcar pagado |
| `/miembros` | Ver miembros |
| `/balance` | Balance grupal |

## 📁 Estructura del Proyecto

```
backend/
├── main.go                 # Handler Lambda principal
├── go.mod
├── Makefile
├── internal/
│   ├── bot/
│   │   └── handler.go      # Lógica de comandos
│   ├── db/
│   │   └── client.go       # Operaciones DynamoDB
│   ├── security/
│   │   └── security.go     # Validación y sanitización
│   └── telegram/
│       └── client.go       # Cliente Telegram API
└── README.md

infrastructure/
├── main.tf                 # Terraform principal
├── terraform.tfvars.example
└── .gitignore
```

## 🛡️ Checklist de Seguridad

- [x] Validación de token de webhook
- [x] Rate limiting (API Gateway + código)
- [x] WAF con reglas de SQL injection
- [x] Sanitización de inputs
- [x] Encriptación en reposo (DynamoDB + KMS)
- [x] Encriptación en tránsito (TLS)
- [x] Secrets en Secrets Manager
- [x] IAM con menor privilegio
- [x] Logging de auditoría
- [x] Alertas de seguridad
- [x] Headers de seguridad en responses

## ⚠️ Notas de Seguridad

1. **Nunca** commitear `terraform.tfvars` o tokens
2. Rotar el token del bot periódicamente
3. Revisar logs de WAF regularmente
4. Mantener dependencias actualizadas: `make update`
5. Ejecutar `make security` antes de deploy


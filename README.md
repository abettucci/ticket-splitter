# 🤖 SplitBot - Bot de División de Gastos para Telegram

Bot de Telegram para dividir gastos grupales de forma simple, segura y **100% gratuita**.

![Telegram](https://img.shields.io/badge/Telegram-Bot-blue?logo=telegram)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![AWS](https://img.shields.io/badge/AWS-Lambda-FF9900?logo=amazonaws)
![GitHub Actions](https://img.shields.io/badge/CI%2FCD-GitHub_Actions-2088FF?logo=githubactions)

## ✨ Características

- 🆓 **100% Gratuito ** - Sin costos ocultos, sin planes premium
- ⚡ **Ultra rápido** - Backend en Go, respuestas en milisegundos
- 🔐 **Seguro** - Encriptación, rate limiting y validación de inputs
- 📱 **Nativo de Telegram** - Sin apps adicionales
- 🚀 **CI/CD Automatizado** - Deploy con GitHub Actions

## 🚀 Deploy Automático con GitHub Actions

### Paso 1: Fork/Clone el repositorio

```bash
git clone https://github.com/tu-usuario/group-split-bot.git
cd group-split-bot
```

### Paso 2: Crear el Bot en Telegram

1. Abre Telegram y busca **@BotFather**
2. Envía `/newbot`
3. Sigue las instrucciones y **guarda el token**

### Paso 3: Configurar Secrets en GitHub

Ve a tu repositorio → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**

| Secret Name | Descripción | Ejemplo |
|-------------|-------------|---------|
| `AWS_ACCESS_KEY_ID` | AWS Access Key | `AKIA...` |
| `AWS_SECRET_ACCESS_KEY` | AWS Secret Key | `wJalrXU...` |
| `AWS_REGION` | Región de AWS | `us-east-1` |
| `TELEGRAM_BOT_TOKEN` | Token de BotFather | `123456789:ABC...` |

**Opcional (para frontend en Netlify):**
| Secret Name | Descripción |
|-------------|-------------|
| `NETLIFY_AUTH_TOKEN` | Token de Netlify |
| `NETLIFY_SITE_ID` | ID del sitio en Netlify |
| `TELEGRAM_BOT_URL` | URL del bot (ej: `https://t.me/MiBot`) |

### Paso 4: Push y Deploy! 🚀

```bash
git add .
git commit -m "Initial deploy"
git push origin main
```

El workflow de GitHub Actions:
1. ✅ Compila el backend en Go
2. ✅ Ejecuta Terraform para crear la infraestructura
3. ✅ Configura el webhook de Telegram automáticamente
4. ✅ (Opcional) Despliega el frontend a Netlify

## 📁 Estructura del Proyecto

```
group-split-bot/
├── .github/
│   └── workflows/
│       ├── deploy.yaml           # CI/CD Backend + Infra
│       └── deploy-frontend.yaml  # CI/CD Frontend
├── backend/                      # Go Lambda
│   ├── main.go
│   ├── go.mod
│   └── internal/
│       ├── bot/                  # Lógica de comandos
│       ├── db/                   # DynamoDB
│       ├── security/             # Validación
│       └── telegram/             # Cliente API
├── infrastructure/               # Terraform
│   └── main.tf
├── src/                          # React Landing
└── scripts/                      # Helpers
```

## 🤖 Comandos del Bot

| Comando | Descripción |
|---------|-------------|
| `/start` | Mensaje de bienvenida |
| `/help` | Ver ayuda |
| `/nuevo_gasto [desc] [monto]` | Crear gasto |
| `/ver_gastos` | Ver gastos |
| `/dividir [id]` | Dividir gasto |
| `/mis_deudas` | Ver deudas |
| `/pagar [id]` | Marcar pagado |
| `/balance` | Balance grupal |

## 💰 Costos Estimados

| Recurso | Free Tier |
|---------|-----------|
| Lambda | 1M req/mes gratis |
| API Gateway | 1M req/mes gratis |
| DynamoDB | 25GB gratis siempre |
| **Total** | **$0/mes** |

## 🔧 Desarrollo Local

```bash
# Backend
cd backend
go mod download
go run .

# Frontend
npm install
npm run dev
```

## 🔍 Monitoreo

```bash
# Ver logs de Lambda
aws logs tail /aws/lambda/splitbot-prod --follow

# Ver estado del webhook
curl "https://api.telegram.org/bot<TOKEN>/getWebhookInfo" | jq
```

## 🛡️ Seguridad

Ver [SECURITY.md](./SECURITY.md) para documentación completa de seguridad.

## 📝 Licencia

MIT License

---

**¿Problemas?** Abre un [Issue](https://github.com/tu-usuario/group-split-bot/issues)

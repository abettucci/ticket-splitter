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

## 🗺️ Roadmap

La V1 web agregará una división de gastos sin registro, guardada localmente en el navegador. La integración para pagar transferencias mediante Mercado Pago está planificada para una segunda versión: [ver diseño de V2](./ROADMAP_V2_MERCADO_PAGO.md).

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

## 📲 Recordatorios por WhatsApp Web (prueba temporal)

El sidecar de WhatsApp Web puede entregar recordatorios individuales que un usuario haya creado desde ese mismo canal. Es una integración de prueba: no usar para difusión ni para una operación productiva.

1. Desplegá `whatsapp-web-sidecar` en un servicio persistente con volumen para `/data/wa-session` y vinculá la cuenta escaneando el QR.
2. En GitHub Actions configurá los secretos `WAWEB_SIDECAR_URL` y `WAWEB_SHARED_SECRET`, y la variable de repositorio `WHATSAPP_WEB_REMINDERS_ENABLED` con el valor `true`. El workflow muestra el estado efectivo antes y después de aplicar Terraform; verificá que `WHATSAPP_WEB_ENABLED` quede en `true` en `splitbot-prod`.
3. Después del deploy, obtené `terraform output -raw wa_web_backend_base_url` y usalo como `GO_BACKEND_URL` del sidecar. Conservá `INBOUND_PATH=/wa-web/inbound`.
4. Para vincular el teléfono, abrí `https://<dominio-sidecar>/qr`. El navegador pedirá autenticación HTTP Basic: usuario `splitbot` y como contraseña el mismo `SHARED_SECRET`. Escaneá el QR desde WhatsApp → Dispositivos vinculados → Vincular un dispositivo. El QR no se guarda ni se expone en los logs. El sidecar limita los ciclos de QR a uno por deploy (`QR_MAX_RETRIES=1`); si vence o falla, esperá el período de enfriamiento de WhatsApp y hacé un redeploy manual antes de pedir otro. No uses proxies ni copies la carpeta de sesión fuera del volumen persistente.
5. Los grupos están habilitados por defecto. Para no interrumpir el chat, mencioná a Splitter en el grupo y pedile una acción (por ejemplo, `@Splitter nuevo gasto`) o mencioná solo al bot para abrir el menú. WhatsApp Web no admite botones ni listas confiables desde este canal no oficial, por lo que el menú siempre ofrece alternativas numeradas y también entiende frases como “ver recordatorios”, “redividir” o “balance”. Un flujo ya iniciado puede continuarse sin volver a mencionarlo. Si la variable de Railway `ALLOW_GROUPS` existe con valor `false`, cambiala a `true` y hacé redeploy.
6. Para recibir recordatorios privados de deudas del grupo, cada integrante debe abrir el chat individual con el bot y enviar `/start`. Eso guarda su consentimiento y la dirección de WhatsApp en el volumen persistente; `/recordar_deudas` solo escribe por privado a quienes hayan hecho ese paso.
7. El usuario también puede crear un recordatorio propio por WhatsApp Web con `/recordar_pago`. El worker horario lo enviará por el sidecar y deja de hacerlo al cancelar el recordatorio. Los avisos usan 🐀 en vez de la campana.

Los recordatorios ya creados y los creados desde Telegram siguen entregándose por Telegram.

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

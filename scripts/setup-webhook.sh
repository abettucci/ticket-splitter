#!/bin/bash
# ============================================
# Script para configurar el webhook de Telegram
# ============================================

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🤖 SplitBot Webhook Setup${NC}"
echo "================================"

# Verificar si se pasaron los argumentos necesarios
if [ -z "$1" ] || [ -z "$2" ]; then
    echo -e "${YELLOW}Uso: $0 <BOT_TOKEN> <WEBHOOK_URL> [WEBHOOK_SECRET]${NC}"
    echo ""
    echo "Ejemplo:"
    echo "  $0 123456:ABC-DEF https://abc123.execute-api.us-east-1.amazonaws.com/webhook mysecret"
    echo ""
    echo "Si no provees WEBHOOK_SECRET, se generará uno automáticamente."
    exit 1
fi

BOT_TOKEN=$1
WEBHOOK_URL=$2
WEBHOOK_SECRET=${3:-$(openssl rand -hex 32)}

echo -e "\n${YELLOW}📋 Configuración:${NC}"
echo "  Bot Token: ${BOT_TOKEN:0:10}...${BOT_TOKEN: -5}"
echo "  Webhook URL: $WEBHOOK_URL"
echo "  Secret: ${WEBHOOK_SECRET:0:8}..."

# Primero, eliminar webhook existente
echo -e "\n${YELLOW}🗑️  Eliminando webhook existente...${NC}"
DELETE_RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/deleteWebhook")
echo "  Respuesta: $DELETE_RESPONSE"

# Configurar nuevo webhook
echo -e "\n${YELLOW}🔧 Configurando nuevo webhook...${NC}"
SET_RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d "{
    \"url\": \"${WEBHOOK_URL}\",
    \"secret_token\": \"${WEBHOOK_SECRET}\",
    \"allowed_updates\": [\"message\", \"callback_query\"],
    \"drop_pending_updates\": true,
    \"max_connections\": 100
  }")

echo "  Respuesta: $SET_RESPONSE"

# Verificar webhook
echo -e "\n${YELLOW}✅ Verificando webhook...${NC}"
INFO_RESPONSE=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getWebhookInfo")
echo "  Info: $INFO_RESPONSE"

# Obtener info del bot
echo -e "\n${YELLOW}🤖 Info del bot:${NC}"
BOT_INFO=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getMe")
echo "  $BOT_INFO"

echo -e "\n${GREEN}✅ Webhook configurado exitosamente!${NC}"
echo ""
echo -e "${YELLOW}⚠️  IMPORTANTE: Guarda este secret token:${NC}"
echo -e "${GREEN}${WEBHOOK_SECRET}${NC}"
echo ""
echo "Debes agregar este secret a tus variables de entorno:"
echo "  TELEGRAM_WEBHOOK_SECRET=${WEBHOOK_SECRET}"


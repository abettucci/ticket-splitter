#!/bin/bash
# ============================================
# Script para probar el bot de Telegram
# ============================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}🧪 SplitBot Test Script${NC}"
echo "========================="

if [ -z "$1" ]; then
    echo -e "${YELLOW}Uso: $0 <BOT_TOKEN>${NC}"
    exit 1
fi

BOT_TOKEN=$1

# Obtener info del bot
echo -e "\n${YELLOW}🤖 Obteniendo info del bot...${NC}"
BOT_INFO=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getMe")
echo "$BOT_INFO" | jq .

# Obtener info del webhook
echo -e "\n${YELLOW}🔗 Info del webhook:${NC}"
WEBHOOK_INFO=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getWebhookInfo")
echo "$WEBHOOK_INFO" | jq .

# Verificar estado
if echo "$WEBHOOK_INFO" | jq -e '.result.url | length > 0' > /dev/null 2>&1; then
    echo -e "\n${GREEN}✅ Webhook configurado correctamente${NC}"
    
    WEBHOOK_URL=$(echo "$WEBHOOK_INFO" | jq -r '.result.url')
    PENDING_COUNT=$(echo "$WEBHOOK_INFO" | jq -r '.result.pending_update_count')
    LAST_ERROR=$(echo "$WEBHOOK_INFO" | jq -r '.result.last_error_message // "ninguno"')
    
    echo "  URL: $WEBHOOK_URL"
    echo "  Updates pendientes: $PENDING_COUNT"
    echo "  Último error: $LAST_ERROR"
else
    echo -e "\n${RED}❌ Webhook no configurado${NC}"
    echo "Ejecuta: ./setup-webhook.sh <TOKEN> <URL>"
fi

# Obtener últimos updates (solo si no hay webhook)
if [ -z "$(echo "$WEBHOOK_INFO" | jq -r '.result.url')" ]; then
    echo -e "\n${YELLOW}📨 Últimos updates (polling):${NC}"
    UPDATES=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates?limit=5")
    echo "$UPDATES" | jq .
fi

echo -e "\n${GREEN}✅ Test completado${NC}"


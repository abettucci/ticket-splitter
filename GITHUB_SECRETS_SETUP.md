# GitHub Secrets Setup

## Secrets Requeridos

Configura estos secrets en: **Settings → Secrets and variables → Actions → New repository secret**

| Secret Name | Descripción | Ejemplo |
|-------------|-------------|---------|
| `AWS_ACCESS_KEY_ID` | AWS Access Key | `AKIA...` |
| `AWS_SECRET_ACCESS_KEY` | AWS Secret Key | `wJalrXU...` |
| `AWS_REGION` | Región AWS | `us-east-1` |
| `TELEGRAM_BOT_TOKEN` | Token de @BotFather | `123456789:ABC...` |

## Deploy

```bash
git push origin main
```

El workflow ejecutará automáticamente:
1. Build del backend Go
2. Terraform apply con los secrets de GitHub
3. Configuración del webhook de Telegram

## Variables Terraform

El workflow pasa los secrets así:

```yaml
env:
  TF_VAR_aws_region: ${{ secrets.AWS_REGION }}
  TF_VAR_telegram_bot_token: ${{ secrets.TELEGRAM_BOT_TOKEN }}
  TF_VAR_environment: "prod"
  TF_VAR_project_name: "splitbot"
```

Terraform los recibe como variables y los pasa directamente a la Lambda.

## Verificar Deploy

```bash
# Ver logs
aws logs tail /aws/lambda/splitbot-prod --follow

# Verificar webhook
curl "https://api.telegram.org/bot<TOKEN>/getWebhookInfo" | jq
```


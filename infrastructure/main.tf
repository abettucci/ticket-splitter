# ============================================
# TERRAFORM CONFIGURATION
# ============================================

terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }

  # Backend S3 para estado remoto (opcional - descomentar en producción)
  # backend "s3" {
  #   bucket         = "splitbot-terraform-state"
  #   key            = "splitbot/terraform.tfstate"
  #   region         = "us-east-1"
  #   encrypt        = true
  #   dynamodb_table = "terraform-locks"
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "SplitBot"
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

# ============================================
# VARIABLES
# ============================================

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
  default     = "prod"
}

variable "telegram_bot_token" {
  description = "Telegram Bot Token from BotFather"
  type        = string
  sensitive   = true
}

variable "whatsapp_verify_token" {
  description = "WhatsApp webhook verify token"
  type        = string
  sensitive   = true
}

variable "twilio_account_sid" {
  description = "twilio_account_sid"
  type        = string
  sensitive   = true
}

variable "twilio_auth_token" {
  description = "twilio_auth_token"
  type        = string
  sensitive   = true
}

variable "twilio_whatsapp_from" {
  description = "twilio_whatsapp_from"
  type        = string
  sensitive   = true
}

variable "twilio_webhook_url" {
  description = "twilio_webhook_url"
  type        = string
  sensitive   = true
}

variable "twilio_skip_signature" {
  description = "twilio_skip_signature"
  type        = string
  sensitive   = true
}


variable "project_name" {
  description = "Project name"
  type        = string
  default     = "splitbot"
}

# ============================================
# LOCALS
# ============================================

locals {
  function_name = "${var.project_name}-${var.environment}"
  table_name    = "${var.project_name}-${var.environment}"
}

# ============================================
# RANDOM STRING FOR WEBHOOK SECRET
# ============================================

resource "random_password" "webhook_secret" {
  length  = 64
  special = false
}


# ============================================
# DYNAMODB TABLE - Con encriptación KMS
# ============================================

resource "aws_dynamodb_table" "main" {
  name         = local.table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  server_side_encryption {
    enabled = true
  }

  point_in_time_recovery {
    enabled = true
  }

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  attribute {
    name = "GSI1PK"
    type = "S"
  }

  attribute {
    name = "GSI1SK"
    type = "S"
  }

  global_secondary_index {
    name            = "GSI1"
    hash_key        = "GSI1PK"
    range_key       = "GSI1SK"
    projection_type = "ALL"
  }

  ttl {
    attribute_name = "ttl"
    enabled        = true
  }

  tags = {
    Name = local.table_name
  }
}

# ============================================
# IAM ROLE PARA LAMBDA - Principio de menor privilegio
# ============================================

resource "aws_iam_role" "lambda_role" {
  name = "${local.function_name}-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy" "lambda_logs" {
  name = "${local.function_name}-logs"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:${var.aws_region}:*:log-group:/aws/lambda/${local.function_name}:*"
      }
    ]
  })
}

resource "aws_iam_role_policy" "lambda_dynamodb" {
  name = "${local.function_name}-dynamodb"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:Query",
          "dynamodb:Scan"
        ]
        Resource = [
          aws_dynamodb_table.main.arn,
          "${aws_dynamodb_table.main.arn}/index/*"
        ]
      }
    ]
  })
}


# ============================================
# CLOUDWATCH LOG GROUP
# ============================================

resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${local.function_name}"
  retention_in_days = 14

  tags = {
    Name = "${local.function_name}-logs"
  }
}

resource "aws_cloudwatch_log_group" "api_logs" {
  name              = "/aws/apigateway/${local.function_name}"
  retention_in_days = 7

  tags = {
    Name = "${local.function_name}-api-logs"
  }
}

# ============================================
# LAMBDA FUNCTION
# ============================================

resource "aws_lambda_function" "bot" {
  function_name = local.function_name
  role          = aws_iam_role.lambda_role.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  filename         = "${path.module}/../backend/function.zip"
  source_code_hash = filebase64sha256("${path.module}/../backend/function.zip")

  memory_size = 128
  timeout     = 10

  environment {
    variables = {
      DYNAMODB_TABLE          = aws_dynamodb_table.main.name
      TELEGRAM_BOT_TOKEN      = var.telegram_bot_token
      TELEGRAM_WEBHOOK_SECRET = random_password.webhook_secret.result
      ENVIRONMENT             = var.environment
      WHATSAPP_VERIFY_TOKEN   = var.whatsapp_verify_token
      TWILIO_ACCOUNT_SID      = var.twilio_account_sid
      TWILIO_AUTH_TOKEN       = var.twilio_auth_token
      TWILIO_WHATSAPP_FROM    = var.twilio_whatsapp_from
      TWILIO_WEBHOOK_URL      = var.twilio_webhook_url
      TWILIO_SKIP_SIGNATURE   = var.twilio_skip_signature
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.lambda_logs,
    aws_iam_role_policy.lambda_logs,
    aws_iam_role_policy.lambda_dynamodb,
  ]

  tags = {
    Name = local.function_name
  }
}

# ============================================
# API GATEWAY V2 (HTTP API)
# ============================================

resource "aws_apigatewayv2_api" "bot_api" {
  name          = "${local.function_name}-api"
  protocol_type = "HTTP"
  description   = "SplitBot Telegram Webhook API"

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["GET", "POST", "OPTIONS"]
    allow_headers = ["content-type", "x-telegram-bot-api-secret-token"]
    max_age       = 300
  }

  tags = {
    Name = "${local.function_name}-api"
  }
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.bot_api.id
  name        = "$default"
  auto_deploy = true

  default_route_settings {
    throttling_burst_limit = 100
    throttling_rate_limit  = 50
  }

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_logs.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      responseLength = "$context.responseLength"
    })
  }

  tags = {
    Name = "${local.function_name}-stage"
  }
}

resource "aws_apigatewayv2_integration" "lambda" {
  api_id                 = aws_apigatewayv2_api.bot_api.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.bot.invoke_arn
  integration_method     = "POST"
  payload_format_version = "1.0"
}

resource "aws_apigatewayv2_route" "webhook" {
  api_id    = aws_apigatewayv2_api.bot_api.id
  route_key = "POST /webhook"
  target    = "integrations/${aws_apigatewayv2_integration.lambda.id}"
}

resource "aws_apigatewayv2_route" "webhook_get" {
  api_id    = aws_apigatewayv2_api.bot_api.id
  route_key = "GET /webhook"
  target    = "integrations/${aws_apigatewayv2_integration.lambda.id}"
}

resource "aws_apigatewayv2_route" "twilio_inbound" {
  api_id    = aws_apigatewayv2_api.bot_api.id
  route_key = "POST /twilio/inbound"
  target    = "integrations/${aws_apigatewayv2_integration.lambda.id}"
}

resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.bot.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.bot_api.execution_arn}/*/*"
}

# ============================================
# WAF (OPCIONAL - Cuesta ~$5/mes)
# Descomenta si quieres protección adicional
# ============================================

# resource "aws_wafv2_web_acl" "bot_waf" {
#   name        = "${local.function_name}-waf"
#   description = "WAF for SplitBot API"
#   scope       = "REGIONAL"
#
#   default_action {
#     allow {}
#   }
#
#   rule {
#     name     = "RateLimitRule"
#     priority = 1
#
#     override_action {
#       none {}
#     }
#
#     statement {
#       rate_based_statement {
#         limit              = 1000
#         aggregate_key_type = "IP"
#       }
#     }
#
#     visibility_config {
#       cloudwatch_metrics_enabled = true
#       metric_name                = "RateLimitRule"
#       sampled_requests_enabled   = true
#     }
#   }
#
#   visibility_config {
#     cloudwatch_metrics_enabled = true
#     metric_name                = "SplitBotWAF"
#     sampled_requests_enabled   = true
#   }
#
#   tags = {
#     Name = "${local.function_name}-waf"
#   }
# }

# resource "aws_wafv2_web_acl_association" "api_waf" {
#   resource_arn = aws_apigatewayv2_stage.default.arn
#   web_acl_arn  = aws_wafv2_web_acl.bot_waf.arn
# }

# ============================================
# CLOUDWATCH ALARMS
# ============================================

resource "aws_cloudwatch_metric_alarm" "high_error_rate" {
  alarm_name          = "${local.function_name}-high-error-rate"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = 10
  alarm_description   = "High error rate detected in Lambda function"

  dimensions = {
    FunctionName = aws_lambda_function.bot.function_name
  }

  tags = {
    Name = "${local.function_name}-error-alarm"
  }
}

# ============================================
# OUTPUTS
# ============================================

output "webhook_url" {
  description = "Telegram webhook URL"
  value       = "${aws_apigatewayv2_api.bot_api.api_endpoint}/webhook"
}

output "twilio_webhook_url" {
  description = "Twilio WhatsApp webhook URL — use this as TWILIO_WEBHOOK_URL"
  value       = "${aws_apigatewayv2_api.bot_api.api_endpoint}/twilio/inbound"
}

output "webhook_secret" {
  description = "Webhook secret token"
  value       = random_password.webhook_secret.result
  sensitive   = true
}

output "lambda_function_name" {
  description = "Lambda function name"
  value       = aws_lambda_function.bot.function_name
}

output "dynamodb_table_name" {
  description = "DynamoDB table name"
  value       = aws_dynamodb_table.main.name
}

output "api_gateway_id" {
  description = "API Gateway ID"
  value       = aws_apigatewayv2_api.bot_api.id
}

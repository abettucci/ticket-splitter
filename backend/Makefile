.PHONY: build clean deploy test lint

# Variables
BINARY_NAME=bootstrap
LAMBDA_ZIP=function.zip
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")

# Build para AWS Lambda (Amazon Linux 2)
build:
	@echo "🔨 Building for AWS Lambda..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags="-s -w" -o $(BINARY_NAME) .
	@echo "📦 Creating zip..."
	zip $(LAMBDA_ZIP) $(BINARY_NAME)
	@echo "✅ Build complete: $(LAMBDA_ZIP)"

# Build para ARM64 (Graviton2 - más barato)
build-arm:
	@echo "🔨 Building for AWS Lambda (ARM64)..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags="-s -w" -o $(BINARY_NAME) .
	@echo "📦 Creating zip..."
	zip $(LAMBDA_ZIP) $(BINARY_NAME)
	@echo "✅ Build complete: $(LAMBDA_ZIP)"

# Limpiar artefactos
clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME) $(LAMBDA_ZIP)
	go clean

# Ejecutar tests
test:
	@echo "🧪 Running tests..."
	go test -v -race -cover ./...

# Ejecutar linter
lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...

# Descargar dependencias
deps:
	@echo "📥 Downloading dependencies..."
	go mod download
	go mod tidy

# Verificar vulnerabilidades
security:
	@echo "🔒 Checking for vulnerabilities..."
	govulncheck ./...

# Deploy con Terraform
deploy: build
	@echo "🚀 Deploying with Terraform..."
	cd ../infrastructure && terraform apply -auto-approve

# Inicializar Terraform
init:
	@echo "🏗️ Initializing Terraform..."
	cd ../infrastructure && terraform init

# Plan de Terraform
plan: build
	@echo "📋 Planning Terraform changes..."
	cd ../infrastructure && terraform plan

# Desarrollo local (para testing)
dev:
	@echo "🖥️ Running locally..."
	go run .

# Actualizar dependencias
update:
	@echo "⬆️ Updating dependencies..."
	go get -u ./...
	go mod tidy

# Formatear código
fmt:
	@echo "✨ Formatting code..."
	gofmt -s -w $(GO_FILES)

# Ver tamaño del binario
size: build
	@echo "📏 Binary size:"
	@ls -lh $(BINARY_NAME)
	@ls -lh $(LAMBDA_ZIP)


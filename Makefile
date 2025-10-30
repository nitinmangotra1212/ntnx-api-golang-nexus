.PHONY: help build run clean test fmt lint docker-build docker-run

# Variables
BINARY_NAME=mock-api-server
MAIN_PATH=./cmd/server
DOCKER_IMAGE=ntnx-api-golang-mock
VERSION?=1.0.0

# Default target
help:
	@echo "Available targets:"
	@echo "  make build          - Build the application"
	@echo "  make run            - Run the application"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linter"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run Docker container"
	@echo "  make install-deps   - Install dependencies"
	@echo "  make all            - Build and run"

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) -ldflags="-s -w" $(MAIN_PATH)
	@echo "Build complete: ./$(BINARY_NAME)"

# Build for production (with optimizations)
build-prod:
	@echo "Building $(BINARY_NAME) for production..."
	CGO_ENABLED=0 go build -o $(BINARY_NAME) \
		-ldflags="-s -w -X main.version=$(VERSION)" \
		-trimpath \
		$(MAIN_PATH)
	@echo "Production build complete: ./$(BINARY_NAME)"

# Build for different platforms
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux $(MAIN_PATH)

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME).exe $(MAIN_PATH)

build-mac:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-mac $(MAIN_PATH)

build-all: build-linux build-windows build-mac
	@echo "Built for all platforms"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	go run $(MAIN_PATH)

# Run with hot reload (requires air)
dev:
	@echo "Running with hot reload..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		go run $(MAIN_PATH); \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-linux
	rm -f $(BINARY_NAME).exe
	rm -f $(BINARY_NAME)-mac
	go clean

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from https://golangci-lint.run/"; \
	fi

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest
	@echo "Docker image built: $(DOCKER_IMAGE):$(VERSION)"

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 9009:9009 \
		-v $(PWD)/configs:/app/configs \
		--name mock-api-server \
		$(DOCKER_IMAGE):latest

# Stop Docker container
docker-stop:
	docker stop mock-api-server
	docker rm mock-api-server

# Generate OpenAPI code (if using oapi-codegen)
generate:
	@echo "Generating code from OpenAPI spec..."
	@if command -v oapi-codegen > /dev/null; then \
		oapi-codegen -package codegen -generate types,server \
			golang-mock-api-definitions/openapi.yaml > golang-mock-codegen/generated.go; \
		echo "Code generation complete"; \
	else \
		echo "oapi-codegen not installed. Install with: go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest"; \
	fi

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest
	@echo "Development tools installed"

# Build and run
all: build run

#!/bin/bash

# Start script for Mock REST API Server

set -e

echo "=================================="
echo "Mock REST API Server - Start Script"
echo "=================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed"
    echo "Please install Go 1.21 or higher from https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Go version: $GO_VERSION"

# Check if go.mod exists
if [ ! -f "go.mod" ]; then
    echo "❌ Error: go.mod not found"
    echo "Please run this script from the project root directory"
    exit 1
fi

echo ""
echo "📦 Downloading dependencies..."
go mod download
go mod tidy

echo ""
echo "🔨 Building application..."
go build -o mock-api-server ./cmd/server

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo ""
    echo "🚀 Starting Mock REST API Server..."
    echo "   Server will run on: http://localhost:9009"
    echo "   Press CTRL+C to stop"
    echo ""
    ./mock-api-server
else
    echo "❌ Build failed"
    exit 1
fi

#!/bin/bash
# Setup script for Go module dependencies
# Usage: ./setup-dependencies.sh YOUR_GITHUB_TOKEN

set -e

if [ -z "$1" ]; then
    echo "❌ Error: GitHub token required"
    echo "Usage: ./setup-dependencies.sh YOUR_GITHUB_TOKEN"
    exit 1
fi

GITHUB_TOKEN=$1

echo "🔧 Setting up Go module dependencies..."
echo ""

# Step 1: Set environment variables
echo "Step 1: Setting environment variables..."
export GITHUB_TOKEN=$GITHUB_TOKEN
export GOPRIVATE=github.com/nutanix-core/*
export GONOSUMDB=github.com/nutanix-core/*

echo "✅ GOPRIVATE=$GOPRIVATE"
echo "✅ GONOSUMDB=$GONOSUMDB"
echo "✅ GITHUB_TOKEN is set"
echo ""

# Step 2: Remove old SSH config
echo "Step 2: Removing old SSH config..."
git config --global --unset url."git@github.com:nutanix-core/".insteadOf 2>/dev/null || true
echo "✅ Removed SSH config"
echo ""

# Step 3: Configure Git with token
echo "Step 3: Configuring Git with token..."
git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"
echo "✅ Git configured with token"
echo ""

# Step 4: Clean module cache
echo "Step 4: Cleaning module cache..."
go clean -modcache 2>/dev/null || true
echo "✅ Module cache cleaned"
echo ""

# Step 5: Download dependencies
echo "Step 5: Downloading dependencies..."
cd "$(dirname "$0")"
export GOPRIVATE=github.com/nutanix-core/*
export GONOSUMDB=github.com/nutanix-core/*
go mod download
echo "✅ Dependencies downloaded"
echo ""

# Step 6: Update go.mod and go.sum
echo "Step 6: Running go mod tidy..."
go mod tidy
echo "✅ go.mod and go.sum updated"
echo ""

echo "🎉 Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Build: make build"
echo "  2. Deploy to PC (follow SIMPLE_DEPLOYMENT_STEPS.md)"


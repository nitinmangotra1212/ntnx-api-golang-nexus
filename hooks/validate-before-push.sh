#!/bin/bash

# Validation script to run before pushing code
# This ensures everything works correctly

set -e  # Exit on any error

echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║           🔍 PRE-PUSH VALIDATION SCRIPT                                ║"
echo "║           Testing Everything Before You Push                           ║"
echo "╚════════════════════════════════════════════════════════════════════════╝"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

VALIDATION_PASSED=true

# Function to print success
success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# Function to print error
error() {
    echo -e "${RED}❌ $1${NC}"
    VALIDATION_PASSED=false
}

# Function to print info
info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# Function to print warning
warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 1: Check Prerequisites"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check Go version
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    success "Go installed: $GO_VERSION"
else
    error "Go is not installed"
fi

# Check if ntnx-api-golang-mock-pc exists
if [ -d "/Users/nitin.mangotra/ntnx-api-golang-mock-pc" ]; then
    success "ntnx-api-golang-mock-pc repository exists"
else
    error "ntnx-api-golang-mock-pc repository not found"
fi

# Check if generated DTOs exist
if [ -f "/Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/dto/src/models/mock/v4/config/config_model.go" ]; then
    success "Generated DTOs exist"
else
    error "Generated DTOs not found - run 'mvn clean install' in ntnx-api-golang-mock-pc first"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 2: Check Go Module Dependencies"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Check go.mod
if grep -q "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto" go.mod; then
    success "go.mod has correct DTO import"
else
    error "go.mod missing DTO import"
fi

if grep -q "replace.*ntnx-api-golang-mock-pc.*generated-code/dto" go.mod; then
    success "go.mod has replace directive"
else
    error "go.mod missing replace directive"
fi

# Try to download dependencies
info "Downloading Go dependencies..."
if go mod download 2>&1 | grep -q "error"; then
    error "Failed to download Go dependencies"
else
    success "Go dependencies downloaded"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 3: Build Servers"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Build API server
info "Building API Handler Server..."
if go build -o /tmp/validate-api-server ./cmd/api-server/main.go 2>/dev/null; then
    success "API Handler Server builds successfully"
    rm -f /tmp/validate-api-server
else
    error "API Handler Server build failed"
fi

# Build Task server
info "Building Task Server..."
if go build -o /tmp/validate-task-server ./cmd/task-server/main.go 2>/dev/null; then
    success "Task Server builds successfully"
    rm -f /tmp/validate-task-server
else
    error "Task Server build failed"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 4: Check for Hardcoded \$objectType Strings"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if service uses generated constructors
if grep -q "generated.NewCat()" services/cat_service_with_dto.go; then
    success "Service uses generated.NewCat() constructor"
else
    warning "Service may not be using generated constructors"
fi

# Check for hardcoded objectType strings (should be very few)
HARDCODED_COUNT=$(grep -r '\$objectType.*:.*"mock\.v4\.config\.' services/ 2>/dev/null | grep -v "\/\/" | wc -l || echo 0)
if [ "$HARDCODED_COUNT" -eq 0 ]; then
    success "No hardcoded \$objectType strings in services/"
else
    warning "Found $HARDCODED_COUNT hardcoded \$objectType strings (should use generated constructors)"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 5: Start Servers for Live Testing"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if servers are already running
if lsof -i :9009 > /dev/null 2>&1; then
    warning "Port 9009 already in use - stopping old server"
    pkill -f api-server || true
    sleep 2
fi

if lsof -i :9010 > /dev/null 2>&1; then
    warning "Port 9010 already in use - stopping old server"
    pkill -f task-server || true
    sleep 2
fi

info "Building and starting servers..."

# Build
go build -o bin/api-server ./cmd/api-server/main.go
go build -o bin/task-server ./cmd/task-server/main.go

# Start Task Server
./bin/task-server > /tmp/validate-task-server.log 2>&1 &
TASK_PID=$!
success "Task Server started (PID: $TASK_PID)"

sleep 3

# Start API Server
./bin/api-server > /tmp/validate-api-server.log 2>&1 &
API_PID=$!
success "API Handler Server started (PID: $API_PID)"

sleep 3

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 6: Test API Endpoints"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Test API Server health
info "Testing API Handler Server health..."
if curl -s http://localhost:9009/health | grep -q "healthy"; then
    success "API Handler Server is healthy"
else
    error "API Handler Server health check failed"
fi

# Test Task Server health
info "Testing Task Server health..."
if curl -s http://localhost:9010/health | grep -q "healthy"; then
    success "Task Server is healthy"
else
    error "Task Server health check failed"
fi

# Test List Cats
info "Testing List Cats endpoint..."
RESPONSE=$(curl -s "http://localhost:9009/mock/v4/config/cats?\$page=1&\$limit=3")

# Check for $objectType
if echo "$RESPONSE" | grep -q '"$objectType".*"mock.v4.config.ListCatsApiResponse"'; then
    success "List response has correct \$objectType"
else
    error "List response missing or incorrect \$objectType"
fi

# Check for $reserved
if echo "$RESPONSE" | grep -q '"$reserved"'; then
    success "List response has \$reserved"
else
    error "List response missing \$reserved"
fi

# Check for pagination metadata
if echo "$RESPONSE" | grep -q '"totalAvailableResults"'; then
    success "List response has pagination metadata"
else
    error "List response missing pagination metadata"
fi

# Check for HATEOAS links
if echo "$RESPONSE" | grep -q '"links"' && echo "$RESPONSE" | grep -q '"rel".*"self"'; then
    success "List response has HATEOAS links"
else
    error "List response missing HATEOAS links"
fi

# Test Get Cat by ID
info "Testing Get Cat by ID..."
RESPONSE=$(curl -s http://localhost:9009/mock/v4/config/cat/42)

if echo "$RESPONSE" | grep -q '"$objectType".*"mock.v4.config.Cat"'; then
    success "Get Cat response has correct \$objectType"
else
    error "Get Cat response missing or incorrect \$objectType"
fi

# Check nested objects
if echo "$RESPONSE" | grep -q '"$objectType".*"mock.v4.config.Location"'; then
    success "Nested Location has auto-set \$objectType"
else
    warning "Nested Location may be missing \$objectType"
fi

# Test Create Cat
info "Testing Create Cat..."
RESPONSE=$(curl -s -X POST http://localhost:9009/mock/v4/config/cats \
  -H "Content-Type: application/json" \
  -d '{"catName":"TestCat","catType":"TestType"}')

if echo "$RESPONSE" | grep -q '"$objectType".*"mock.v4.config.Cat"'; then
    success "Created Cat has correct \$objectType"
else
    error "Created Cat missing or incorrect \$objectType"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 7: Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

info "Stopping servers..."
kill $API_PID $TASK_PID 2>/dev/null || true
sleep 2

# Make sure they're stopped
pkill -f api-server 2>/dev/null || true
pkill -f task-server 2>/dev/null || true

success "Servers stopped"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "VALIDATION SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "$VALIDATION_PASSED" = true ]; then
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                  ✅ ALL VALIDATIONS PASSED!                            ║${NC}"
    echo -e "${GREEN}║                  Your code is ready to push!                           ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Review git status: git status"
    echo "  2. Commit: git commit -m 'Your message'"
    echo "  3. Push: git push origin main"
    echo ""
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║                  ❌ VALIDATION FAILED!                                 ║${NC}"
    echo -e "${RED}║                  Please fix errors before pushing                      ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Check the logs:"
    echo "  - /tmp/validate-api-server.log"
    echo "  - /tmp/validate-task-server.log"
    echo ""
    exit 1
fi


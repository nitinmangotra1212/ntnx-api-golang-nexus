#!/bin/bash

# Quick test script for the Mock REST API server

set -e

echo "======================================"
echo "Mock REST API Server - Test Script"
echo "======================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PORT=9009
BASE_URL="http://localhost:$PORT"

# Check if server is already running
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null ; then
    echo -e "${YELLOW}⚠️  Server already running on port $PORT${NC}"
    echo "Using existing server for tests..."
    echo ""
    STOP_SERVER=false
else
    # Start server in background
    echo "🚀 Starting server..."
    ./mock-api-server > /dev/null 2>&1 &
    SERVER_PID=$!
    STOP_SERVER=true
    
    # Wait for server to start
    echo "⏳ Waiting for server to start..."
    for i in {1..30}; do
        if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Server started successfully (PID: $SERVER_PID)${NC}"
            break
        fi
        if [ $i -eq 30 ]; then
            echo -e "${RED}❌ Server failed to start${NC}"
            exit 1
        fi
        sleep 1
    done
fi

echo ""
echo "Running API tests..."
echo "------------------------------------"

# Test 1: Health check
echo -n "Test 1: Health check... "
RESPONSE=$(curl -s "$BASE_URL/health")
if echo "$RESPONSE" | grep -q '"status":"UP"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 2: List cats
echo -n "Test 2: List cats... "
RESPONSE=$(curl -s "$BASE_URL/mock/v4/config/cats?limit=2")
if echo "$RESPONSE" | grep -q '"catName"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 3: Get cat by ID
echo -n "Test 3: Get cat by ID... "
RESPONSE=$(curl -s "$BASE_URL/mock/v4/config/cat/5")
if echo "$RESPONSE" | grep -q '"catId"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 4: Create cat
echo -n "Test 4: Create cat... "
RESPONSE=$(curl -s -X POST "$BASE_URL/mock/v4/config/cats" \
    -H "Content-Type: application/json" \
    -d '{"catName":"TestCat","catType":"TYPE1","description":"Test"}')
if echo "$RESPONSE" | grep -q '"catName":"TestCat"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 5: Update cat
echo -n "Test 5: Update cat... "
RESPONSE=$(curl -s -X PUT "$BASE_URL/mock/v4/config/cat/10" \
    -H "Content-Type: application/json" \
    -d '{"catName":"UpdatedCat"}')
if echo "$RESPONSE" | grep -q '"catId"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 6: Delete cat
echo -n "Test 6: Delete cat... "
RESPONSE=$(curl -s -X DELETE "$BASE_URL/mock/v4/config/cat/15")
if echo "$RESPONSE" | grep -q '"catId"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 7: Add IPv4
echo -n "Test 7: Add IPv4 to cat... "
RESPONSE=$(curl -s -X POST "$BASE_URL/mock/v4/config/cat/5/ipv4" \
    -H "Content-Type: application/json" \
    -d '{"value":"192.168.1.100","prefixLength":24}')
if echo "$RESPONSE" | grep -q '"value":"192.168.1.100"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 8: Get task status with UUID
echo -n "Test 8: Get task status by UUID... "
UUID1="550e8400-e29b-41d4-a716-446655440000"
UUID2="660e8400-e29b-41d4-a716-446655440001"
RESPONSE=$(curl -s -H "NTNX-Request-ID: $UUID1" \
    -H "USER-Request-ID: $UUID2" \
    "$BASE_URL/mock/v4/config/cat/status")
if echo "$RESPONSE" | grep -q '"extId"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

# Test 9: Test with delay
echo -n "Test 9: Test with delay (2s)... "
START_TIME=$(date +%s)
RESPONSE=$(curl -s "$BASE_URL/mock/v4/config/cats?delay=2000&limit=1")
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
if [ $DURATION -ge 2 ] && echo "$RESPONSE" | grep -q '"catName"'; then
    echo -e "${GREEN}✅ PASSED (took ${DURATION}s)${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Duration: ${DURATION}s, Response: $RESPONSE"
fi

# Test 10: Test with size parameter (File download test removed due to SFTP dependency)
echo -n "Test 10: Test with size=small... "
RESPONSE=$(curl -s "$BASE_URL/mock/v4/config/cats?size=small&limit=1")
if echo "$RESPONSE" | grep -q '"catName"'; then
    echo -e "${GREEN}✅ PASSED${NC}"
else
    echo -e "${RED}❌ FAILED${NC}"
    echo "Response: $RESPONSE"
fi

echo ""
echo "------------------------------------"
echo -e "${GREEN}All tests completed!${NC}"
echo ""

# Stop server if we started it
if [ "$STOP_SERVER" = true ]; then
    echo "🛑 Stopping server (PID: $SERVER_PID)..."
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
    echo "Server stopped."
fi

echo ""
echo "======================================"
echo "✅ Test suite completed successfully!"
echo "======================================"


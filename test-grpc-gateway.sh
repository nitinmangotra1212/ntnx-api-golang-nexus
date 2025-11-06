#!/bin/bash

# Test script for gRPC Gateway Architecture
# Tests both synchronous and asynchronous operations

echo "🧪 Testing Nutanix gRPC Gateway Architecture"
echo "============================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

API_URL="http://localhost:9009"
TASK_URL="http://localhost:9010"

# Test 1: Synchronous GET - List Cats
echo -e "${GREEN}Test 1: List Cats (Synchronous)${NC}"
echo "GET $API_URL/mock/v4/config/cats?\$page=1&\$limit=3"
curl -s "$API_URL/mock/v4/config/cats?\$page=1&\$limit=3" | jq '.' || curl -s "$API_URL/mock/v4/config/cats?\$page=1&\$limit=3"
echo ""
echo "---"
echo ""

# Test 2: Synchronous GET - Get Cat by ID
echo -e "${GREEN}Test 2: Get Cat by ID (Synchronous)${NC}"
echo "GET $API_URL/mock/v4/config/cat/42"
curl -s "$API_URL/mock/v4/config/cat/42" | jq '.' || curl -s "$API_URL/mock/v4/config/cat/42"
echo ""
echo "---"
echo ""

# Test 3: Synchronous POST - Create Cat
echo -e "${GREEN}Test 3: Create Cat (Synchronous)${NC}"
echo "POST $API_URL/mock/v4/config/cats"
curl -s -X POST "$API_URL/mock/v4/config/cats" \
  -H "Content-Type: application/json" \
  -d '{"catName":"Whiskers","catType":"Persian","description":"A fluffy cat"}' | jq '.' || \
  curl -s -X POST "$API_URL/mock/v4/config/cats" -H "Content-Type: application/json" -d '{"catName":"Whiskers","catType":"Persian"}'
echo ""
echo "---"
echo ""

# Test 4: Asynchronous GET - Start Async Operation
echo -e "${BLUE}Test 4: Get Cat Async (Returns Task ID)${NC}"
echo "GET $API_URL/mock/v4/config/cat/42/async"
RESPONSE=$(curl -s "$API_URL/mock/v4/config/cat/42/async")
echo "$RESPONSE" | jq '.' || echo "$RESPONSE"
echo ""

# Extract task ID
TASK_ID=$(echo "$RESPONSE" | jq -r '.taskId' 2>/dev/null || echo "")
if [ -z "$TASK_ID" ] || [ "$TASK_ID" = "null" ]; then
    echo "⚠️  Could not extract task ID"
else
    echo -e "${YELLOW}Task ID: $TASK_ID${NC}"
    echo ""
    echo "---"
    echo ""
    
    # Test 5: Poll Task Progress
    echo -e "${BLUE}Test 5: Polling Task Progress${NC}"
    echo "This demonstrates the async workflow..."
    echo ""
    
    for i in {1..4}; do
        echo -e "${YELLOW}Poll #$i - GET $TASK_URL/tasks/$TASK_ID${NC}"
        TASK_RESPONSE=$(curl -s "$TASK_URL/tasks/$TASK_ID")
        echo "$TASK_RESPONSE" | jq '.' || echo "$TASK_RESPONSE"
        
        PERCENTAGE=$(echo "$TASK_RESPONSE" | jq -r '.percentageComplete' 2>/dev/null || echo "0")
        STATUS=$(echo "$TASK_RESPONSE" | jq -r '.status' 2>/dev/null || echo "UNKNOWN")
        
        echo "Status: $STATUS | Progress: $PERCENTAGE%"
        echo ""
        
        if [ "$STATUS" = "COMPLETED" ]; then
            echo -e "${GREEN}✅ Task completed!${NC}"
            echo ""
            echo "Final result:"
            echo "$TASK_RESPONSE" | jq '.objectReturned' || echo "$TASK_RESPONSE"
            break
        fi
        
        if [ $i -lt 4 ]; then
            echo "Waiting 3 seconds before next poll..."
            sleep 3
            echo ""
        fi
    done
fi

echo ""
echo "============================================="
echo "✅ All tests complete!"
echo ""
echo "💡 Key Observations:"
echo "   1. Synchronous operations return data immediately"
echo "   2. Async operations return task ID"
echo "   3. Client polls Task Server for progress"
echo "   4. All responses have \$objectType and \$reserved"
echo ""


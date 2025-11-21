#!/bin/bash

# Start both gRPC Gateway servers
# Following Nutanix two-server architecture pattern

echo "🚀 Starting Nutanix gRPC Gateway Architecture"
echo "=============================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Build servers
echo "📦 Building servers..."
go build -o bin/api-server ./cmd/api-server/main.go
go build -o bin/task-server ./cmd/task-server/main.go
echo "✅ Build complete"
echo ""

# Start Task Server (Port 9010) in background
echo -e "${BLUE}Starting Task Server (Port 9010)...${NC}"
./bin/task-server > /tmp/task-server.log 2>&1 &
TASK_PID=$!
echo "✅ Task Server PID: $TASK_PID"
echo "   Logs: /tmp/task-server.log"
echo ""

# Wait a moment for Task Server to start
sleep 2

# Start API Handler Server (Port 9009) in background
echo -e "${GREEN}Starting API Handler Server (Port 9009)...${NC}"
./bin/api-server > /tmp/api-server.log 2>&1 &
API_PID=$!
echo "✅ API Handler Server PID: $API_PID"
echo "   Logs: /tmp/api-server.log"
echo ""

# Wait for servers to start
sleep 2

# Test health endpoints
echo "🏥 Testing health endpoints..."
echo ""

echo "API Handler Server:"
curl -s http://localhost:9009/health | jq '.' || curl -s http://localhost:9009/health
echo ""

echo "Task Server:"
curl -s http://localhost:9010/health | jq '.' || curl -s http://localhost:9010/health
echo ""

echo "=============================================="
echo "✅ Both servers are running!"
echo ""
echo "📝 API Handler Server (Port 9009):"
echo "   http://localhost:9009/mock/v4/config/cats"
echo ""
echo "📊 Task Server (Port 9010):"
echo "   http://localhost:9010/tasks/{taskId}"
echo ""
echo "🛑 To stop servers:"
echo "   kill $API_PID $TASK_PID"
echo "   OR run: ./stop-servers.sh"
echo ""
echo "📋 View logs:"
echo "   tail -f /tmp/api-server.log"
echo "   tail -f /tmp/task-server.log"
echo ""

# Save PIDs for stop script
echo "$API_PID" > /tmp/api-server.pid
echo "$TASK_PID" > /tmp/task-server.pid

echo "✅ Ready for testing!"


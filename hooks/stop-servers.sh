#!/bin/bash

# Stop both gRPC Gateway servers

echo "🛑 Stopping servers..."

if [ -f /tmp/api-server.pid ]; then
    API_PID=$(cat /tmp/api-server.pid)
    echo "Stopping API Handler Server (PID: $API_PID)..."
    kill $API_PID 2>/dev/null && echo "✅ API Handler Server stopped" || echo "⚠️  API Handler Server not running"
    rm /tmp/api-server.pid
fi

if [ -f /tmp/task-server.pid ]; then
    TASK_PID=$(cat /tmp/task-server.pid)
    echo "Stopping Task Server (PID: $TASK_PID)..."
    kill $TASK_PID 2>/dev/null && echo "✅ Task Server stopped" || echo "⚠️  Task Server not running"
    rm /tmp/task-server.pid
fi

echo ""
echo "✅ All servers stopped"


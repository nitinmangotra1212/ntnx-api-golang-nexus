# 🚀 How to Run - Complete Guide

Step-by-step instructions to build, run, and test the Mock API servers.

---

## 📋 Prerequisites

Before you start, ensure you have:

- ✅ **Go 1.21+** installed
- ✅ **ntnx-api-golang-mock-pc** repository cloned in the same parent directory
- ✅ Generated DTOs built (from ntnx-api-golang-mock-pc)

### Check Prerequisites

```bash
# Check Go version
go version
# Expected: go version go1.21 or higher

# Check repositories are in correct location
ls -la /Users/nitin.mangotra/
# Should see both: ntnx-api-golang-mock and ntnx-api-golang-mock-pc

# Check generated DTOs exist
ls /Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/dto/src/models/mock/v4/config/
# Should see: config_model.go
```

---

## 🔨 Step 1: Generate DTOs (If Not Already Done)

**Only needed if generated-code/ doesn't exist in ntnx-api-golang-mock-pc**

```bash
# Go to API definitions repository
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc

# Generate code from YAML
mvn clean install -s settings.xml

# This creates:
# - generated-code/dto/src/models/mock/v4/config/config_model.go
# - Auto-generated constructors: NewCat(), NewLocation(), NewCountry()
```

**Expected Output:**
```
[INFO] BUILD SUCCESS
[INFO] Total time: 2-3 minutes
```

---

## 📦 Step 2: Download Go Dependencies

```bash
# Go to service implementation repository
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Download all Go dependencies
go mod download

# Verify dependencies
go mod tidy
```

**Expected Output:**
```
go: downloading github.com/gorilla/mux v1.8.1
go: downloading github.com/sirupsen/logrus v1.9.3
... (more dependencies)
```

---

## 🏗️ Step 3: Build the Servers

### Option A: Build Both Servers at Once

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Build API Handler Server
go build -o bin/api-server ./cmd/api-server/main.go

# Build Task Server
go build -o bin/task-server ./cmd/task-server/main.go

# Verify binaries were created
ls -lh bin/
```

**Expected Output:**
```
bin/api-server   (executable, ~15-20MB)
bin/task-server  (executable, ~15-20MB)
```

### Option B: Use the Start Script (Builds + Runs)

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# This script builds and starts both servers
./start-servers.sh
```

---

## ▶️ Step 4: Start the Servers

### Option A: Manual Start

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Start Task Server (in background)
./bin/task-server > /tmp/task-server.log 2>&1 &
echo $! > /tmp/task-server.pid
echo "Task Server PID: $(cat /tmp/task-server.pid)"

# Wait 2 seconds
sleep 2

# Start API Handler Server (in background)
./bin/api-server > /tmp/api-server.log 2>&1 &
echo $! > /tmp/api-server.pid
echo "API Server PID: $(cat /tmp/api-server.pid)"

# Wait for servers to fully start
sleep 3
```

### Option B: Use the Start Script (Recommended)

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# This does everything: build + start both servers
./start-servers.sh
```

**Expected Output:**
```
🚀 Starting Nutanix gRPC Gateway Architecture
==============================================

📦 Building servers...
✅ Build complete

Starting Task Server (Port 9010)...
✅ Task Server PID: 12345
   Logs: /tmp/task-server.log

Starting API Handler Server (Port 9009)...
✅ API Handler Server PID: 12346
   Logs: /tmp/api-server.log

🏥 Testing health endpoints...

API Handler Server:
{"status":"healthy","server":"API Handler Server"}

Task Server:
{"status":"healthy","server":"Task Server"}

==============================================
✅ Both servers are running!
```

---

## ✅ Step 5: Verify Servers Are Running

### Check Server Status

```bash
# Check if processes are running
ps aux | grep -E "(api-server|task-server)" | grep -v grep

# Check if ports are listening
lsof -i :9009  # API Handler Server
lsof -i :9010  # Task Server

# Or use netstat (on some systems)
netstat -an | grep -E "(9009|9010)"
```

### Check Health Endpoints

```bash
# API Handler Server health
curl http://localhost:9009/health
# Expected: {"status":"healthy","server":"API Handler Server"}

# Task Server health
curl http://localhost:9010/health
# Expected: {"status":"healthy","server":"Task Server"}
```

---

## 🧪 Step 6: Test the API

### Quick Test (5 minutes)

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Run comprehensive test script
./test-grpc-gateway.sh
```

**This tests:**
1. ✅ List cats (synchronous)
2. ✅ Get cat by ID (synchronous)
3. ✅ Create cat (synchronous)
4. ✅ Get cat async (returns task ID)
5. ✅ Poll task progress (asynchronous workflow)

### Manual Tests

#### Test 1: List Cats (Pagination)

```bash
curl "http://localhost:9009/mock/v4/config/cats?$page=1&$limit=3" | jq
```

**Expected Response:**
```json
{
  "$objectType": "mock.v4.config.ListCatsApiResponse",
  "$reserved": {
    "$fv": "v4.r1"
  },
  "data": [
    {
      "$objectType": "mock.v4.config.Cat",
      "catId": 1,
      "catName": "Cat-1",
      "catType": "TYPE1"
    },
    // ... 2 more cats
  ],
  "metadata": {
    "$objectType": "common.v1.response.ApiResponseMetadata",
    "totalAvailableResults": 100,
    "flags": [...],
    "links": [
      {"rel": "self", "href": "...?$page=1&$limit=3"},
      {"rel": "next", "href": "...?$page=2&$limit=3"}
    ]
  }
}
```

**✅ Verify:**
- ✅ `$objectType` is auto-set (no manual strings!)
- ✅ `$reserved` with `$fv` present
- ✅ Pagination metadata present
- ✅ HATEOAS links (self, next, last)

#### Test 2: Get Cat by ID

```bash
curl http://localhost:9009/mock/v4/config/cat/42 | jq
```

**Expected Response:**
```json
{
  "$objectType": "mock.v4.config.Cat",
  "$reserved": {
    "$fv": "v4.r1"
  },
  "catId": 42,
  "catName": "Cat-42",
  "catType": "TYPE2",
  "description": "A fluffy cat",
  "location": {
    "$objectType": "mock.v4.config.Location",
    "city": "CatCity-42",
    "country": {
      "$objectType": "mock.v4.config.Country",
      "name": "Catland"
    }
  }
}
```

**✅ Verify:**
- ✅ Nested objects have auto-set `$objectType`
- ✅ Location, Country all have correct types

#### Test 3: Create Cat

```bash
curl -X POST http://localhost:9009/mock/v4/config/cats \
  -H "Content-Type: application/json" \
  -d '{
    "catName": "Whiskers",
    "catType": "Persian",
    "description": "A fluffy cat"
  }' | jq
```

**Expected Response:**
```json
{
  "$objectType": "mock.v4.config.Cat",
  "$reserved": {
    "$fv": "v4.r1"
  },
  "catId": 101,
  "catName": "Whiskers",
  "catType": "Persian",
  "description": "A fluffy cat"
}
```

#### Test 4: Async Operation (Task-based)

```bash
# Start async operation (returns task ID immediately)
RESPONSE=$(curl -s http://localhost:9009/mock/v4/config/cat/42/async)
echo $RESPONSE | jq

# Extract task ID
TASK_ID=$(echo $RESPONSE | jq -r '.taskId')
echo "Task ID: $TASK_ID"

# Poll task progress (on Task Server - Port 9010)
curl http://localhost:9010/tasks/$TASK_ID | jq

# Poll again after a few seconds
sleep 3
curl http://localhost:9010/tasks/$TASK_ID | jq
```

**Expected Flow:**
1. First call returns: `{"taskId": "xxx-xxx-xxx"}`
2. First poll: `{"status": "PENDING", "percentageComplete": 0}`
3. Second poll: `{"status": "IN_PROGRESS", "percentageComplete": 50}`
4. Final poll: `{"status": "COMPLETED", "percentageComplete": 100, "objectReturned": {...}}`

---

## 📊 Step 7: View Logs

```bash
# API Handler Server logs
tail -f /tmp/api-server.log

# Task Server logs
tail -f /tmp/task-server.log

# View last 50 lines
tail -50 /tmp/api-server.log
tail -50 /tmp/task-server.log
```

---

## 🛑 Step 8: Stop the Servers

### Option A: Use Stop Script

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock
./stop-servers.sh
```

### Option B: Manual Stop

```bash
# Kill by PID
kill $(cat /tmp/api-server.pid)
kill $(cat /tmp/task-server.pid)

# Or kill by process name
pkill -f api-server
pkill -f task-server

# Verify they're stopped
ps aux | grep -E "(api-server|task-server)" | grep -v grep
```

---

## 🔧 Troubleshooting

### Problem: Port Already in Use

```bash
# Error: "bind: address already in use"

# Find what's using the port
lsof -i :9009
lsof -i :9010

# Kill the process
kill -9 <PID>

# Or kill all old instances
pkill -f api-server
pkill -f task-server
```

### Problem: Import Errors

```bash
# Error: "cannot find package github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto"

# Check if DTOs are generated
ls /Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/dto/src/

# If missing, generate them:
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc
mvn clean install -s settings.xml

# Then rebuild:
cd /Users/nitin.mangotra/ntnx-api-golang-mock
go mod tidy
go build ./...
```

### Problem: Build Fails

```bash
# Clear Go cache
go clean -cache -modcache

# Re-download dependencies
cd /Users/nitin.mangotra/ntnx-api-golang-mock
rm -rf vendor/
go mod download
go mod tidy

# Try building again
go build -v ./cmd/api-server/main.go
```

### Problem: $objectType Not Auto-Set

```bash
# Check if you're using the correct service
grep "generated.NewCat" services/cat_service_with_dto.go

# Verify go.mod has correct replace directive
grep "replace.*ntnx-api-golang-mock-pc" go.mod

# Should see:
# replace github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto => ../ntnx-api-golang-mock-pc/generated-code/dto/src
```

---

## 📝 Development Workflow

### Making Changes

1. **Modify YAML** (in ntnx-api-golang-mock-pc):
   ```bash
   cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc
   # Edit: golang-mock-api-definitions/defs/.../catModel.yaml
   ```

2. **Regenerate DTOs**:
   ```bash
   mvn clean install -s settings.xml
   ```

3. **Update Service Logic** (in ntnx-api-golang-mock):
   ```bash
   cd /Users/nitin.mangotra/ntnx-api-golang-mock
   # Edit: services/cat_service_with_dto.go
   ```

4. **Rebuild and Test**:
   ```bash
   ./stop-servers.sh
   ./start-servers.sh
   ./test-grpc-gateway.sh
   ```

---

## ✅ Validation Checklist

Before considering the system ready:

- [ ] Both servers build without errors
- [ ] Both servers start successfully
- [ ] Health endpoints respond correctly
- [ ] List cats returns paginated data
- [ ] All responses have `$objectType` and `$reserved`
- [ ] Pagination links (self, next, prev, last) are correct
- [ ] Async operations return task IDs
- [ ] Task polling works on Task Server
- [ ] Logs show no errors
- [ ] Can create, read, update, delete cats
- [ ] Can stop servers cleanly

---

## 🎯 Quick Reference

| Command | Purpose |
|---------|---------|
| `./start-servers.sh` | Build + Start both servers |
| `./stop-servers.sh` | Stop both servers |
| `./test-grpc-gateway.sh` | Run comprehensive tests |
| `curl localhost:9009/health` | Check API server health |
| `curl localhost:9010/health` | Check Task server health |
| `tail -f /tmp/api-server.log` | View API server logs |
| `tail -f /tmp/task-server.log` | View Task server logs |

---

## 📞 Help

If you encounter issues:
1. Check logs: `/tmp/api-server.log` and `/tmp/task-server.log`
2. Verify prerequisites are met
3. Ensure ntnx-api-golang-mock-pc DTOs are generated
4. Check ports 9009 and 9010 are not in use

---

**🎉 You're all set! Enjoy your Nutanix v4 compliant Mock API!**


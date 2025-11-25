# Running golang-mock Server Locally

This guide shows you how to run the gRPC server on your local machine for development and testing.

## Prerequisites

1. **Go 1.21+** installed
2. **grpcurl** for testing (optional but recommended):
   ```bash
   # macOS
   brew install grpcurl
   
   # Linux
   sudo apt-get install grpcurl
   # OR download from: https://github.com/fullstorydev/grpcurl/releases
   ```

3. **Generated code** from `ntnx-api-golang-mock-pc`:
   ```bash
   cd ~/ntnx-api-golang-mock-pc
   mvn clean install -DskipTests -s settings.xml
   ./generate-grpc.sh  # Generate .pb.go files from .proto
   ```
   
   **Why is this needed?**
   
   The Go server (`ntnx-api-golang-mock`) **depends on generated code** that doesn't exist in the repository:
   
   - **Go DTOs** (Data Transfer Objects) - Generated from YAML → `generated-code/dto/src/models/`
   - **Protocol Buffer Go code** (.pb.go files) - Generated from .proto → `generated-code/protobuf/mock/v4/config/`
   
   The Go code imports these:
   ```go
   // In cat_grpc_service.go
   import (
       pb "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config"
   )
   
   // In global.go
   import (
       generated "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto/models/mock/v4/config"
   )
   ```
   
   These imports are resolved via `replace` directives in `go.mod`:
   ```go
   replace github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto => ../ntnx-api-golang-mock-pc/generated-code/dto/src
   replace github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config => ../ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config
   ```
   
   **Without the Maven build, these directories/files don't exist, so the Go code won't compile!**
   
   **When to run this:**
   - ✅ **First time** setting up the project
   - ✅ **After changing YAML** API definitions
   - ✅ **After pulling latest** changes to `ntnx-api-golang-mock-pc`
   - ❌ **Not needed** if you're just running the server and haven't changed API definitions

## 🚀 Quick Start (3 Steps)

### Step 1: Build the Server

```bash
cd ~/ntnx-api-golang-mock

# Option A: Using Make (recommended)
make build-local

# Option B: Direct Go build
go build -o golang-mock-server-local golang-mock-service/server/main.go
GOOS=linux GOARCH=amd64 go build -o golang-mock-server-local-linux golang-mock-service/server/main.go

```

### Step 2: Run the Server

**Option A: Run in Background (Recommended for Testing)**
```bash
# Run in background with logs (default: info level)
nohup ./golang-mock-server-local -port 9090 > golang-mock-server.log 2>&1 &

# Run with debug logging enabled
nohup ./golang-mock-server-local -port 9090 -log-level debug > golang-mock-server.log 2>&1 &

# Wait 2 seconds for server to start
sleep 2

# Verify it's running
ps aux | grep golang-mock-server-local | grep -v grep
```

**Option B: Run in Foreground (For Development)**
```bash
# Run on default port 9090 with info logging (keep terminal open)
./golang-mock-server-local -port 9090

# Run with debug logging enabled
./golang-mock-server-local -port 9090 -log-level debug
```

**Available Log Levels:**
- `debug` - Most verbose (shows all gRPC requests/responses, detailed service logs)
- `info` - Default (shows service operations, errors)
- `warn` - Warnings and errors only
- `error` - Errors only

**Expected Output:**
```
INFO[2025-11-25 XX:XX:XX] Starting Golang Mock Service...
INFO[2025-11-25 XX:XX:XX] Starting GRPC Server on port 9090
INFO[2025-11-25 XX:XX:XX] Registering services with the gRPC server...
INFO[2025-11-25 XX:XX:XX] 🎯 Initializing gRPC Cat Service with mock data
INFO[2025-11-25 XX:XX:XX] ✅ Initialized 100 cats in gRPC service
INFO[2025-11-25 XX:XX:XX] Registered CatService with the gRPC server
INFO[2025-11-25 XX:XX:XX] Registered reflection service
INFO[2025-11-25 XX:XX:XX] Starting Golang Mock gRPC server on :9090.
INFO[2025-11-25 XX:XX:XX] Golang Mock gRPC server listening on :9090.
```

### Step 3: Test the Server

**⚠️ Important: `-d` flag must come BEFORE the address!**

```bash
# 1. List available services
grpcurl -plaintext localhost:9090 list

# 2. List methods in CatService
grpcurl -plaintext localhost:9090 list mock.v4.config.CatService

# 3. Test ListCats (no parameters, uses defaults)
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/listCats

# 4. Test ListCats with pagination (CORRECT syntax)
grpcurl -plaintext -d '{"page": 1, "limit": 5}' localhost:9090 mock.v4.config.CatService/listCats

# 5. Test GetCat
grpcurl -plaintext -d '{"catId": 1}' localhost:9090 mock.v4.config.CatService/getCat

# 6. Test CreateCat
grpcurl -plaintext -d '{"cat": {"catName": "Fluffy", "catType": "TYPE1", "description": "A test cat"}}' localhost:9090 mock.v4.config.CatService/createCat

# 7. Test UpdateCat
grpcurl -plaintext -d '{"catId": 1, "cat": {"catName": "Updated Cat", "catType": "TYPE2", "description": "Updated description"}}' localhost:9090 mock.v4.config.CatService/updateCat

# 8. Test DeleteCat
grpcurl -plaintext -d '{"catId": 1}' localhost:9090 mock.v4.config.CatService/deleteCat
```

**✅ Correct Syntax:**
```bash
grpcurl -plaintext -d '{"page": 1, "limit": 5}' localhost:9090 mock.v4.config.CatService/listCats
#          ^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
#          flags   data (BEFORE address)        address         method
```

**❌ Wrong Syntax (will give "Too many arguments" error):**
```bash
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/listCats -d '{"page": 1, "limit": 5}'
#                                                                     ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
#                                                                     WRONG: -d after address
```

**Option 2: Using Postman**

1. **Create SSH tunnel** (if testing remote server):
   ```bash
   ssh -L 9090:localhost:9090 user@remote-server
   ```

2. **Import Proto File**:
   - Open Postman → **Import** → **Files**
   - Navigate to: `ntnx-api-golang-mock-pc/generated-code/protobuf/swagger/mock/v4/config/`
   - Import: `cat_service.proto`
   - When prompted for import paths, add:
     - `/Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/protobuf/swagger`
   - Also import dependencies:
     - `mock/v4/api_version.proto`
     - `mock/v4/http_method_options.proto`
     - `mock/v4/config/config.proto`
     - `mock/v4/error/error.proto`

3. **Create gRPC Request**:
   - Select **gRPC** request type
   - Server URL: `localhost:9090`
   - Select service: `mock.v4.config.CatService`
   - Select method: `ListCats`
   - Request body (JSON):
     ```json
     {
       "page": 1,
       "limit": 10
     }
     ```
   - Click **Invoke**

## Running in Background

### Using nohup (Recommended)

```bash
# Run in background with logs
nohup ./golang-mock-server-local -port 9090 > golang-mock-server.log 2>&1 &

# Wait for server to start (2 seconds)
sleep 2

# Check if running
ps aux | grep golang-mock-server-local | grep -v grep

# View logs
tail -f golang-mock-server.log

# Stop server
pkill -f golang-mock-server-local
```

### Quick Test Script

A test script is available that starts the server and runs tests:

```bash
cd ~/ntnx-api-golang-mock
./test-server.sh
```

This script will:
1. Start the server in the background
2. Wait for it to initialize
3. Run basic tests automatically
4. Show you how to stop it

### Using screen

```bash
# Start a screen session
screen -S golang-mock

# Run server
./golang-mock-server-local -port 9090

# Detach: Press Ctrl+A, then D
# Reattach: screen -r golang-mock
# Kill: screen -X -S golang-mock quit
```

### Using tmux

```bash
# Start a tmux session
tmux new -s golang-mock

# Run server
./golang-mock-server-local -port 9090

# Detach: Press Ctrl+B, then D
# Reattach: tmux attach -t golang-mock
# Kill: tmux kill-session -t golang-mock
```

## Development Workflow

### 1. Make Code Changes

Edit files in `golang-mock-service/` directory.

### 2. Rebuild and Restart

```bash
# Stop existing server
pkill -f golang-mock-server-local

# Rebuild
go build -o golang-mock-server-local golang-mock-service/server/main.go

# Start again
./golang-mock-server-local -port 9090
```

### 3. Test Changes

```bash
# Quick test
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/listCats
```

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 9090
lsof -i :9090
# OR
netstat -an | grep 9090

# Kill the process or use a different port
./golang-mock-server-local -port 9091
```

### Import Errors

If you see import errors:
```bash
# Ensure generated code exists
ls -la ~/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config/

# Rebuild generated code
cd ~/ntnx-api-golang-mock-pc
mvn clean install -DskipTests -s settings.xml

# Clean and rebuild
cd ~/ntnx-api-golang-mock
go clean -cache
go mod tidy
go build -o golang-mock-server-local golang-mock-service/server/main.go
```

### Server Not Starting

```bash
# Check for errors
./golang-mock-server-local -port 9090 2>&1 | tee server.log

# Verify dependencies
go mod verify

# Check Go version
go version  # Should be 1.21+
```

## Expected Responses

### ListCats Response

```json
{
  "cats": [
    {
      "catId": 1,
      "catName": "Cat-1",
      "catType": "TYPE1",
      "description": "A fluffy cat"
    },
    {
      "catId": 2,
      "catName": "Cat-2",
      "catType": "TYPE1",
      "description": "A fluffy cat",
      "location": {
        "city": "San Francisco",
        "country": {
          "state": "California"
        }
      }
    }
  ],
  "totalCount": 100,
  "page": 1,
  "limit": 10
}
```

### GetCat Response

```json
{
  "cat": {
    "catId": 1,
    "catName": "Cat-1",
    "catType": "TYPE1",
    "description": "A fluffy cat"
  }
}
```

## Next Steps

Once the server is running locally:

1. **Test all endpoints** using grpcurl or Postman
2. **Make code changes** and rebuild
3. **Test changes** before deploying to PC
4. **Deploy to PC** following `SETUP_GOLANG_MOCK_IN_PC.md`

## Differences: Local vs PC

| Aspect | Local | PC |
|--------|-------|-----|
| **Port** | Any (default: 9090) | 9090 (or configured) |
| **Binary** | `golang-mock-server-local` | `golang-mock-server` (Linux) |
| **Build** | `go build` (native) | `GOOS=linux GOARCH=amd64 go build` |
| **Testing** | Direct gRPC | Via Adonis/Mercury (REST) |
| **Logs** | Console/stdout | File: `~/golang-mock-build/golang-mock-server.log` |

---

## Complete Working Example

Here's a complete example that works:

```bash
# Terminal 1: Build and start server
cd ~/ntnx-api-golang-mock
make build-local
nohup ./golang-mock-server-local -port 9090 > golang-mock-server.log 2>&1 &
sleep 2

# Terminal 2: Test the server
# List services
grpcurl -plaintext localhost:9090 list

# List CatService methods
grpcurl -plaintext localhost:9090 list mock.v4.config.CatService

# Test ListCats (default)
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/listCats

# Test ListCats with pagination (CORRECT: -d before address)
grpcurl -plaintext -d '{"page": 1, "limit": 5}' localhost:9090 mock.v4.config.CatService/listCats

# Test GetCat
grpcurl -plaintext -d '{"catId": 1}' localhost:9090 mock.v4.config.CatService/getCat

# Stop server when done
pkill -f golang-mock-server-local
```

## Common Issues

### Issue: "Too many arguments" error

**Problem:** The `-d` flag is placed after the address.

**Wrong:**
```bash
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/listCats -d '{"page": 1}'
```

**Correct:**
```bash
grpcurl -plaintext -d '{"page": 1}' localhost:9090 mock.v4.config.CatService/listCats
```

### Issue: "Connection refused"

**Problem:** Server is not running.

**Solution:**
```bash
# Check if server is running
ps aux | grep golang-mock-server-local | grep -v grep

# If not running, start it
nohup ./golang-mock-server-local -port 9090 > golang-mock-server.log 2>&1 &
sleep 2
```

### Issue: Port already in use

**Problem:** Port 9090 is already occupied.

**Solution:**
```bash
# Find what's using port 9090
lsof -i :9090

# Kill it or use a different port
./golang-mock-server-local -port 9091
```

---

**Last Updated**: 2025-11-25  
**Quick Command**: `make build-local && nohup ./golang-mock-server-local -port 9090 > golang-mock-server.log 2>&1 &`


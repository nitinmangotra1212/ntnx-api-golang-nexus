# Debug Logging in golang-mock Server

## 🎯 Overview

The golang-mock server supports configurable log levels for debugging and troubleshooting.

## 📋 Available Log Levels

| Level | Description | Use Case |
|-------|-------------|----------|
| `debug` | Most verbose - shows all gRPC requests/responses, detailed service logs | Troubleshooting, development |
| `info` | Default - shows service operations, errors | Normal operation |
| `warn` | Warnings and errors only | Production monitoring |
| `error` | Errors only | Critical issues only |

## 🚀 How to Enable Debug Logging

### Local Development

```bash
# Build server
cd ~/ntnx-api-golang-mock
make build-local

# Run with debug logging
./golang-mock-server-local -port 9090 -log-level debug

# OR run in background
nohup ./golang-mock-server-local -port 9090 -log-level debug > golang-mock-server.log 2>&1 &
```

### PC Deployment

```bash
# SSH to PC
ssh nutanix@<PC_IP>

# Start with debug logging
cd ~/golang-mock-build
nohup ./golang-mock-server -port 9090 -log-level debug > ~/golang-mock-build/golang-mock-server.log 2>&1 &

# View debug logs
tail -f ~/golang-mock-build/golang-mock-server.log
```

## 📊 What Debug Logs Show

### 1. gRPC Request/Response Logging

When debug logging is enabled, you'll see detailed gRPC request and response information:

```
DEBU[2025-11-25 14:32:46] gRPC Request: Method=mock.v4.config.CatService/listCats, Request=&{Page:1 Limit:5}
DEBU[2025-11-25 14:32:46] gRPC: ListCats request details: &{Page:1 Limit:5}
DEBU[2025-11-25 14:32:46] gRPC: Total cats in memory: 100
DEBU[2025-11-25 14:32:46] gRPC: Using pagination: page=1, limit=5
DEBU[2025-11-25 14:32:46] gRPC: Pagination calculated - startIdx=0, endIdx=5, totalCount=100
INFO[2025-11-25 14:32:46] ✅ gRPC: Returning 5 cats (page 1, limit 5)
DEBU[2025-11-25 14:32:46] gRPC: Returning cats: [&{CatId:1 CatName:Cat-1 ...} ...]
DEBU[2025-11-25 14:32:46] gRPC Response: Method=mock.v4.config.CatService/listCats, Response=&{Cats:[...] TotalCount:100 Page:1 Limit:5}
```

### 2. Service Operation Details

Debug logs show:
- Request parameters in detail
- Internal state (e.g., total cats in memory)
- Pagination calculations
- Response data (when debug is enabled)

### 3. gRPC Stream Logging

For streaming RPCs, debug logs show:
- Stream start/end
- Stream type (client-stream, server-stream, bidirectional)
- Stream errors

## 🔍 Example Debug Output

### ListCats Request (Debug Level)

```
DEBU[2025-11-25 14:32:46] gRPC Request: Method=mock.v4.config.CatService/listCats, Request=&{Page:1 Limit:5}
DEBU[2025-11-25 14:32:46] gRPC: ListCats request details: &{Page:1 Limit:5}
DEBU[2025-11-25 14:32:46] gRPC: Total cats in memory: 100
DEBU[2025-11-25 14:32:46] gRPC: Using pagination: page=1, limit=5
DEBU[2025-11-25 14:32:46] gRPC: Pagination calculated - startIdx=0, endIdx=5, totalCount=100
INFO[2025-11-25 14:32:46] ✅ gRPC: Returning 5 cats (page 1, limit 5)
DEBU[2025-11-25 14:32:46] gRPC: Returning cats: [&{CatId:1 CatName:Cat-1 CatType:TYPE1 Description:A fluffy cat Location:<nil>} ...]
DEBU[2025-11-25 14:32:46] gRPC Response: Method=mock.v4.config.CatService/listCats, Response=&{Cats:[...] TotalCount:100 Page:1 Limit:5}
```

### GetCat Request (Debug Level)

```
DEBU[2025-11-25 14:32:50] gRPC Request: Method=mock.v4.config.CatService/getCat, Request=&{CatId:1}
INFO[2025-11-25 14:32:50] gRPC: GetCat called (catId=1)
DEBU[2025-11-25 14:32:50] gRPC: Total cats in memory: 100
INFO[2025-11-25 14:32:50] ✅ gRPC: Returning cat 1
DEBU[2025-11-25 14:32:50] gRPC Response: Method=mock.v4.config.CatService/getCat, Response=&{Cat:&{CatId:1 CatName:Cat-1 ...}}
```

## 🛠️ Command-Line Options

```bash
# Show help
./golang-mock-server-local -help

# Available flags:
#   -port int        The server port (default: 9090)
#   -log-level string Log level: debug, info, warn, error (default: "info")
```

## 📝 Logging Interceptors

The server includes gRPC interceptors that log:
- **Request details**: Method name, request payload
- **Response details**: Method name, response payload
- **Errors**: Error details with stack traces (if available)

These interceptors are automatically enabled when debug logging is active.

## 🔧 Troubleshooting with Debug Logs

### Issue: Request not reaching server

**Enable debug logging and check:**
```bash
# Start with debug
./golang-mock-server-local -port 9090 -log-level debug

# Make a request
grpcurl -plaintext -d '{"page": 1}' localhost:9090 mock.v4.config.CatService/listCats

# Check logs for:
# - "gRPC Request: Method=..." (confirms request received)
# - "gRPC: ListCats called..." (confirms handler invoked)
```

### Issue: Response not matching expected format

**Enable debug logging to see:**
- Exact request received
- Exact response sent
- Any transformations applied

### Issue: Performance problems

**Debug logs show:**
- Request/response timing (if added)
- Internal state information
- Pagination calculations

## ⚠️ Performance Considerations

**Debug logging is verbose and can impact performance:**
- ✅ Use for **development** and **troubleshooting**
- ⚠️ Use **info level** for **production**
- ❌ Don't use **debug level** in production (too verbose)

## 📋 Quick Reference

```bash
# Local: Run with debug
./golang-mock-server-local -port 9090 -log-level debug

# PC: Run with debug
nohup ./golang-mock-server -port 9090 -log-level debug > golang-mock-server.log 2>&1 &

# View logs
tail -f golang-mock-server.log | grep -E "DEBU|INFO|ERROR"

# Filter debug logs only
tail -f golang-mock-server.log | grep "DEBU"
```

---

**Last Updated**: 2025-11-25  
**Default Log Level**: `info`  
**Debug Flag**: `-log-level debug`


# Getting Started with ntnx-api-golang-mock

This guide will help you get the Mock REST API server up and running in minutes!

## 📋 Prerequisites

Before you begin, ensure you have:

- **Go 1.21 or higher** installed
  - Check: `go version`
  - Install from: https://golang.org/dl/

## 🚀 Quick Start (3 Steps)

### 1. Navigate to Project Directory

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Run the Server

**Option A: Using the start script (recommended)**
```bash
chmod +x start.sh
./start.sh
```

**Option B: Using Make**
```bash
make run
```

**Option C: Using Go directly**
```bash
go run cmd/server/main.go
```

**Option D: Build and run**
```bash
go build -o mock-api-server ./cmd/server
./mock-api-server
```

The server will start on **http://localhost:9009** 🎉

## ✅ Verify Installation

Open a new terminal and test the health endpoint:

```bash
curl http://localhost:9009/health
```

Expected response:
```json
{
  "status": "UP",
  "service": "ntnx-api-golang-mock"
}
```

## 🧪 Try Your First API Call

### List Cats
```bash
curl http://localhost:9009/mock/v4/config/cats
```

### Create a Cat
```bash
curl -X POST http://localhost:9009/mock/v4/config/cats \
  -H "Content-Type: application/json" \
  -d '{
    "catName": "Fluffy",
    "catType": "TYPE1",
    "description": "My first test cat"
  }'
```

### Get Cat by ID
```bash
curl http://localhost:9009/mock/v4/config/cat/5
```

## 🎯 Common Use Cases

### 1. Testing with Delays
Simulate slow responses for timeout testing:
```bash
curl "http://localhost:9009/mock/v4/config/cats?delay=3000"
```

### 2. Load Testing with Large Responses
Generate large responses for performance testing:
```bash
curl "http://localhost:9009/mock/v4/config/cats?size=huge&\$limit=100"
```

### 3. Pagination
Test pagination logic:
```bash
curl "http://localhost:9009/mock/v4/config/cats?\$page=2&\$limit=10"
```

### 4. Task Status Tracking
Test UUID-based task tracking:
```bash
curl -H "NTNX-Request-ID: $(uuidgen)" \
     -H "USER-Request-ID: $(uuidgen)" \
     http://localhost:9009/mock/v4/config/cat/status
```

## ⚙️ Configuration

The default configuration works out of the box. To customize:

1. Edit `configs/config.yaml`
2. Restart the server

### Change Port

Edit `configs/config.yaml`:
```yaml
server:
  port: 8080  # Change to desired port
```

Or use environment variable:
```bash
export SERVER_PORT=8080
./mock-api-server
```

## 🔧 Development Mode

For development with hot-reload:

1. Install Air:
```bash
go install github.com/cosmtrek/air@latest
```

2. Run with hot-reload:
```bash
make dev
```

Or:
```bash
air
```

Now your changes will automatically restart the server!

## 🐳 Docker

### Build and Run with Docker

```bash
# Build image
docker build -t ntnx-api-golang-mock .

# Run container
docker run -p 9009:9009 ntnx-api-golang-mock
```

### Using Docker Compose (if available)

```bash
docker-compose up
```

## 📚 Next Steps

- **View all endpoints**: Check `README.md` for complete API documentation
- **OpenAPI spec**: See `golang-mock-api-definitions/openapi.yaml`
- **Run tests**: `go test ./...`
- **Build for production**: `make build-prod`

## 🆘 Troubleshooting

### Port Already in Use
If port 9009 is already in use:
```bash
# Change port in config
export SERVER_PORT=8080
./mock-api-server
```

### Dependencies Not Found
```bash
go mod download
go mod tidy
```

### Build Errors
```bash
# Clean and rebuild
make clean
make build
```

### SFTP File Operations Not Working
- Check SFTP server configuration in `configs/config.yaml`
- Ensure network connectivity to SFTP server
- File operations will fail gracefully if SFTP is not configured

## 💡 Tips

1. **Use Make commands** for common tasks:
   - `make help` - See all available commands
   - `make test` - Run tests
   - `make fmt` - Format code
   - `make build` - Build binary

2. **Check logs** for debugging:
   - All requests are logged with details
   - Errors include stack traces

3. **API is compatible** with the Java version:
   - Same endpoints
   - Same request/response formats
   - Can replace Java service without client changes

## 🎉 Success!

You now have a fully functional Mock REST API server running!

For more details, see the main [README.md](README.md).

---

**Need help?** Open an issue or contact the team!

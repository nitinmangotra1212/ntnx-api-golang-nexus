# ntnx-api-golang-mock

Mock REST API service built with Go, following Nutanix v4 API standards with gRPC Gateway architecture.

## 🏗️ Architecture

**Two-Server Pattern (gRPC Gateway):**
- **API Handler Server** (Port 9009): Handles REST API requests
- **Task Server** (Port 9010): Manages asynchronous task processing

```
ntnx-api-golang-mock/
├── cmd/
│   ├── api-server/main.go         # API Handler Server
│   └── task-server/main.go        # Task Server
├── services/
│   └── cat_service_with_dto.go    # Business logic using auto-generated DTOs
├── routes/
│   └── routes.go                  # gorilla/mux routing
├── interfaces/
│   └── apis/mock/v4/config/       # Interface definitions
├── global/
│   └── global.go                  # Global state management
├── go.mod                         # Uses generated DTOs from ntnx-api-golang-mock-pc
└── configs/
    └── config.yaml                # Configuration
```

## ✨ Features

- ✅ **Auto-generated DTOs** from YAML (no manual $objectType strings)
- ✅ **Nutanix v4 Compliance** ($objectType, $reserved, flags, links)
- ✅ **Pagination** ($page, $limit, HATEOAS links)
- ✅ **Async Operations** (Task-based workflow)
- ✅ **Full CRUD** for Cat entities
- ✅ **OData Query Parameters** ($filter, $orderby, $select)

## 🚀 Quick Start

### Prerequisites
- **Go 1.21+**
- **ntnx-api-golang-mock-pc** (for generated DTOs)

### Build & Run

```bash
# Start both servers
./start-servers.sh

# Or manually:
go build -o bin/api-server ./cmd/api-server/main.go
go build -o bin/task-server ./cmd/task-server/main.go
./bin/api-server &    # Port 9009
./bin/task-server &   # Port 9010
```

### Test

```bash
# Run test script
./test-grpc-gateway.sh

# Or manually:
curl http://localhost:9009/mock/v4/config/cats
curl http://localhost:9009/mock/v4/config/cats/1
```

## 📊 API Endpoints

### Synchronous (API Handler - Port 9009)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/mock/v4/config/cats` | List cats (paginated) |
| GET | `/mock/v4/config/cats/{id}` | Get cat by ID |
| POST | `/mock/v4/config/cats` | Create cat |
| PUT | `/mock/v4/config/cats/{id}` | Update cat |
| DELETE | `/mock/v4/config/cats/{id}` | Delete cat |

### Asynchronous (Task Server - Port 9010)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/mock/v4/config/cats/{id}/_process` | Start async processing |
| GET | `/tasks/{taskId}` | Poll task status |

## 🔗 Related Repositories

- **API Definitions:** [ntnx-api-golang-mock-pc](../ntnx-api-golang-mock-pc) - YAML, Proto, DTO generation

## 📝 Configuration

Edit `configs/config.yaml`:
```yaml
server:
  port: 9009
```

## 📞 Contact

nitin.mangotra@nutanix.com

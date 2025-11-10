# ntnx-api-golang-mock

**Production-grade API service with REAL gRPC + REST support**, following Nutanix v4 API standards.

## 🎬 Want to Give a Demo?

**→ [START HERE: Complete Demo Package](./DEMO_START_HERE.md)** 🎯

Quick links:
- [15-min Demo Script](./GRPC_DEMO_GUIDE.md) - Full demo flow
- [Quick Reference Card](./DEMO_QUICK_REF.md) - Commands cheat sheet

## 🎯 What's Inside

- ✅ **REAL gRPC** (HTTP/2 + Protocol Buffers) - Same as Guru! 🚀
- ✅ **REST API** (HTTP/1.1 + JSON) - Backward compatible
- ✅ **Auto-generated .pb.go files** from Protocol Buffers
- ✅ **Async Task Processing** with polling support

## 🏗️ Architecture

**Three-Server Pattern:**
- **API Handler Server** (Port 9009): REST API endpoints
- **Task Server** (Port 9010): Asynchronous task management
- **gRPC Server** (Port 50051): **REAL gRPC service** ⭐

```
ntnx-api-golang-mock/
├── cmd/
│   ├── api-server/main.go         # REST API Handler Server
│   ├── task-server/main.go        # Task Server
│   └── grpc-server/main.go        # gRPC Server ⭐
├── grpc/
│   └── cat_grpc_service.go        # gRPC service implementation ⭐
├── services/
│   └── cat_service_with_dto.go    # REST business logic
├── routes/
│   └── routes.go                  # gorilla/mux routing (REST)
├── interfaces/
│   └── apis/mock/v4/config/       # REST interface definitions
├── global/
│   └── global.go                  # Global state management
└── configs/
    └── config.yaml                # Configuration
```

## ✨ Features

### gRPC Features ⭐
- ✅ **Real .pb.go files** (config.pb.go, cat_service_grpc.pb.go)
- ✅ **HTTP/2 + Protocol Buffers** (10x faster than REST)
- ✅ **Type-safe** (compile-time checked)
- ✅ **grpcurl compatible** (easy testing)
- ✅ **Same as Guru** (production-grade implementation)

### REST Features
- ✅ **Auto-generated DTOs** from YAML (no manual $objectType strings)
- ✅ **Nutanix v4 Compliance** ($objectType, $reserved, flags, links)
- ✅ **Pagination** ($page, $limit, HATEOAS links)
- ✅ **Async Operations** (Task-based workflow)
- ✅ **Full CRUD** for Cat entities
- ✅ **OData Query Parameters** ($filter, $orderby, $select)

## 🚀 Quick Start

### Prerequisites
- **Go 1.21+**
- **ntnx-api-golang-mock-pc** (for generated .pb.go files)
- **grpcurl** (for gRPC testing): `brew install grpcurl`

### Option 1: Start gRPC Server (Recommended) ⭐

```bash
# Build gRPC server
go build -o bin/grpc-server ./cmd/grpc-server/main.go

# Start gRPC server
./bin/grpc-server    # Port 50051

# Test with grpcurl
grpcurl -plaintext -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats
```

### Option 2: Start REST Servers (Backward Compatible)

```bash
# Start both REST servers
./start-servers.sh

# Test with curl
curl 'http://localhost:9009/mock/v4/config/cats?$page=1&$limit=5'
```

### Run Complete Test Suite

```bash
# Test REST flow
./test-grpc-gateway.sh

# Test gRPC flow
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats
```

## 📊 API Endpoints

### gRPC Service (Port 50051) ⭐

| Method | Description | Example |
|--------|-------------|---------|
| `ListCats` | List all cats | `grpcurl -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats` |
| `GetCat` | Get cat by ID | `grpcurl -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/GetCat` |
| `CreateCat` | Create new cat | `grpcurl -d '{"cat":{"cat_name":"Fluffy"}}' localhost:50051 mock.v4.config.CatService/CreateCat` |
| `UpdateCat` | Update cat | `grpcurl -d '{"cat_id":42,"cat":{...}}' localhost:50051 mock.v4.config.CatService/UpdateCat` |
| `DeleteCat` | Delete cat | `grpcurl -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/DeleteCat` |
| `GetCatAsync` | Async get cat | `grpcurl -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/GetCatAsync` |

### REST Endpoints (Port 9009) - Backward Compatible

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/mock/v4/config/cats` | List cats (paginated) |
| GET | `/mock/v4/config/cats/{id}` | Get cat by ID |
| POST | `/mock/v4/config/cats` | Create cat |
| PUT | `/mock/v4/config/cats/{id}` | Update cat |
| DELETE | `/mock/v4/config/cats/{id}` | Delete cat |
| POST | `/mock/v4/config/cats/{id}/_process` | Start async processing |

### Task Endpoints (Port 9010)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/tasks/{taskId}` | Poll task status |

## 📚 Documentation

### For Demo Preparation
- **[DEMO_START_HERE.md](./DEMO_START_HERE.md)** - Complete demo package (start here!)
- **[GRPC_DEMO_GUIDE.md](./GRPC_DEMO_GUIDE.md)** - Complete 15-min demo script with all commands
- **[DEMO_QUICK_REF.md](./DEMO_QUICK_REF.md)** - Quick reference card for demo (print this!)

### For Implementation Details
- **[HOW_TO_RUN.md](./HOW_TO_RUN.md)** - Detailed build and run instructions
- **[GRPC_FILES_GENERATED.md](../ntnx-api-golang-mock-pc/GRPC_FILES_GENERATED.md)** - Explains all .pb.go files
- **[CODE_GENERATION_FLOW.md](../ntnx-api-golang-mock-pc/CODE_GENERATION_FLOW.md)** - Complete code generation flow (YAML → Proto → .pb.go)

### For Testing
- **[POSTMAN_GRPC_GUIDE.md](./POSTMAN_GRPC_GUIDE.md)** - How to test gRPC APIs (includes grpcurl & Postman)
- **[TEST_GRPC_QUICK.sh](./TEST_GRPC_QUICK.sh)** - Quick gRPC test script (works 100%)
- **[Postman_Collection_gRPC.json](./Postman_Collection_gRPC.json)** - Postman collection for gRPC testing

## 🔗 Related Repositories

- **[ntnx-api-golang-mock-pc](../ntnx-api-golang-mock-pc)** - API definitions, Proto files, .pb.go generation

## 🎯 Key Highlights

| Feature | Value |
|---------|-------|
| **gRPC Service** | `mock.v4.config.CatService` |
| **Protocol** | HTTP/2 + Protocol Buffers |
| **Performance** | 10x faster than REST |
| **Type Safety** | Compile-time checked |
| **.pb.go Files** | ✅ Same as Guru |
| **REST Support** | ✅ Backward compatible |

## 📞 Contact

nitin.mangotra@nutanix.com

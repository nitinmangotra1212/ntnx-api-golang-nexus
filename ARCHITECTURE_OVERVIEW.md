# 🏗️ Complete Architecture Overview

**Generated:** 2025-01-20  
**Purpose:** Comprehensive understanding of the entire codebase architecture

---

## 📚 Repository Structure

### 1. **ntnx-api-golang-mock** (Go Service Implementation)

**Purpose:** Production-grade Go gRPC service implementing the Mock Cat API

**Key Files:**

```
ntnx-api-golang-mock/
├── cmd/
│   ├── grpc-server/main.go        # gRPC server (Port 50051) ⭐ PRIMARY
│   ├── api-server/main.go         # REST API Handler (Port 9009)
│   └── task-server/main.go        # Task Server (Port 9010)
│
├── grpc/
│   └── cat_grpc_service.go        # gRPC service implementation
│       ├── CatGrpcService struct
│       ├── ListCats()  - implements pb.CatServiceServer
│       ├── GetCat()
│       ├── CreateCat()
│       ├── UpdateCat()
│       ├── DeleteCat()
│       └── GetCatAsync()
│
├── services/
│   └── cat_service_with_dto.go    # REST business logic (uses DTOs)
│
├── routes/
│   └── routes.go                  # gorilla/mux REST routing
│
├── interfaces/
│   └── apis/mock/v4/config/
│       └── cat_endpoints.go       # REST endpoint definitions
│
├── global/
│   └── global.go                  # Global state (task storage)
│
├── configs/
│   └── config.yaml                # Server configuration
│
└── go.mod                         # Go dependencies
    ├── Imports: github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto
    └── Imports: github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config
```

**What it does:**
- Implements gRPC server using auto-generated `.pb.go` files
- Implements REST server using auto-generated Go DTOs
- Handles CRUD operations for Cat entities
- Supports async task processing
- In-memory data storage (100 mock cats)

---

### 2. **ntnx-api-golang-mock-pc** (API Definitions & Code Generation)

**Purpose:** YAML API definitions + Maven code generation pipeline

**Key Files:**

```
ntnx-api-golang-mock-pc/
├── pom.xml                        # Maven parent POM
│
├── golang-mock-api-definitions/
│   ├── pom.xml                    # Processes YAML → OpenAPI
│   └── defs/
│       └── namespaces/mock/versioned/v4/modules/config/released/
│           ├── models/
│           │   └── catModel.yaml  # Cat, Location, Country schemas
│           └── api/
│               └── catEndpoint.yaml  # REST endpoints (GET/POST/PUT/DELETE)
│
├── golang-mock-api-codegen/       # Code generation modules
│   ├── pom.xml                    # Parent codegen POM
│   │
│   ├── golang-mock-protobuf-messages/
│   │   └── Generates: .proto message files from YAML
│   │
│   ├── golang-mock-java-dto-definitions/
│   │   └── Generates: Java DTOs (Cat.java, Location.java)
│   │
│   ├── golang-mock-proto-to-java/
│   │   └── Generates: Java classes from .proto files
│   │
│   ├── golang-mock-protobuf-mappers/
│   │   └── Generates: MapStruct mappers (Java DTO ↔ Proto)
│   │
│   ├── golang-mock-springmvc-interfaces/
│   │   └── Generates: Spring MVC controller interfaces
│   │
│   └── golang-mock-grpc-client/
│       ├── pom.xml
│       ├── src/main/proto/mock/v4/config/
│       │   ├── config.proto       # Proto message definitions
│       │   └── cat_service.proto  # gRPC service definition
│       └── Generates: Java gRPC client stubs
│
└── generated-code/
    ├── dto/src/models/mock/v4/config/
    │   └── config_model.go        # Go DTOs (NewCat(), NewLocation())
    │
    └── protobuf/
        ├── swagger/mock/v4/config/
        │   ├── config.proto       # Generated proto messages
        │   └── cat_service.proto  # Manually created gRPC service
        │
        └── mock/v4/config/
            ├── config.pb.go       # Protobuf message implementations
            ├── cat_service.pb.go  # Service message implementations
            └── cat_service_grpc.pb.go  # gRPC stubs ⭐
```

**What it does:**
- Defines API schema in YAML (OpenAPI)
- Generates Go DTOs with auto-set `$objectType` and `$reserved`
- Generates `.proto` files from YAML
- Generates `.pb.go` files from `.proto` (via `generate-grpc.sh`)
- Generates Java DTOs, mappers, and gRPC client for Adonis

**Build Commands:**
```bash
# Generate all Java code
mvn clean install -s settings.xml

# Generate Go protobuf code
./generate-grpc.sh
```

---

### 3. **ntnx-api-prism-service** (Adonis - REST Gateway)

**Purpose:** Spring Boot gateway that routes REST → gRPC

**Key Files:**

```
ntnx-api-prism-service/
├── pom.xml
│   ├── Dependencies on ALL API controllers
│   ├── guru-pc-grpc-client (Domain Manager)
│   └── golang-mock-grpc-client (Mock Service) ⭐
│
└── src/main/java/com/nutanix/
    ├── api/restserver/
    │   ├── main/PrismService.java        # Spring Boot main
    │   ├── config/                       # Spring configuration
    │   ├── filters/                      # Request filters
    │   ├── handlers/                     # Request handlers
    │   └── interceptors/                 # gRPC interceptors
    │
    └── mock/                             # Mock service integration ⭐
        ├── client/                       # gRPC client wrapper
        ├── controller/                   # Auto-generated controllers
        └── config/                       # Mock service config
```

**What it does:**
- Runs as Spring Boot application (Port 8888 internally, 9440 externally via Mercury)
- Receives REST/JSON requests
- Auto-generated controllers convert JSON → Java DTO
- Auto-generated mappers convert Java DTO → Proto
- gRPC client calls Go service
- Reverse process for responses (Proto → Java DTO → JSON)
- Includes ALL Nutanix v4 API controllers (Guru, VMM, Networking, etc.)

---

### 4. **ntnx-api-guru** (Reference Implementation)

**Purpose:** Production Go gRPC service for Domain Manager (PC management)

**Key Files:**

```
ntnx-api-guru/
├── guru-api-service/
│   ├── grpc/
│   │   └── grpc_server.go            # Real gRPC server implementation
│   ├── services/
│   │   ├── domain_manager/           # Domain Manager service
│   │   └── domain_manager_config/    # Config service
│   ├── background/                   # Background jobs
│   ├── poller/                       # Polling mechanisms
│   └── models/                       # Data models
│
└── go.mod
```

**What it does:**
- Production gRPC service running on PC
- Implements Domain Manager APIs
- Pattern that Mock Service follows
- Uses same `.pb.go` generation approach

---

### 5. **ntnx-api-guru-pc** (Guru Code Generation)

**Purpose:** Code generation for Guru service (same pattern as Mock)

**Key Files:**

```
ntnx-api-guru-pc/
├── guru-pc-api-definitions/          # YAML API definitions
├── guru-pc-api-codegen/              # Code generators
│   ├── guru-pc-go-dto-definitions/
│   ├── guru-pc-java-dto-definitions/
│   ├── guru-pc-protobuf-mappers/
│   └── guru-pc-grpc-client/
└── generated-code/
    ├── dto/
    ├── edm/
    └── protobuf/
```

**What it does:**
- Same pattern as `ntnx-api-golang-mock-pc`
- Generates code for Guru service
- Reference for how production services are structured

---

## 🔄 Complete Data Flow

### REST Request Flow (Client → Adonis → Go gRPC Service)

```
1. Client sends REST/JSON request
   ↓
   curl -k https://10.112.90.239/api/mock/v4.0.a1/config/cats

2. Mercury (Nginx on Port 9440)
   ↓ Routes to Adonis

3. Adonis (Spring Boot on Port 8888)
   ├─ Auto-generated Spring MVC Controller receives request
   ├─ Jackson deserializes JSON → Java DTO (Cat.java)
   ├─ Auto-generated Mapper: Java DTO → Proto (CatProto)
   ├─ gRPC client stub created from .proto
   ↓ gRPC call (HTTP/2 + Protobuf) on Port 50051

4. Go gRPC Service (ntnx-api-golang-mock)
   ├─ grpc-server listening on Port 50051
   ├─ CatGrpcService.ListCats() receives ListCatsRequest proto
   ├─ Business logic executes
   ├─ Returns ListCatsResponse proto
   ↓ gRPC response

5. Adonis
   ├─ Auto-generated Mapper: Proto → Java DTO
   ├─ Jackson serializes Java DTO → JSON
   ├─ Adds Nutanix v4 fields ($objectType, $reserved, metadata, links)
   ↓ HTTP/JSON response

6. Client receives JSON response
   {
     "data": [
       {
         "$objectType": "mock.v4.config.Cat",
         "$reserved": {...},
         "catId": 5,
         "catName": "Cat-5",
         ...
       }
     ]
   }
```

### Direct gRPC Flow (Client → Go gRPC Service)

```
1. gRPC client (grpcurl)
   ↓
   grpcurl -plaintext 10.112.90.239:50051 mock.v4.config.CatService/ListCats

2. Go gRPC Service (Port 50051)
   ├─ CatGrpcService.ListCats() receives ListCatsRequest
   ├─ Business logic executes
   ├─ Returns ListCatsResponse (pure proto, no $objectType wrapper)
   ↓

3. Client receives protobuf response
   {
     "cats": [
       {
         "catId": 5,
         "catName": "Cat-5",
         ...
       }
     ],
     "totalCount": 100
   }
```

---

## 🛠️ Code Generation Pipeline

### YAML → Proto → Go Code

```
Step 1: YAML API Definition (catModel.yaml)
   ↓
   Maven Plugin: dev-platform-maven-plugins
   ↓
Step 2: OpenAPI Spec (swagger-all-*.yaml)
   ↓
   Maven Plugin: ProtoMessageGenerator
   ↓
Step 3: Proto Message Files (config.proto)
   ↓
   Manual: Create cat_service.proto (service definition)
   ↓
Step 4: Generate Go Code (./generate-grpc.sh)
   ↓
   protoc + protoc-gen-go + protoc-gen-go-grpc
   ↓
Step 5: .pb.go Files
   ├─ config.pb.go (11KB) - Message implementations
   ├─ cat_service.pb.go (35KB) - Service messages
   └─ cat_service_grpc.pb.go (19KB) - gRPC stubs ⭐
       ├─ CatServiceClient interface
       ├─ CatServiceServer interface (YOU IMPLEMENT THIS)
       └─ RegisterCatServiceServer() function
```

### YAML → Java Code (for Adonis)

```
Step 1: YAML API Definition
   ↓
Step 2: Maven Module Pipeline
   ↓
   ├─ golang-mock-java-dto-definitions
   │  └─ JavaDtoGenerator → Cat.java, Location.java
   │
   ├─ golang-mock-proto-to-java
   │  └─ protobuf-maven-plugin → Java proto classes
   │
   ├─ golang-mock-protobuf-mappers
   │  └─ MapstructMapperGenerator → CatMapper.java
   │
   ├─ golang-mock-springmvc-interfaces
   │  └─ SpringMvcApiInterfaceGenerator → CatApi.java (interface)
   │
   └─ golang-mock-grpc-client
      └─ GrpcClientGenerator → CatApiController.java (implementation)
         ├─ Receives REST request
         ├─ Uses CatMapper to convert DTO → Proto
         ├─ Calls Go gRPC service
         └─ Returns REST response
```

---

## 🎯 Key Mechanisms Confirmed

### 1. Auto-Generation

✅ **Spring MVC Controllers** - Generated by `GrpcClientGenerator` from YAML
- `CatApiController.java` implements `CatApi` interface
- Handles REST endpoints
- Calls gRPC service

✅ **Java DTOs** - Generated by `JavaDtoGenerator` from YAML
- `Cat.java`, `Location.java`, `Country.java`
- Used for JSON serialization/deserialization

✅ **Proto Classes** - Generated by `protobuf-maven-plugin` from `.proto`
- `ConfigProto.java`, `CatServiceGrpc.java`
- Used for gRPC communication

✅ **Mappers** - Generated by `MapstructMapperGenerator`
- `CatMapper.java` converts Java DTO ↔ Proto
- Auto-handles nested objects

✅ **gRPC Client Stubs** - Generated by protoc
- `CatServiceGrpc.CatServiceBlockingStub`
- Used by Adonis to call Go service

### 2. Service Discovery/Routing

✅ **Simple Configuration-Based**
- Adonis connects to Go service via configured host:port
- Default: `localhost:50051` (or configured endpoint)
- No complex service registry needed for this setup
- Production: Mercury routes `/api/mock/v4.0.a1/*` → Adonis → gRPC service

### 3. Translation Process

✅ **Automatic Conversion**
```
REST/JSON → Java DTO → Proto → Go gRPC Service
                 ↓               ↓
          (auto-generated)  (your code)
```

**What's auto-generated:**
- JSON ↔ Java DTO conversion (Spring MVC)
- Java DTO ↔ Proto conversion (MapStruct mappers)
- gRPC client stubs (protoc)
- Controller implementations (GrpcClientGenerator)

**What you implement:**
- Go gRPC service business logic (`CatGrpcService`)
- Service registration with gRPC server

---

## 📦 Deployment Architecture

### On Prism Central (Production)

```
Prism Central (10.112.90.239)
│
├─ Mercury (Nginx) - Port 9440
│  ├─ Routes: /api/mock/v4.0.a1/* → Adonis (Port 8888)
│  └─ Config: /home/nutanix/config/mercury/mercury_request_handler_config_apimock_golang.json
│
├─ Adonis (Java/Spring Boot) - Port 8888
│  ├─ JAR: /usr/local/nutanix/adonis/lib/prism-service-17.6.0-SNAPSHOT.jar
│  ├─ Contains: golang-mock-grpc-client + all auto-generated code
│  ├─ Service: genesis managed (genesis stop/start adonis)
│  └─ Logs: ~/data/logs/adonis.out
│
├─ Go gRPC Service - Port 50051
│  ├─ Binary: ~/golang-mock-service/grpc-server
│  ├─ Service: Standalone process (nohup)
│  └─ Logs: ~/golang-mock-service/grpc-server.log
│
└─ API Artifacts
   └─ Path: ~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT/
      ├─ api-manifest-1.0.0-SNAPSHOT.json (CRITICAL for Adonis routing)
      ├─ swagger-all-1.0.0-SNAPSHOT.yaml
      └─ Other metadata files
```

### Lookup Cache Configuration

File: `~/api_artifacts/lookup_cache.json`

```json
[
  {
    "apiPath": "/mock/v4.0.a1/config",
    "artifactPath": "mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT"
  }
]
```

**Purpose:** Tells Adonis where to find API artifact metadata for the Mock service

---

## 🔧 Build & Deployment Process

### Local Development

**1. Build Code Generation (Java)**
```bash
cd ntnx-api-golang-mock-pc
mvn clean install -s settings.xml
# Generates: Java DTOs, Proto files, Mappers, Controllers
```

**2. Generate Go Protobuf Code**
```bash
cd ntnx-api-golang-mock-pc
./generate-grpc.sh
# Generates: config.pb.go, cat_service.pb.go, cat_service_grpc.pb.go
```

**3. Build Adonis JAR**
```bash
cd ntnx-api-prism-service
mvn clean package -DskipTests
# Creates: target/prism-service-17.6.0-SNAPSHOT.jar (348MB)
```

**4. Build Go gRPC Server**
```bash
cd ntnx-api-golang-mock
GOOS=linux GOARCH=amd64 go build -o bin/grpc-server-linux ./cmd/grpc-server/main.go
# Creates: bin/grpc-server-linux (15MB)
```

### Deployment to PC

**Files to copy:**
1. `bin/grpc-server-linux` → `~/golang-mock-service/grpc-server`
2. `prism-service-17.6.0-SNAPSHOT.jar` → `/usr/local/nutanix/adonis/lib/`
3. API artifacts → `~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT/`

**Configuration:**
1. Update `~/api_artifacts/lookup_cache.json`
2. Create Mercury config: `/home/nutanix/config/mercury/mercury_request_handler_config_apimock_golang.json`
3. Ensure `/etc/hosts` has zk entries

**Start Services:**
```bash
# Start gRPC server
cd ~/golang-mock-service
nohup ./grpc-server > grpc-server.log 2>&1 &

# Restart Adonis
genesis stop adonis mercury
cluster start
```

---

## 🎓 Architecture Patterns

### Following Guru Pattern

The Mock service follows the **exact same architecture** as `ntnx-api-guru`:

| Aspect | Guru | Mock Service |
|--------|------|--------------|
| **Language** | Go | Go |
| **Protocol** | gRPC (HTTP/2 + Proto) | gRPC (HTTP/2 + Proto) |
| **Gateway** | Adonis + Java controllers | Adonis + Java controllers |
| **Code Gen** | Maven (guru-pc) | Maven (golang-mock-pc) |
| **.pb.go files** | ✅ Yes | ✅ Yes |
| **Deployment** | Genesis managed + standalone | Genesis managed + standalone |
| **Integration** | REST → Adonis → gRPC → Guru | REST → Adonis → gRPC → Mock |

### Key Design Decisions

1. **Separation of Concerns**
   - API definitions (YAML) separate from implementation
   - Code generation separate from service logic
   - Gateway (Adonis) separate from service (Go)

2. **Type Safety**
   - Protocol Buffers ensure type safety across Go and Java
   - Compile-time checks prevent runtime errors
   - Auto-generated code reduces manual errors

3. **Performance**
   - gRPC (HTTP/2) is 10x faster than REST
   - Protocol Buffers are more efficient than JSON
   - Go service is lightweight and fast

4. **Maintainability**
   - YAML as single source of truth
   - Auto-generated code reduces maintenance
   - Clear separation between layers

5. **Production Ready**
   - Same pattern as production Guru service
   - Tested deployment process
   - Comprehensive documentation

---

## 📝 File Ownership

### Files You Create/Modify

**YAML API Definitions:**
- `catModel.yaml` - Data schemas
- `catEndpoint.yaml` - REST endpoints

**Go Service Implementation:**
- `cmd/grpc-server/main.go` - Server setup
- `grpc/cat_grpc_service.go` - Business logic

**Proto Service Definition (Optional):**
- `cat_service.proto` - gRPC service interface

### Files Auto-Generated (DON'T MODIFY)

**Go Code:**
- `config.pb.go`
- `cat_service.pb.go`
- `cat_service_grpc.pb.go`
- `config_model.go` (Go DTOs)

**Java Code:**
- `Cat.java`, `Location.java` (Java DTOs)
- `CatMapper.java` (Mappers)
- `CatApi.java` (Spring MVC interface)
- `CatApiController.java` (Controller implementation)
- Java proto classes

---

## 🚀 Summary

**What you have:**
- ✅ Complete gRPC service implementation (Go)
- ✅ Complete REST gateway integration (Adonis/Java)
- ✅ Full code generation pipeline (Maven)
- ✅ Production-ready deployment process
- ✅ Following Nutanix Guru patterns

**Your service is correctly integrated when:**
- Client sends REST/JSON → Adonis
- Adonis auto-converts JSON → Proto
- Adonis calls your Go gRPC service
- Your Go service processes the request
- Response flows back through Adonis
- Client receives JSON with Nutanix v4 fields

**Your understanding is 100% correct!** ✅

The auto-generated proxy code in Adonis handles all the translation, so your Go service only needs to handle gRPC requests and responses. This is exactly how production services like Guru work.

---

**Last Updated:** 2025-01-20  
**Status:** ✅ Complete Understanding Documented


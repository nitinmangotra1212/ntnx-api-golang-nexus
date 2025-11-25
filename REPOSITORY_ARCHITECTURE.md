# Repository Architecture & Deployment Flow

This document explains how each repository works and how deployment happens locally and on PC.

## 📦 Repository Overview

There are **3 main repositories** involved in the golang-mock service:

1. **`ntnx-api-golang-mock-pc`** - Code Generation (Maven-based)
2. **`ntnx-api-golang-mock`** - Go gRPC Server
3. **`ntnx-api-prism-service`** - Adonis (REST-to-gRPC Gateway)

---

## 1️⃣ ntnx-api-golang-mock-pc (Code Generation)

### Purpose
**Code generation repository** that converts YAML API definitions into:
- Go DTOs (Data Transfer Objects)
- Protocol Buffer definitions (.proto files)
- Java gRPC client code
- Java DTOs for Adonis

### Structure

```
ntnx-api-golang-mock-pc/
├── golang-mock-api-definitions/          # Source: YAML API definitions
│   └── defs/namespaces/mock/v4/modules/config/released/
│       ├── models/catModel.yaml          # Cat schema definition
│       └── api/catEndpoint.yaml          # API endpoint definitions
│
├── golang-mock-api-codegen/               # Code generators (Maven modules)
│   ├── golang-mock-go-dto-definitions/   # Generates Go DTOs
│   ├── golang-mock-protobuf-messages/    # Generates .proto message files
│   ├── golang-mock-protobuf-services/    # Generates .proto service files
│   ├── golang-mock-grpc-client/          # Generates Java gRPC client
│   └── golang-mock-java-dto-definitions/ # Generates Java DTOs
│
└── generated-code/                       # Generated output
    ├── dto/src/models/                   # Go DTOs (used by Go server)
    ├── protobuf/swagger/                 # .proto source files
    └── protobuf/mock/v4/config/          # Compiled .pb.go files
```

### How It Works

#### Step 1: YAML → Swagger/OpenAPI
```bash
cd ntnx-api-golang-mock-pc
mvn clean install -s settings.xml
```

**What happens:**
1. Maven reads YAML files from `golang-mock-api-definitions/defs/`
2. Combines them into a single `swagger-all-17.0.0-SNAPSHOT.yaml`
3. Validates the OpenAPI spec

#### Step 2: Swagger → Go DTOs
**Module:** `golang-mock-go-dto-definitions`

**Process:**
- Uses `swagger-codegen-maven-plugin` with custom Go templates
- Reads `swagger-all-*.yaml`
- Generates Go structs in `generated-code/dto/src/models/`
- Fixes import paths via `publishCode.sh` script

**Output:**
```go
// generated-code/dto/src/models/mock/v4/config/config_model.go
type Cat struct {
    CatId       int32     `json:"catId"`
    CatName     string    `json:"catName"`
    CatType     string    `json:"catType"`
    Description string    `json:"description"`
    Location    *Location `json:"location,omitempty"`
    Reserved_   map[string]interface{} `json:"$reserved,omitempty"`
}
```

#### Step 3: Swagger → Protocol Buffers
**Modules:** `golang-mock-protobuf-messages` + `golang-mock-protobuf-services`

**Process:**
- Converts Swagger to `.proto` files
- Generates message definitions (`Cat`, `Location`, etc.)
- Generates service definitions (`CatService` with RPC methods)
- Copies to `generated-code/protobuf/swagger/`

**Output:**
```protobuf
// generated-code/protobuf/swagger/mock/v4/config/cat_service.proto
service CatService {
  rpc ListCats(ListCatsRequest) returns (ListCatsResponse);
  rpc GetCat(GetCatRequest) returns (GetCatResponse);
  rpc CreateCat(CreateCatRequest) returns (CreateCatResponse);
  // ...
}
```

#### Step 4: .proto → .pb.go (Go gRPC Code)
**Script:** `generate-grpc.sh` (manual step)

**Process:**
```bash
cd ntnx-api-golang-mock-pc
./generate-grpc.sh
```

**What happens:**
1. Uses `protoc` (Protocol Buffer compiler)
2. Compiles `.proto` files to `.pb.go` files
3. Generates gRPC service stubs (`*_grpc.pb.go`)
4. Outputs to `generated-code/protobuf/mock/v4/config/`

**Output:**
- `config.pb.go` - Message types (Cat, Location, etc.)
- `cat_service.pb.go` - Request/Response types
- `cat_service_grpc.pb.go` - gRPC service interface

#### Step 5: Swagger → Java gRPC Client
**Module:** `golang-mock-grpc-client`

**Process:**
- Generates Java classes for Adonis
- Creates `MockConfigCatController` (REST endpoint handler)
- Creates `MockConfigCatServiceImpl` (gRPC client wrapper)
- Creates `GolangmockGrpcConfiguration` (Spring configuration)
- Packages as JAR: `golang-mock-grpc-client-17.0.0-SNAPSHOT.jar`

**Output:**
- Java classes in `target/generated-sources/swagger/src/`
- JAR file installed to local Maven repository

### Build Command

```bash
cd ~/ntnx-api-golang-mock-pc
mvn clean install -DskipTests -s settings.xml
```

**What this does:**
1. ✅ Generates Go DTOs → `generated-code/dto/`
2. ✅ Generates .proto files → `generated-code/protobuf/swagger/`
3. ✅ Generates Java gRPC client → JAR in local Maven repo
4. ✅ Generates Java DTOs → JAR in local Maven repo

**Then manually:**
```bash
./generate-grpc.sh  # Compiles .proto → .pb.go
```

---

## 2️⃣ ntnx-api-golang-mock (Go gRPC Server)

### Purpose
**Go-based gRPC server** that implements the CatService API.

### Structure

```
ntnx-api-golang-mock/
├── golang-mock-service/
│   ├── server/
│   │   └── main.go                    # Entry point
│   ├── grpc/
│   │   ├── grpc_server.go             # gRPC server setup
│   │   └── cat_grpc_service.go        # CatService implementation
│   ├── utils/
│   │   └── logging/
│   │       └── logger.go              # Logging setup
│   └── global/
│       └── global.go                  # Global state
│
├── go.mod                             # Go module definition
└── Makefile                           # Build commands
```

### How It Works

#### Dependencies (go.mod)
```go
require (
    github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto v0.0.0
    github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config v0.0.0
    google.golang.org/grpc v1.77.0
)

// Local replace directives (for development)
replace github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/dto => ../ntnx-api-golang-mock-pc/generated-code/dto/src
replace github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config => ../ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config
```

**Key Point:** Uses `replace` directives to point to local `generated-code` from `ntnx-api-golang-mock-pc`.

#### Server Implementation

1. **main.go** - Entry point:
   - Parses command-line flags (`-port`)
   - Initializes logger
   - Starts gRPC server

2. **grpc_server.go** - Server setup:
   - Creates gRPC server instance
   - Registers services (CatService)
   - Enables reflection (for grpcurl)
   - Listens on specified port

3. **cat_grpc_service.go** - Business logic:
   - Implements `CatServiceServer` interface (from generated code)
   - Handles RPC calls: `ListCats`, `GetCat`, `CreateCat`, etc.
   - Maintains in-memory mock data (100 cats)

### Build Command

**Local (macOS/Linux):**
```bash
cd ~/ntnx-api-golang-mock
make build-local
# OR
go build -o golang-mock-server-local-linux2 golang-mock-service/server/main.go
```

**For PC (Linux):**
```bash
make build
# OR
GOOS=linux GOARCH=amd64 go build -o golang-mock-server golang-mock-service/server/main.go
```

### Run Command

```bash
./golang-mock-server-local -port 9090
```

**What happens:**
1. Server starts on port 9090
2. Registers `CatService` with gRPC server
3. Initializes 100 mock cats
4. Enables gRPC reflection
5. Listens for incoming gRPC requests

---

## 3️⃣ ntnx-api-prism-service (Adonis Gateway)

### Purpose
**REST-to-gRPC gateway** that:
- Exposes REST API endpoints (port 8888)
- Converts REST requests to gRPC calls
- Routes to backend gRPC services (like golang-mock)

### Structure

```
ntnx-api-prism-service/
├── pom.xml                             # Maven POM with golang-mock dependency
├── src/main/resources/
│   └── application.yaml               # Spring Boot config
│       ├── adonis.controller.packages.onprem  # Package scanning
│       └── grpc.golangmock             # gRPC client config
└── target/
    └── prism-service-17.6.0-SNAPSHOT.jar  # Built JAR
```

### How It Works

#### Dependency (pom.xml)
```xml
<dependency>
    <groupId>com.nutanix.nutanix-core.ntnx-api.golang-mock-pc</groupId>
    <artifactId>golang-mock-grpc-client</artifactId>
    <version>17.0.0-SNAPSHOT</version>
</dependency>
```

**This JAR contains:**
- `MockConfigCatController` - REST endpoint: `GET /api/mock/v4.1/config/cats`
- `MockConfigCatServiceImpl` - Calls gRPC `CatService.ListCats()`
- `GolangmockGrpcConfiguration` - Creates `ManagedChannel` bean

#### Configuration (application.yaml)
```yaml
adonis:
  controller:
    packages:
      onprem: |
        mock.v4.config.server.controllers, \
        mock.v4.config.server.services, \
        mock.v4.server.configuration, \

grpc:
  golangmock:
    host: localhost
    port: 9090
```

**What this does:**
1. **Package scanning** - Spring discovers `MockConfigCatController`
2. **gRPC config** - Creates `ManagedChannel` to connect to golang-mock server

#### Request Flow

```
Client (REST) 
  → Mercury (port 9440, HTTPS)
    → Adonis (port 8888, HTTP)
      → MockConfigCatController (REST handler)
        → MockConfigCatServiceImpl (service layer)
          → ManagedChannel (gRPC client)
            → golang-mock-server (port 9090, gRPC)
```

### Build Command

```bash
cd ~/ntnx-api-prism-service
mvn clean install -DskipTests -s settings.xml
```

**Output:** `target/prism-service-17.6.0-SNAPSHOT.jar`

---

## 🔄 Complete Build & Deployment Flow

### Local Development Flow

#### Step 1: Generate Code
```bash
# Repository 1: Generate all code
cd ~/ntnx-api-golang-mock-pc
mvn clean install -DskipTests -s settings.xml
./generate-grpc.sh
```

**Output:**
- ✅ Go DTOs in `generated-code/dto/`
- ✅ .pb.go files in `generated-code/protobuf/mock/v4/config/`
- ✅ Java JARs in local Maven repo

#### Step 2: Build Go Server
```bash
# Repository 2: Build Go server
cd ~/ntnx-api-golang-mock
make build-local
```

**What happens:**
- Go compiler reads `go.mod`
- Uses `replace` directives to find generated code
- Compiles Go server binary

#### Step 3: Run Server Locally
```bash
# Run server
./golang-mock-server-local -port 9090

# Test directly (bypass Adonis)
grpcurl -plaintext -d '{"page": 1, "limit": 5}' localhost:9090 mock.v4.config.CatService/listCats
```

### PC Deployment Flow

#### Step 1: Generate Code (Same as Local)
```bash
cd ~/ntnx-api-golang-mock-pc
mvn clean install -DskipTests -s settings.xml
./generate-grpc.sh
```

#### Step 2: Build Go Server (Linux Binary)
```bash
cd ~/ntnx-api-golang-mock
make build  # Builds for Linux
# Output: golang-mock-server (Linux binary)
```

#### Step 3: Build Adonis (with golang-mock client)
```bash
cd ~/ntnx-api-prism-service
mvn clean install -DskipTests -s settings.xml
# Output: prism-service-17.6.0-SNAPSHOT.jar
```

#### Step 4: Deploy to PC
```bash
# Copy Go binary
scp golang-mock-server nutanix@PC_IP:/home/nutanix/golang-mock-build/

# Copy Adonis JAR
scp target/prism-service-17.6.0-SNAPSHOT.jar nutanix@PC_IP:/home/nutanix/adonis/lib/

# Copy API artifacts
scp -r ntnx-api-golang-mock-pc/golang-mock-api-definitions/target/generated-api-artifacts/* \
  nutanix@PC_IP:/home/nutanix/api_artifacts/mock/v4.r1.a1/golang-mock-api-definitions-17.0.0-SNAPSHOT/
```

#### Step 5: Configure & Start on PC
```bash
# SSH to PC
ssh nutanix@PC_IP

# Start Go server
cd ~/golang-mock-build
nohup ./golang-mock-server -port 9090 > golang-mock-server.log 2>&1 &

# Restart Adonis
genesis stop adonis
cluster start
```

#### Step 6: Test on PC
```bash
# Via Mercury (REST API)
curl -k -H "Authorization: Bearer $TOKEN" \
  https://PC_IP:9440/api/mock/v4.1/config/cats

# Direct gRPC (if grpcurl available)
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/listCats
```

---

## 🔗 Repository Dependencies

```
┌─────────────────────────────────────┐
│  ntnx-api-golang-mock-pc            │
│  (Code Generation)                   │
│                                      │
│  Input:  YAML files                  │
│  Output: Go DTOs, .proto, .pb.go,   │
│          Java JARs                   │
└──────────────┬───────────────────────┘
               │
               │ generates
               │
       ┌───────┴────────┐
       │                 │
       ▼                 ▼
┌──────────────┐  ┌──────────────────────┐
│ ntnx-api-    │  │ ntnx-api-prism-     │
│ golang-mock  │  │ service (Adonis)   │
│              │  │                     │
│ Uses:        │  │ Uses:               │
│ - Go DTOs    │  │ - Java gRPC client  │
│ - .pb.go     │  │   JAR               │
│              │  │                     │
│ Output:      │  │ Output:             │
│ - gRPC       │  │ - REST API          │
│   server     │  │ - REST→gRPC gateway │
│   (port 9090)│  │   (port 8888)       │
└──────────────┘  └──────────────────────┘
       │                 │
       │                 │
       └────────┬────────┘
                │
                │ gRPC calls
                │
                ▼
        ┌───────────────┐
        │   Client      │
        │   (REST API)  │
        └───────────────┘
```

---

## 📊 Key Differences: Local vs PC

| Aspect | Local Development | PC Deployment |
|--------|------------------|---------------|
| **Go Binary** | `golang-mock-server-local` (native OS) | `golang-mock-server` (Linux) |
| **Build Command** | `make build-local` | `make build` (GOOS=linux) |
| **Testing** | Direct gRPC (`grpcurl`) | Via Adonis/Mercury (REST) |
| **Adonis** | Not needed for testing | Required for REST API |
| **Port** | Any (default: 9090) | 9090 (gRPC), 8888 (Adonis), 9440 (Mercury) |
| **Dependencies** | Local `replace` directives | Same, but deployed separately |
| **Logs** | Console/stdout | File: `~/golang-mock-build/golang-mock-server.log` |

---

## 🎯 Summary

### Repository Roles

1. **`ntnx-api-golang-mock-pc`** = **Code Generator**
   - Converts YAML → Go DTOs, .proto, Java code
   - Run once when API definitions change
   - Outputs: `generated-code/` directory + Maven JARs

2. **`ntnx-api-golang-mock`** = **gRPC Server**
   - Implements the API business logic
   - Uses generated code from repository 1
   - Outputs: Go binary (gRPC server)

3. **`ntnx-api-prism-service`** = **REST Gateway**
   - Converts REST → gRPC
   - Uses Java gRPC client from repository 1
   - Outputs: Spring Boot JAR (Adonis)

### Build Order

```
1. ntnx-api-golang-mock-pc     (Generate code)
   ↓
2. ntnx-api-golang-mock        (Build Go server)
   ↓
3. ntnx-api-prism-service      (Build Adonis with golang-mock client)
   ↓
4. Deploy to PC                 (Copy files, configure, start)
```

### Local Testing

```bash
# 1. Generate code
cd ~/ntnx-api-golang-mock-pc && mvn clean install -s settings.xml && ./generate-grpc.sh

# 2. Build & run Go server
cd ~/ntnx-api-golang-mock && make build-local && ./golang-mock-server-local -port 9090

# 3. Test (in another terminal)
grpcurl -plaintext -d '{"page": 1, "limit": 5}' localhost:9090 mock.v4.config.CatService/listCats
```

**No Adonis needed for local gRPC testing!**

---

**Last Updated**: 2025-11-25  
**Key Takeaway**: `ntnx-api-golang-mock-pc` generates code → `ntnx-api-golang-mock` uses it → `ntnx-api-prism-service` bridges REST to gRPC


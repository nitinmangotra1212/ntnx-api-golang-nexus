# 🚀 gRPC Testing Guide (grpcurl Recommended)

## ⚠️ **IMPORTANT: Postman gRPC Has Known Issues**

**If you get "12 UNIMPLEMENTED" error in Postman, see:** [GRPC_TESTING_SOLUTION.md](./GRPC_TESTING_SOLUTION.md)

**TL;DR:** 
- ❌ **Postman gRPC is unreliable** (buggy implementation)
- ✅ **Use grpcurl instead** (100% reliable, industry standard)
- ✅ **Your server works perfectly** (proven with grpcurl)

---

## 📦 Prerequisites

1. **grpcurl** (Recommended): `brew install grpcurl`
2. **gRPC Server Running:** `./bin/grpc-server` on port 50051
3. **Proto Files:** Available in `ntnx-api-golang-mock-pc/generated-code/protobuf/swagger/mock/v4/config/`

---

## 🎯 Method 1: Using grpcurl (Recommended)

Since Postman's gRPC support is still evolving, **grpcurl is the best tool for testing gRPC APIs**.

### Installation

```bash
# macOS
brew install grpcurl

# Verify installation
grpcurl --version
```

### Quick Test Commands

```bash
# 1. List all services
grpcurl -plaintext localhost:50051 list

# 2. List methods in CatService
grpcurl -plaintext localhost:50051 list mock.v4.config.CatService

# 3. List Cats
grpcurl -plaintext -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats

# 4. Get Cat by ID
grpcurl -plaintext -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/GetCat

# 5. Create Cat
grpcurl -plaintext -d '{
  "cat": {
    "cat_name": "Whiskers",
    "cat_type": "Persian",
    "description": "A fluffy white cat"
  }
}' localhost:50051 mock.v4.config.CatService/CreateCat

# 6. Update Cat
grpcurl -plaintext -d '{
  "cat_id": 42,
  "cat": {
    "cat_name": "Updated Cat",
    "cat_type": "Updated Type"
  }
}' localhost:50051 mock.v4.config.CatService/UpdateCat

# 7. Delete Cat
grpcurl -plaintext -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/DeleteCat

# 8. Async Operation
grpcurl -plaintext -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/GetCatAsync
```

---

## 🔧 Method 2: Using Postman (Alternative)

### Step 1: Enable gRPC in Postman

1. Open Postman
2. Click **New** → **gRPC Request** (not REST)
3. Or use **File** → **New** → **gRPC Request**

### Step 2: Import Proto Files

**Option A: Via URL (if server has reflection)**
1. In the gRPC request window, click **Select a method**
2. Choose **Server Reflection**
3. Enter server URL: `localhost:50051`
4. Click **Connect**
5. Postman will auto-load all services and methods

**Option B: Via Proto Files (Manual)**
1. Click **Select a method** → **Import a .proto file**
2. Navigate to `/Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/protobuf/swagger/mock/v4/config/`
3. Select `cat_service.proto`
4. Import dependencies:
   - Also import `config.proto` when prompted
5. Click **Next**

### Step 3: Configure Connection

1. **Server URL:** `localhost:50051`
2. **TLS:** OFF (we're using `-plaintext` / insecure connection)
3. **Service:** `mock.v4.config.CatService`

### Step 4: Test Requests

#### Request 1: List Cats

**Method:** `ListCats`

**Message:**
```json
{
  "page": 1,
  "limit": 10
}
```

**Expected Response:**
```json
{
  "cats": [
    {
      "catId": 1,
      "catName": "gRPC-Cat-1",
      "catType": "TYPE_GRPC",
      "description": "A gRPC-powered cat"
    }
  ],
  "totalAvailableResults": 100
}
```

---

#### Request 2: Get Cat

**Method:** `GetCat`

**Message:**
```json
{
  "cat_id": 42
}
```

**Expected Response:**
```json
{
  "cat": {
    "catId": 42,
    "catName": "gRPC-Cat-42",
    "catType": "TYPE_GRPC",
    "description": "A gRPC-powered cat",
    "location": {
      "city": "gRPC City",
      "country": {
        "state": "gRPC State"
      }
    }
  }
}
```

---

#### Request 3: Create Cat

**Method:** `CreateCat`

**Message:**
```json
{
  "cat": {
    "cat_name": "Postman Cat",
    "cat_type": "Test",
    "description": "Created via Postman",
    "location": {
      "city": "Test City",
      "country": {
        "state": "Test State"
      }
    }
  }
}
```

**Expected Response:**
```json
{
  "cat": {
    "catId": 101,
    "catName": "Postman Cat",
    "catType": "Test",
    "description": "Created via Postman"
  }
}
```

---

#### Request 4: Update Cat

**Method:** `UpdateCat`

**Message:**
```json
{
  "cat_id": 42,
  "cat": {
    "cat_name": "Updated via Postman",
    "cat_type": "Updated",
    "description": "Updated description"
  }
}
```

---

#### Request 5: Delete Cat

**Method:** `DeleteCat`

**Message:**
```json
{
  "cat_id": 42
}
```

**Expected Response:**
```json
{
  "message": "Cat with ID 42 deleted via gRPC"
}
```

---

#### Request 6: Async Operation

**Method:** `GetCatAsync`

**Message:**
```json
{
  "cat_id": 42
}
```

**Expected Response:**
```json
{
  "taskId": "550e8400-e29b-41d4-a716-446655440000",
  "pollUrl": "http://localhost:9010/tasks/550e8400-e29b-41d4-a716-446655440000"
}
```

**Note:** Poll the REST Task Server (port 9010) to check status.

---

## 🎨 Method 3: Using BloomRPC (Alternative GUI)

BloomRPC is a GUI client specifically for gRPC (like Postman for REST).

### Installation

```bash
# macOS
brew install --cask bloomrpc

# Or download from: https://github.com/bloomrpc/bloomrpc/releases
```

### Usage

1. Open BloomRPC
2. Click **Import Protos** (+ icon)
3. Navigate to `/Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/protobuf/swagger/mock/v4/config/`
4. Select `cat_service.proto`
5. Also import `config.proto`
6. Set server URL: `localhost:50051`
7. Uncheck **TLS** (we're using insecure connection)
8. Select method (e.g., `ListCats`)
9. Enter request JSON
10. Click **Play** button

---

## 🧪 Method 4: Using Go gRPC Client

### Create a Test Client

**File:** `test-grpc-client.go`

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/nutanix/ntnx-api-golang-mock-pc/generated-code/protobuf/mock/v4/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Connect to gRPC server
	conn, err := grpc.Dial("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewCatServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test 1: List Cats
	fmt.Println("=== Test 1: List Cats ===")
	listResp, err := client.ListCats(ctx, &pb.ListCatsRequest{
		Page:  1,
		Limit: 5,
	})
	if err != nil {
		log.Fatalf("ListCats failed: %v", err)
	}
	fmt.Printf("Total cats: %d\n", listResp.TotalAvailableResults)
	for _, cat := range listResp.Cats {
		fmt.Printf("  - Cat %d: %s (%s)\n", cat.CatId, cat.CatName, cat.CatType)
	}

	// Test 2: Get Cat
	fmt.Println("\n=== Test 2: Get Cat ===")
	getResp, err := client.GetCat(ctx, &pb.GetCatRequest{
		CatId: 42,
	})
	if err != nil {
		log.Fatalf("GetCat failed: %v", err)
	}
	fmt.Printf("Cat: %+v\n", getResp.Cat)

	// Test 3: Create Cat
	fmt.Println("\n=== Test 3: Create Cat ===")
	createResp, err := client.CreateCat(ctx, &pb.CreateCatRequest{
		Cat: &pb.Cat{
			CatName:     "Test Cat",
			CatType:     "Test",
			Description: "Created via Go client",
		},
	})
	if err != nil {
		log.Fatalf("CreateCat failed: %v", err)
	}
	fmt.Printf("Created cat: %+v\n", createResp.Cat)

	// Test 4: Update Cat
	fmt.Println("\n=== Test 4: Update Cat ===")
	updateResp, err := client.UpdateCat(ctx, &pb.UpdateCatRequest{
		CatId: createResp.Cat.CatId,
		Cat: &pb.Cat{
			CatName:     "Updated Test Cat",
			CatType:     "Updated",
			Description: "Updated via Go client",
		},
	})
	if err != nil {
		log.Fatalf("UpdateCat failed: %v", err)
	}
	fmt.Printf("Updated cat: %+v\n", updateResp.Cat)

	// Test 5: Delete Cat
	fmt.Println("\n=== Test 5: Delete Cat ===")
	deleteResp, err := client.DeleteCat(ctx, &pb.DeleteCatRequest{
		CatId: createResp.Cat.CatId,
	})
	if err != nil {
		log.Fatalf("DeleteCat failed: %v", err)
	}
	fmt.Printf("Delete response: %s\n", deleteResp.Message)

	// Test 6: Async Operation
	fmt.Println("\n=== Test 6: Async Operation ===")
	asyncResp, err := client.GetCatAsync(ctx, &pb.GetCatAsyncRequest{
		CatId: 42,
	})
	if err != nil {
		log.Fatalf("GetCatAsync failed: %v", err)
	}
	fmt.Printf("Task ID: %s\n", asyncResp.TaskId)
	fmt.Printf("Poll URL: %s\n", asyncResp.PollUrl)
}
```

**Run:**
```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock
go run test-grpc-client.go
```

---

## 📊 Comparison: grpcurl vs Postman vs BloomRPC

| Feature | grpcurl | Postman | BloomRPC |
|---------|---------|---------|----------|
| **Ease of Use** | CLI (technical) | GUI (familiar) | GUI (gRPC-specific) |
| **Proto Loading** | Auto (reflection) | Manual or reflection | Manual |
| **Speed** | ⚡ Fastest | Medium | Medium |
| **Automation** | ✅ Scriptable | ✅ Collection Runner | ❌ Manual only |
| **Best For** | CI/CD, Scripts | Teams, Collections | Quick testing |
| **Installation** | brew install | Download app | Download app |

---

## 🎯 Recommended Testing Flow

### For Development:
```bash
# Quick tests
grpcurl -plaintext -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats
```

### For Demo:
1. Use Postman or BloomRPC (visual)
2. Show the GUI making gRPC calls
3. Show Protocol Buffer request/response

### For CI/CD:
```bash
# Automated tests
./test-grpc-gateway.sh  # Your existing script
```

---

## 🚨 Troubleshooting

### Error: "Failed to connect"
```bash
# Check if server is running
lsof -ti:50051

# Start server if not running
cd /Users/nitin.mangotra/ntnx-api-golang-mock
./bin/grpc-server
```

### Error: "Method not found"
```bash
# Verify server has reflection enabled
grpcurl -plaintext localhost:50051 list

# Should show: mock.v4.config.CatService
```

### Error: "Proto file not found" (Postman)
```bash
# Make sure to import both proto files:
# 1. cat_service.proto
# 2. config.proto (dependency)
```

### Error: "Invalid JSON"
```bash
# Field names must match .proto file:
# ✅ Correct: "cat_id"
# ❌ Wrong: "catId" (Go field name)
```

---

## 📚 Additional Resources

### Proto Files Location:
```
/Users/nitin.mangotra/ntnx-api-golang-mock-pc/generated-code/protobuf/swagger/mock/v4/config/
├── config.proto          ← Message definitions
└── cat_service.proto     ← Service definition
```

### gRPC Server Code:
```
/Users/nitin.mangotra/ntnx-api-golang-mock/
├── grpc/cat_grpc_service.go     ← Service implementation
└── cmd/grpc-server/main.go      ← Server setup
```

### Documentation:
- **Code Generation Flow:** See `CODE_GENERATION_FLOW.md`
- **gRPC Implementation:** See `GRPC_IMPLEMENTATION.md`
- **Demo Guide:** See `GRPC_DEMO_GUIDE.md`

---

## 🎉 Quick Start

**Fastest way to test:**
```bash
# 1. Start server
./bin/grpc-server

# 2. Test with grpcurl (new terminal)
grpcurl -plaintext -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats
```

**That's it! You're testing gRPC! 🚀**


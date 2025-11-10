#!/bin/bash

echo "🧪 Quick gRPC Test Script"
echo "================================"
echo ""

# Check if server is running
if ! lsof -ti:50051 > /dev/null 2>&1; then
    echo "❌ gRPC server not running on port 50051"
    echo "Start it with: ./bin/grpc-server"
    exit 1
fi

echo "✅ gRPC server is running"
echo ""

# Test 1: List services
echo "📋 Test 1: List Services"
grpcurl -plaintext localhost:50051 list
echo ""

# Test 2: List Cats
echo "📋 Test 2: List Cats (5 cats)"
grpcurl -plaintext -d '{"page":1,"limit":5}' localhost:50051 mock.v4.config.CatService/ListCats | head -30
echo ""

# Test 3: Get Cat
echo "📋 Test 3: Get Cat by ID (42)"
grpcurl -plaintext -d '{"cat_id":42}' localhost:50051 mock.v4.config.CatService/GetCat
echo ""

# Test 4: Create Cat
echo "📋 Test 4: Create Cat"
grpcurl -plaintext -d '{"cat":{"cat_name":"Test Cat","cat_type":"Test","description":"Created via grpcurl"}}' localhost:50051 mock.v4.config.CatService/CreateCat
echo ""

echo "================================"
echo "✅ All gRPC tests completed!"

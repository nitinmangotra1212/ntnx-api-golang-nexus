#!/bin/bash

# Generate Go code from protobuf definitions for statsGW
# This generates the gRPC client code from graphql_interface.proto

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🔧 Generating protobuf code for statsGW..."

# Check if protoc is available
if ! command -v protoc &> /dev/null; then
    echo "❌ Error: protoc not found. Please install Protocol Buffers compiler."
    exit 1
fi

# Check if protoc-gen-go is available
if ! command -v protoc-gen-go &> /dev/null; then
    echo "⚠️  Warning: protoc-gen-go not found. Installing..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is available
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "⚠️  Warning: protoc-gen-go-grpc not found. Installing..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Generate Go code
echo "📦 Generating Go code from graphql_interface.proto..."
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    graphql_interface.proto

if [ $? -eq 0 ]; then
    echo "✅ Protobuf code generated successfully!"
    echo "   - graphql_interface.pb.go"
    echo "   - graphql_interface_grpc.pb.go"
else
    echo "❌ Failed to generate protobuf code"
    exit 1
fi


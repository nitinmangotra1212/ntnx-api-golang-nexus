build:
	$(info Building Golang Nexus Service binary for Linux)
	$(info ===================================)
	$(info )
	@echo "Checking if protobuf files are generated..."
	@if [ ! -f "../ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config/item_service.pb.go" ]; then \
		echo "❌ Protobuf files not found. Running generate-grpc.sh..."; \
		cd ../ntnx-api-golang-nexus-pc && ./generate-grpc.sh; \
	fi
	@echo "Verifying import paths are fixed..."
	@cd ../ntnx-api-golang-nexus-pc && \
		find generated-code/protobuf -name "*.pb.go" -type f -exec grep -l 'response "common/v1/response"' {} \; > /tmp/needs_fix.txt 2>/dev/null || true; \
		if [ -s /tmp/needs_fix.txt ]; then \
			echo "Fixing common/v1 import paths..."; \
			find generated-code/protobuf -name "*.pb.go" -type f -exec sed -i '' 's|response "common/v1/response"|response "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response"|g' {} \; && \
			find generated-code/protobuf -name "*.pb.go" -type f -exec sed -i '' 's|config "common/v1/config"|config "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config"|g' {} \; && \
			find generated-code/protobuf -name "*.pb.go" -type f -exec sed -i '' 's|error1 "nexus/v4/error"|error1 "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error"|g' {} \; && \
			echo "✅ Import paths fixed"; \
		fi
	GOOS=linux GOARCH=amd64 go build -o golang-nexus-server golang-nexus-service/server/main.go

build-local:
	$(info Building Golang Nexus Service binary for local OS)
	$(info ===================================)
	$(info )
	@echo "Checking if protobuf files are generated..."
	@if [ ! -f "../ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/config/item_service.pb.go" ]; then \
		echo "❌ Protobuf files not found. Running generate-grpc.sh..."; \
		cd ../ntnx-api-golang-nexus-pc && ./generate-grpc.sh; \
	fi
	@echo "Verifying import paths are fixed..."
	@cd ../ntnx-api-golang-nexus-pc && \
		find generated-code/protobuf -name "*.pb.go" -type f -exec grep -l 'response "common/v1/response"' {} \; > /tmp/needs_fix.txt 2>/dev/null || true; \
		if [ -s /tmp/needs_fix.txt ]; then \
			echo "Fixing common/v1 import paths..."; \
			find generated-code/protobuf -name "*.pb.go" -type f -exec sed -i '' 's|response "common/v1/response"|response "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/response"|g' {} \; && \
			find generated-code/protobuf -name "*.pb.go" -type f -exec sed -i '' 's|config "common/v1/config"|config "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/common/v1/config"|g' {} \; && \
			find generated-code/protobuf -name "*.pb.go" -type f -exec sed -i '' 's|error1 "nexus/v4/error"|error1 "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/protobuf/nexus/v4/error"|g' {} \; && \
			echo "✅ Import paths fixed"; \
		fi
	go build -o golang-nexus-server-local golang-nexus-service/server/main.go

run-local: build-local
	$(info Running golang-nexus-server locally)
	$(info ===================================)
	$(info )
	./golang-nexus-server-local -port 9090

test:
	$(info Running tests)
	$(info =============)
	$(info )
	go test -v ./...

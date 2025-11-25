build:
	$(info Building Golang Mock Service binary for Linux)
	$(info ===================================)
	$(info )
	GOOS=linux GOARCH=amd64 go build -o golang-mock-server golang-mock-service/server/main.go

build-local:
	$(info Building Golang Mock Service binary for local OS)
	$(info ===================================)
	$(info )
	go build -o golang-mock-server-local golang-mock-service/server/main.go

run-local: build-local
	$(info Running golang-mock-server locally)
	$(info ===================================)
	$(info )
	./golang-mock-server-local -port 9090

test:
	$(info Running tests)
	$(info =============)
	$(info )
	go test -v ./...

build:
	$(info Building Golang Mock Service binary)
	$(info ===================================)
	$(info )
	GOOS=linux GOARCH=amd64 go build -o golang-mock-server golang-mock-service/server/main.go

test:
	$(info Running tests)
	$(info =============)
	$(info )
	go test -v ./...

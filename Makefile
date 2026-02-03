.PHONY: all build install generate test lint clean help

# Default target
all: generate build test

# Build the protoc plugin
build:
	go build -v ./...

# Install the protoc plugin to $GOPATH/bin
install:
	go install ./protoc-gen-grpc-mock

# Generate protobuf code (requires buf and protoc plugins)
generate: install
	buf generate

# Run tests
test:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -f coverage.out coverage.html
	rm -f protoc-gen-grpc-mock

# Format code
fmt:
	go fmt ./...
	goimports -w -local github.com/ankurs/grpc-mock .

# Tidy dependencies
tidy:
	go mod tidy

# Verify dependencies
verify:
	go mod verify

# Show help
help:
	@echo "Available targets:"
	@echo "  all            - Generate, build, and test (default)"
	@echo "  build          - Build the project"
	@echo "  install        - Install the protoc plugin"
	@echo "  generate       - Generate protobuf code"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run linter"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format code"
	@echo "  tidy           - Tidy go.mod"
	@echo "  verify         - Verify dependencies"
	@echo "  help           - Show this help"

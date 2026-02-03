# grpc-mock

[![Go](https://github.com/ankurs/grpc-mock/actions/workflows/go.yml/badge.svg)](https://github.com/ankurs/grpc-mock/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ankurs/grpc-mock)](https://goreportcard.com/report/github.com/ankurs/grpc-mock)
[![GoDoc](https://godoc.org/github.com/ankurs/grpc-mock?status.svg)](https://godoc.org/github.com/ankurs/grpc-mock)

A gRPC mocking library for Go that enables recording and replaying gRPC interactions. Perfect for testing gRPC clients and services without requiring actual server connections.

## Features

- **Record Mode**: Capture real gRPC interactions to JSON files
- **Replay Mode**: Replay recorded responses for deterministic testing
- **Request Matching**: Match requests based on service, method, and request content
- **Field Ignoring**: Ignore specific fields during request matching (e.g., timestamps, IDs)
- **Error Simulation**: Simulate error responses for testing error handling
- **Configurable Delays**: Add artificial delays to simulate network latency
- **Code Generation**: Generate mock server implementations from `.proto` files
- **Type Safety**: Full proto.Message type safety with proper error handling

## Installation

```bash
go get github.com/ankurs/grpc-mock
```

To install the protoc plugin for code generation:

```bash
go install github.com/ankurs/grpc-mock/protoc-gen-grpc-mock@latest
```

## Quick Start

### 1. Recording gRPC Interactions

Use the `MockingInterceptor` to record all gRPC calls to a file:

```go
package main

import (
    "github.com/ankurs/grpc-mock/mocker"
    "google.golang.org/grpc"
)

func main() {
    // Create a gRPC connection with the recording interceptor
    conn, err := grpc.Dial(
        "localhost:50051",
        grpc.WithUnaryInterceptor(mocker.MockingInterceptor("recordings.json")),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    defer mocker.CloseCapture() // Ensure all recordings are flushed

    // Use your gRPC client as normal
    client := pb.NewMyServiceClient(conn)
    resp, err := client.MyMethod(ctx, &pb.MyRequest{...})
    // Interactions are automatically recorded to recordings.json
}
```

### 2. Replaying Recorded Interactions

Use the `Mocker` to replay recorded responses in tests:

```go
package myservice_test

import (
    "context"
    "testing"

    "github.com/ankurs/grpc-mock/mocker"
    pb "github.com/example/myservice"
)

func TestMyService(t *testing.T) {
    // Load recorded interactions
    mock, err := mocker.NewMocker("testdata/recordings.json")
    if err != nil {
        t.Fatal(err)
    }

    // Create client using the mock
    client := pb.NewMyServiceClient(mock)

    // Call methods - responses come from recorded data
    resp, err := client.MyMethod(context.Background(), &pb.MyRequest{
        Field: "expected_value",
    })
    if err != nil {
        t.Fatal(err)
    }

    // Assert on response
    if resp.Result != "expected_result" {
        t.Errorf("unexpected result: %s", resp.Result)
    }
}
```

### 3. Using Generated Mock Servers

Generate mock server implementations using the protoc plugin:

```bash
# Using buf
buf generate

# Or using protoc directly
protoc --go_out=. --go-grpc_out=. --grpc-mock_out=. myservice.proto
```

Then use the generated mock in your tests:

```go
package myservice_test

import (
    "context"
    "testing"

    "github.com/ankurs/grpc-mock/mocker"
    pb "github.com/example/myservice"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
)

func TestMyServiceServer(t *testing.T) {
    // Load mock data
    mock, err := mocker.NewMocker("testdata/recordings.json")
    if err != nil {
        t.Fatal(err)
    }

    // Create mock server using generated code
    server := pb.MakeMockMyServiceServer(mock)

    // Set up in-memory gRPC server
    listener := bufconn.Listen(1024 * 1024)
    s := grpc.NewServer()
    pb.RegisterMyServiceServer(s, server)

    go s.Serve(listener)
    defer s.Stop()

    // Create client and test
    conn, _ := grpc.DialContext(ctx, "",
        grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
            return listener.Dial()
        }),
        grpc.WithInsecure(),
    )
    defer conn.Close()

    client := pb.NewMyServiceClient(conn)
    resp, err := client.MyMethod(ctx, &pb.MyRequest{...})
    // ...
}
```

## Recording Format

Recorded interactions are stored as JSON with a custom separator:

```json
{
  "service": "MyService",
  "method": "MyMethod",
  "request": {
    "field": "value"
  },
  "response": {
    "result": "success"
  }
}
------##------
{
  "service": "MyService",
  "method": "AnotherMethod",
  "request": null,
  "response": {
    "data": "any request matches"
  }
}
```

### Configuration Options

- **request**: The expected request. Use `null` to match any request.
- **response**: The response to return. Use `null` for empty responses.
- **error**: An error message to return instead of a response.
- **ignore**: List of field paths to ignore during request matching.

### Ignoring Fields

Use the `ignore` field to exclude certain paths from request matching:

```json
{
  "service": "MyService",
  "method": "CreateOrder",
  "request": {
    "userId": "user123",
    "timestamp": "2024-01-01T00:00:00Z"
  },
  "response": {
    "orderId": "order456"
  },
  "ignore": ["root.timestamp"]
}
```

### Simulating Errors

Return errors instead of responses:

```json
{
  "service": "MyService",
  "method": "GetUser",
  "request": {
    "userId": "nonexistent"
  },
  "response": null,
  "error": "user not found"
}
```

## Configuration Options

### Mocker Options

```go
// Add artificial delay to responses
mock, err := mocker.NewMocker("recordings.json",
    mocker.WithMinDelay(10*time.Millisecond),
    mocker.WithMaxDelay(50*time.Millisecond),
)

// Or use fixed delay
mock, err := mocker.NewMocker("recordings.json",
    mocker.WithDelay(25*time.Millisecond),
)

// Or use delay range
mock, err := mocker.NewMocker("recordings.json",
    mocker.WithDelayRange(10*time.Millisecond, 100*time.Millisecond),
)
```

### Custom Logger

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

mock, err := mocker.NewMockerWithLogger("recordings.json", logger)
```

## Development

### Prerequisites

- Go 1.22 or later
- [buf](https://buf.build/) (for protobuf generation)
- [golangci-lint](https://golangci-lint.run/) (for linting)

### Building

```bash
# Build all packages
make build

# Install the protoc plugin
make install
```

### Testing

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage
```

### Linting

```bash
make lint
```

### Code Generation

```bash
# Regenerate protobuf code
make generate
```

## API Reference

### mocker.MockingInterceptor

```go
func MockingInterceptor(filePath string) grpc.UnaryClientInterceptor
```

Returns a gRPC interceptor that records all unary RPC calls to the specified file.

### mocker.NewMocker

```go
func NewMocker(filePath string, opts ...Option) (Mocker, error)
```

Creates a new Mocker from a recorded interactions file.

### mocker.CloseCapture

```go
func CloseCapture() error
```

Closes the capture file and flushes all pending writes. Call this when done recording.

### Interfaces

```go
// Mocker combines client and server mocking capabilities
type Mocker interface {
    MockServer
    MockClient
}

// MockServer provides server-side mocking
type MockServer interface {
    Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error)
}

// MockClient provides client-side mocking (implements grpc.ClientConnInterface)
type MockClient interface {
    grpc.ClientConnInterface
}
```

## Error Types

```go
var (
    ErrNoMatchingRequest      = errors.New("no matching request found")
    ErrStreamingNotSupported  = errors.New("streaming RPCs are not supported")
    ErrRequestNotProtoMessage = errors.New("request is not a proto.Message")
    ErrResponseNotProtoMessage = errors.New("response is not a proto.Message")
)
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

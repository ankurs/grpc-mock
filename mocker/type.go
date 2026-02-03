package mocker

import (
	"context"

	"google.golang.org/grpc"
)

// MockServer is the interface for serving mock gRPC responses.
// It is typically embedded in generated mock server implementations.
type MockServer interface {
	// Serve handles a mock request and returns the corresponding response.
	// The service and method parameters identify the gRPC method being called,
	// and request contains the serialized request message.
	Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error)
}

// MockClient is the interface for making mock gRPC client calls.
// It embeds grpc.ClientConnInterface to be compatible with generated gRPC clients.
type MockClient interface {
	grpc.ClientConnInterface
}

// Mocker combines both MockServer and MockClient interfaces,
// allowing the same mock to be used for both client-side and server-side mocking.
type Mocker interface {
	MockServer
	MockClient
}

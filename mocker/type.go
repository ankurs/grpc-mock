package mocker

import (
	"context"

	"google.golang.org/grpc"
)

type MockServer interface {
	Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error)
}

type MockClient interface {
	grpc.ClientConnInterface
}

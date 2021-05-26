package server

import "context"

type MockServer interface {
	Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error)
}

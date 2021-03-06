// Code generated by protoc-gen-grpc-mock. DO NOT EDIT.

package example

import (
	context "context"
	json "encoding/json"
	mocker "github.com/ankurs/grpc-mock/mocker"
)

// ExampleService
type mockExampleService struct {
	mocker.MockServer // embedding mock server interface
}

// method -- Echo
func (m *mockExampleService) Echo(ctx context.Context, input *EchoRequest) (*EchoResponse, error) {
	req, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	resp, err := m.Serve(ctx, "ExampleService", "Echo", req)
	if err != nil {
		return nil, err
	}
	output := &EchoResponse{}
	err = json.Unmarshal(resp, output)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func MakeMockExampleServiceServer(mock mocker.MockServer) ExampleServiceServer {
	return &mockExampleService{mock}
}

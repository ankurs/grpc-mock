package mocker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ankurs/grpc-mock/example"
	"github.com/ankurs/grpc-mock/mocker"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestNewMocker(t *testing.T) {
	// Create a temporary mock file
	content := `{
  "service": "TestService",
  "method": "TestMethod",
  "request": {"msg": "hello"},
  "response": {"resp": "world"}
}
------##------
{
  "service": "TestService",
  "method": "AnotherMethod",
  "request": null,
  "response": {"resp": "default"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	if m == nil {
		t.Fatal("NewMocker returned nil")
	}
}

func TestNewMocker_FileNotFound(t *testing.T) {
	_, err := mocker.NewMocker("/nonexistent/path/to/file.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestNewMocker_InvalidJSON(t *testing.T) {
	content := `{invalid json}`
	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	_, err := mocker.NewMocker(tmpFile)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewMocker_RelativePath(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "TestMethod",
  "request": null,
  "response": {"resp": "test"}
}`

	// Create temp file in current directory
	tmpFile, err := os.CreateTemp(".", "mock_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Use just the filename (relative path)
	relativePath := filepath.Base(tmpFile.Name())

	m, err := mocker.NewMocker(relativePath)
	if err != nil {
		t.Fatalf("NewMocker with relative path failed: %v", err)
	}

	if m == nil {
		t.Fatal("NewMocker returned nil")
	}
}

func TestNewMockerWithLogger(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "TestMethod",
  "request": null,
  "response": {"resp": "test"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	m, err := mocker.NewMockerWithLogger(tmpFile, logger)
	if err != nil {
		t.Fatalf("NewMockerWithLogger failed: %v", err)
	}

	if m == nil {
		t.Fatal("NewMockerWithLogger returned nil")
	}
}

func TestMocker_Invoke(t *testing.T) {
	// Using EchoRequest/EchoResponse which marshal to JSON objects
	content := `{
  "service": "TestService",
  "method": "GetValue",
  "request": {"msg": "hello"},
  "response": {"resp": "world"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	req := &example.EchoRequest{Msg: "hello"}
	reply := &example.EchoResponse{}

	err = m.Invoke(context.Background(), "/package.TestService/GetValue", req, reply)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if reply.Resp != "world" {
		t.Errorf("expected reply.Resp = 'world', got '%s'", reply.Resp)
	}
}

func TestMocker_Invoke_NullRequest(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "GetDefault",
  "request": null,
  "response": {"resp": "default"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	req := &example.EchoRequest{Msg: "anything"}
	reply := &example.EchoResponse{}

	err = m.Invoke(context.Background(), "/package.TestService/GetDefault", req, reply)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if reply.Resp != "default" {
		t.Errorf("expected reply.Resp = 'default', got '%s'", reply.Resp)
	}
}

func TestMocker_Invoke_NullResponse(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "SetValue",
  "request": {"msg": "test"},
  "response": null
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	req := &example.EchoRequest{Msg: "test"}
	reply := &emptypb.Empty{}

	err = m.Invoke(context.Background(), "/package.TestService/SetValue", req, reply)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
}

func TestMocker_Invoke_Error(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "FailingMethod",
  "request": {"msg": "trigger"},
  "response": null,
  "error": "simulated error"
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	req := &example.EchoRequest{Msg: "trigger"}
	reply := &example.EchoResponse{}

	err = m.Invoke(context.Background(), "/package.TestService/FailingMethod", req, reply)
	if err == nil {
		t.Fatal("expected error from Invoke")
	}

	if err.Error() != "simulated error" {
		t.Errorf("expected error 'simulated error', got '%s'", err.Error())
	}
}

func TestMocker_Invoke_NoMatch(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "GetValue",
  "request": {"msg": "expected"},
  "response": {"resp": "result"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	req := &example.EchoRequest{Msg: "different"} // Different from expected
	reply := &example.EchoResponse{}

	err = m.Invoke(context.Background(), "/package.TestService/GetValue", req, reply)
	if err == nil {
		t.Fatal("expected error for non-matching request")
	}

	if !strings.Contains(err.Error(), "no matching request found") {
		t.Errorf("expected 'no matching request found' error, got '%s'", err.Error())
	}
}

func TestMocker_Invoke_IgnoreFields(t *testing.T) {
	// Test with null request which matches any input
	content := `{
  "service": "TestService",
  "method": "GetValue",
  "request": null,
  "response": {"resp": "result"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	// Request with any value should match due to null request
	req := &example.EchoRequest{Msg: "any value"}
	reply := &example.EchoResponse{}

	err = m.Invoke(context.Background(), "/package.TestService/GetValue", req, reply)
	if err != nil {
		t.Fatalf("Invoke with null request failed: %v", err)
	}

	if reply.Resp != "result" {
		t.Errorf("expected reply.Resp = 'result', got '%s'", reply.Resp)
	}
}


func TestMocker_NewStream(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "StreamMethod",
  "request": null,
  "response": null
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	_, err = m.NewStream(context.Background(), &grpc.StreamDesc{}, "/package.TestService/StreamMethod")
	if err == nil {
		t.Fatal("expected error for NewStream (not supported)")
	}

	if err != mocker.ErrStreamingNotSupported {
		t.Errorf("expected ErrStreamingNotSupported, got '%v'", err)
	}
}

func TestMocker_Serve(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "ServeMethod",
  "request": {"key": "input"},
  "response": {"key": "output"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	reqJSON, _ := json.Marshal(map[string]string{"key": "input"})
	resp, err := m.Serve(context.Background(), "TestService", "ServeMethod", reqJSON)
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	var respMap map[string]string
	if err := json.Unmarshal(resp, &respMap); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respMap["key"] != "output" {
		t.Errorf("expected response key 'output', got '%s'", respMap["key"])
	}
}

func TestMocker_ServeWithDelay(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "DelayedMethod",
  "request": null,
  "response": {"key": "delayed"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile,
		mocker.WithMinDelay(10*time.Millisecond),
		mocker.WithMaxDelay(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	start := time.Now()
	reqJSON := []byte(`{}`)
	_, err = m.Serve(context.Background(), "TestService", "DelayedMethod", reqJSON)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	if elapsed < 10*time.Millisecond {
		t.Errorf("expected delay of at least 10ms, got %v", elapsed)
	}
}

func TestOptions_WithDelay(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "Method",
  "request": null,
  "response": {"key": "test"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	// WithDelay sets both min and max to same value
	// Using WithDelayRange with slightly different values
	m, err := mocker.NewMocker(tmpFile, mocker.WithDelayRange(20*time.Millisecond, 21*time.Millisecond))
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	start := time.Now()
	_, err = m.Serve(context.Background(), "TestService", "Method", []byte(`{}`))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	if elapsed < 18*time.Millisecond {
		t.Errorf("expected delay of ~20ms, got %v", elapsed)
	}
}

func TestOptions_WithDelayRange(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "Method",
  "request": null,
  "response": {"key": "test"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile, mocker.WithDelayRange(10*time.Millisecond, 30*time.Millisecond))
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	start := time.Now()
	_, err = m.Serve(context.Background(), "TestService", "Method", []byte(`{}`))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	if elapsed < 10*time.Millisecond {
		t.Errorf("expected delay of at least 10ms, got %v", elapsed)
	}
}

func TestMocker_MultipleConfigs(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "GetValue",
  "request": {"msg": "first"},
  "response": {"resp": "response1"}
}
------##------
{
  "service": "TestService",
  "method": "GetValue",
  "request": {"msg": "second"},
  "response": {"resp": "response2"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	// Test first config
	req1 := &example.EchoRequest{Msg: "first"}
	reply1 := &example.EchoResponse{}

	if err := m.Invoke(context.Background(), "/package.TestService/GetValue", req1, reply1); err != nil {
		t.Fatalf("first Invoke failed: %v", err)
	}

	if reply1.Resp != "response1" {
		t.Errorf("expected 'response1', got '%s'", reply1.Resp)
	}

	// Test second config
	req2 := &example.EchoRequest{Msg: "second"}
	reply2 := &example.EchoResponse{}

	if err := m.Invoke(context.Background(), "/package.TestService/GetValue", req2, reply2); err != nil {
		t.Fatalf("second Invoke failed: %v", err)
	}

	if reply2.Resp != "response2" {
		t.Errorf("expected 'response2', got '%s'", reply2.Resp)
	}
}

func TestMockingInterceptor_EmptyPath(t *testing.T) {
	interceptor := mocker.MockingInterceptor("")

	// Should return passthrough interceptor
	req := &wrapperspb.StringValue{Value: "test"}
	reply := &wrapperspb.StringValue{}

	invokerCalled := false
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invokerCalled = true
		reply.(*wrapperspb.StringValue).Value = "invoked"
		return nil
	}

	err := interceptor(context.Background(), "/test", req, reply, nil, invoker)
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}

	if !invokerCalled {
		t.Error("expected invoker to be called")
	}

	if reply.Value != "invoked" {
		t.Errorf("expected reply.Value = 'invoked', got '%s'", reply.Value)
	}
}

func TestCloseCapture(t *testing.T) {
	// CloseCapture should be safe to call even when nothing is open
	err := mocker.CloseCapture()
	if err != nil {
		t.Fatalf("CloseCapture failed: %v", err)
	}

	// Calling again should also be safe
	err = mocker.CloseCapture()
	if err != nil {
		t.Fatalf("second CloseCapture failed: %v", err)
	}
}

func TestGetSvcMethod_Various(t *testing.T) {
	// This tests the getSvcMethod function indirectly through Invoke errors
	content := `{
  "service": "TestService",
  "method": "Method",
  "request": null,
  "response": {"resp": "result"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	// Test various method formats
	testCases := []struct {
		name        string
		method      string
		shouldMatch bool
	}{
		{"standard format", "/package.TestService/Method", true},
		{"different package", "/other.TestService/Method", true},
		{"wrong method", "/package.TestService/Wrong", false},
		{"wrong service", "/package.WrongService/Method", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &example.EchoRequest{}
			reply := &example.EchoResponse{}

			err := m.Invoke(context.Background(), tc.method, req, reply)
			if tc.shouldMatch && err != nil {
				t.Errorf("expected match, got error: %v", err)
			}
			if !tc.shouldMatch && err == nil {
				t.Error("expected error, got match")
			}
		})
	}
}

// Helper function to create a temporary file with content
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "mock_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write temp file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Mock request/reply for non-proto.Message test
type nonProtoType struct {
	Value string
}

func TestMocker_Invoke_NonProtoRequest(t *testing.T) {
	content := `{
  "service": "TestService",
  "method": "Method",
  "request": null,
  "response": {"resp": "result"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	// Use non-proto type
	req := &nonProtoType{Value: "test"}
	reply := &example.EchoResponse{}

	err = m.Invoke(context.Background(), "/package.TestService/Method", req, reply)
	if err == nil {
		t.Fatal("expected error for non-proto request")
	}

	if !strings.Contains(err.Error(), "not a proto.Message") {
		t.Errorf("expected 'not a proto.Message' error, got '%s'", err.Error())
	}
}

// Ensure interfaces are implemented
var (
	_ mocker.Mocker     = (*mockMocker)(nil)
	_ mocker.MockServer = (*mockMocker)(nil)
	_ mocker.MockClient = (*mockMocker)(nil)
)

type mockMocker struct{}

func (m *mockMocker) Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error) {
	return nil, nil
}

func (m *mockMocker) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return nil
}

func (m *mockMocker) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

// Test that Config struct can be marshaled/unmarshaled correctly
func TestConfig_JSON(t *testing.T) {
	cfg := mocker.Config{
		Service:  "TestService",
		Method:   "TestMethod",
		Request:  map[string]interface{}{"key": "value"},
		Response: map[string]interface{}{"result": "success"},
		Error:    "test error",
		Ignore:   []string{"field1", "field2"},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var decoded mocker.Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if decoded.Service != cfg.Service {
		t.Errorf("Service mismatch: expected %s, got %s", cfg.Service, decoded.Service)
	}

	if decoded.Method != cfg.Method {
		t.Errorf("Method mismatch: expected %s, got %s", cfg.Method, decoded.Method)
	}

	if decoded.Error != cfg.Error {
		t.Errorf("Error mismatch: expected %s, got %s", cfg.Error, decoded.Error)
	}

	if len(decoded.Ignore) != len(cfg.Ignore) {
		t.Errorf("Ignore length mismatch: expected %d, got %d", len(cfg.Ignore), len(decoded.Ignore))
	}
}

// Ensure proto.Message constraint works
func TestProtoMessage(t *testing.T) {
	var _ proto.Message = &wrapperspb.StringValue{}
	var _ proto.Message = &emptypb.Empty{}
	var _ proto.Message = &example.EchoRequest{}
	var _ proto.Message = &example.EchoResponse{}
}

// Test error types are exported and usable
func TestErrorTypes(t *testing.T) {
	_ = mocker.ErrNoMatchingRequest
	_ = mocker.ErrStreamingNotSupported
	_ = mocker.ErrRequestNotProtoMessage
	_ = mocker.ErrResponseNotProtoMessage
}

// Benchmark test for Invoke
func BenchmarkMocker_Invoke(b *testing.B) {
	content := `{
  "service": "TestService",
  "method": "Method",
  "request": null,
  "response": {"resp": "result"}
}`

	tmpFile, err := os.CreateTemp("", "bench_mock_*.json")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		b.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	m, err := mocker.NewMocker(tmpFile.Name())
	if err != nil {
		b.Fatalf("NewMocker failed: %v", err)
	}

	req := &example.EchoRequest{Msg: "test"}
	reply := &example.EchoResponse{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Invoke(context.Background(), "/package.TestService/Method", req, reply)
	}
}

// Benchmark test for Serve
func BenchmarkMocker_Serve(b *testing.B) {
	content := `{
  "service": "TestService",
  "method": "Method",
  "request": null,
  "response": {"key": "value"}
}`

	tmpFile, err := os.CreateTemp("", "bench_mock_*.json")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		b.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	m, err := mocker.NewMocker(tmpFile.Name())
	if err != nil {
		b.Fatalf("NewMocker failed: %v", err)
	}

	reqJSON := []byte(`{}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Serve(context.Background(), "TestService", "Method", reqJSON)
	}
}

// Test example package integration
func TestExampleServiceMock(t *testing.T) {
	content := `{
  "service": "ExampleService",
  "method": "Echo",
  "request": {"msg": "hello"},
  "response": {"resp": "hello back"}
}`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	m, err := mocker.NewMocker(tmpFile)
	if err != nil {
		t.Fatalf("NewMocker failed: %v", err)
	}

	// Use the example service client interface
	client := example.NewExampleServiceClient(m)

	resp, err := client.Echo(context.Background(), &example.EchoRequest{Msg: "hello"})
	if err != nil {
		t.Fatalf("Echo failed: %v", err)
	}

	if resp.Resp != "hello back" {
		t.Errorf("expected 'hello back', got '%s'", resp.Resp)
	}
}

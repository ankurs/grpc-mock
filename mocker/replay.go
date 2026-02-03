package mocker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	sep = "\n------##------\n"
)

// nullJSON represents a JSON null value for comparison.
var nullJSON = []byte("null")

// mockClient implements the Mocker interface for replaying recorded gRPC interactions.
type mockClient struct {
	cfg     map[string][]Config
	options options
	logger  *slog.Logger
}

// lookupKey creates a key for looking up configurations by service and method.
func lookupKey(svc, met string) string {
	return svc + "/" + met
}

// Invoke performs a unary RPC and returns after the response is received
// into reply. It matches the request against recorded configurations and
// returns the corresponding response.
func (m *mockClient) Invoke(ctx context.Context, infoMethod string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	argsMsg, ok := args.(proto.Message)
	if !ok {
		return fmt.Errorf("%w: got %T", ErrRequestNotProtoMessage, args)
	}

	req, err := protojson.Marshal(argsMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := m.matchRequest(ctx, infoMethod, req)
	if err != nil {
		return err
	}

	// Check for null response
	if bytes.Equal(resp, nullJSON) {
		return nil
	}

	replyMsg, ok := reply.(proto.Message)
	if !ok {
		return fmt.Errorf("%w: got %T", ErrResponseNotProtoMessage, reply)
	}

	if err := protojson.Unmarshal(resp, replyMsg); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// matchRequest finds a matching configuration for the given request.
func (m *mockClient) matchRequest(ctx context.Context, infoMethod string, request []byte) ([]byte, error) {
	svc, met := getSvcMethod(infoMethod)
	key := lookupKey(svc, met)

	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request JSON: %w", err)
	}

	list, ok := m.cfg[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoMatchingRequest, key)
	}

	for _, l := range list {
		opt := m.createIgnoreOption(l.Ignore)

		if l.Request == nil || cmp.Equal(l.Request, req, opt) {
			if l.Error != "" {
				return nil, errors.New(l.Error)
			}

			d, err := json.Marshal(l.Response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return d, nil
		}

		if m.logger != nil {
			m.logger.Debug("request mismatch",
				slog.String("service", svc),
				slog.String("method", met),
				slog.String("diff", cmp.Diff(l.Request, req, opt)))
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNoMatchingRequest, key)
}

// createIgnoreOption creates a cmp.Option that ignores specified paths.
func (m *mockClient) createIgnoreOption(ignorePaths []string) cmp.Option {
	return cmp.FilterPath(func(path cmp.Path) bool {
		s := path.GoString()
		// Normalize path format for comparison
		s = strings.ReplaceAll(s, "root[\"", "root.")
		s = strings.ReplaceAll(s, "[\"", "")
		s = strings.ReplaceAll(s, "\"]", "")
		s = strings.ReplaceAll(s, "([]interface {})", "")
		s = strings.ReplaceAll(s, "(map[string]interface {})", "")

		for _, p := range ignorePaths {
			if p == s {
				return true
			}
		}
		return false
	}, cmp.Ignore())
}

// Serve handles a mock server request, optionally applying configured delays.
func (m *mockClient) Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error) {
	m.applyDelay()
	return m.matchRequest(ctx, lookupKey(svc, method), request)
}

// applyDelay applies a random delay between MinDelay and MaxDelay if configured.
func (m *mockClient) applyDelay() {
	if m.options.MaxDelay <= m.options.MinDelay {
		return
	}

	minMs := m.options.MinDelay.Milliseconds()
	maxMs := m.options.MaxDelay.Milliseconds()

	if maxMs > minMs {
		r := rand.Int64N(maxMs-minMs) + minMs
		time.Sleep(time.Duration(r) * time.Millisecond)
	}
}

// NewStream begins a streaming RPC. This method returns an error as streaming
// RPCs are not yet supported in the mock client.
func (m *mockClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, ErrStreamingNotSupported
}

// NewMocker creates a new Mocker from a recorded interactions file.
// The file should contain JSON configurations separated by the standard separator.
func NewMocker(filePath string, opts ...Option) (Mocker, error) {
	return NewMockerWithLogger(filePath, slog.Default(), opts...)
}

// NewMockerWithLogger creates a new Mocker from a recorded interactions file,
// using the provided logger for debug output.
func NewMockerWithLogger(filePath string, logger *slog.Logger, opts ...Option) (Mocker, error) {
	// Resolve relative paths
	if !filepath.IsAbs(filePath) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		filePath = filepath.Join(dir, filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open mock file: %w", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	s.Split(scanSep)

	c := &mockClient{
		cfg:    make(map[string][]Config),
		logger: logger,
	}

	for s.Scan() {
		var cfg Config
		data := s.Bytes()
		if len(data) == 0 {
			continue
		}

		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}

		key := lookupKey(cfg.Service, cfg.Method)
		c.cfg[key] = append(c.cfg[key], cfg)

		if logger != nil {
			logger.Debug("loaded mock configuration",
				slog.String("key", key),
				slog.String("service", cfg.Service),
				slog.String("method", cfg.Method))
		}
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("error reading mock file: %w", err)
	}

	for _, opt := range opts {
		opt(&c.options)
	}

	return c, nil
}

// scanSep is a split function for bufio.Scanner that splits on the separator.
func scanSep(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, []byte(sep)); i >= 0 {
		return i + len(sep), dropCR(data[0:i]), nil
	}

	if atEOF {
		return len(data), dropCR(data), nil
	}

	return 0, nil, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

// Error definitions for the mocker package.
var (
	// ErrNoMatchingRequest is returned when no matching request configuration is found.
	ErrNoMatchingRequest = errors.New("no matching request found")

	// ErrStreamingNotSupported is returned when a streaming RPC is attempted.
	ErrStreamingNotSupported = errors.New("streaming RPCs are not supported")
)

// Package mocker provides gRPC mocking capabilities for recording and replaying
// gRPC interactions. It supports capturing client requests/responses and replaying
// them for testing purposes.
package mocker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Config represents a single recorded gRPC interaction containing the request,
// response, service name, method name, any error that occurred, and fields to ignore
// during request matching.
type Config struct {
	Request  map[string]interface{} `json:"request,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"`
	Service  string                 `json:"service"`
	Method   string                 `json:"method"`
	Error    string                 `json:"error,omitempty"`
	Ignore   []string               `json:"ignore,omitempty"`
}

// captureState holds the state for the capture interceptor.
type captureState struct {
	file   *os.File
	ch     chan Config
	once   sync.Once
	closed bool
	mu     sync.Mutex
}

var globalCapture = &captureState{}

// MockingInterceptor returns a gRPC unary client interceptor that records
// all gRPC interactions to the specified file path. If filePath is empty,
// a passthrough interceptor is returned that doesn't record anything.
//
// The recorded interactions can be replayed using NewMocker.
func MockingInterceptor(filePath string) grpc.UnaryClientInterceptor {
	return MockingInterceptorWithLogger(filePath, slog.Default())
}

// MockingInterceptorWithLogger returns a gRPC unary client interceptor that records
// all gRPC interactions to the specified file path, using the provided logger.
func MockingInterceptorWithLogger(filePath string, logger *slog.Logger) grpc.UnaryClientInterceptor {
	if filePath == "" {
		return passthroughInterceptor
	}

	globalCapture.mu.Lock()
	defer globalCapture.mu.Unlock()

	if globalCapture.file == nil {
		globalCapture.once.Do(func() {
			var err error
			globalCapture.file, err = os.Create(filePath)
			if err != nil {
				logger.Error("could not create capture file",
					slog.String("path", filePath),
					slog.String("error", err.Error()))
				return
			}
			globalCapture.ch = make(chan Config, 100) // buffered channel for performance
			go writer(globalCapture.file, globalCapture.ch, logger)
		})
	}

	if globalCapture.file == nil {
		return passthroughInterceptor
	}

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		globalCapture.mu.Lock()
		if !globalCapture.closed && globalCapture.ch != nil {
			go writeCapture(globalCapture.ch, method, req, reply, err, logger)
		}
		globalCapture.mu.Unlock()
		return err
	}
}

// CloseCapture closes the capture file and stops recording. This should be called
// when done recording to ensure all data is flushed to disk.
func CloseCapture() error {
	globalCapture.mu.Lock()
	defer globalCapture.mu.Unlock()

	if globalCapture.closed {
		return nil
	}

	globalCapture.closed = true

	if globalCapture.ch != nil {
		close(globalCapture.ch)
		globalCapture.ch = nil
	}

	if globalCapture.file != nil {
		err := globalCapture.file.Close()
		globalCapture.file = nil
		return err
	}

	return nil
}

// passthroughInterceptor is a no-op interceptor that just calls the underlying invoker.
func passthroughInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return invoker(ctx, method, req, reply, cc, opts...)
}

// writer writes configs to the file as they come in through the channel.
func writer(f *os.File, ch <-chan Config, logger *slog.Logger) {
	w := bufio.NewWriter(f)
	defer func() {
		if err := w.Flush(); err != nil {
			logger.Error("failed to flush capture buffer", slog.String("error", err.Error()))
		}
	}()

	for c := range ch {
		cData, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			logger.Error("failed to marshal config",
				slog.String("service", c.Service),
				slog.String("method", c.Method),
				slog.String("error", err.Error()))
			continue
		}

		if _, err := w.WriteString(string(cData) + sep); err != nil {
			logger.Error("failed to write to capture file", slog.String("error", err.Error()))
			continue
		}

		if err := w.Flush(); err != nil {
			logger.Error("failed to flush capture buffer", slog.String("error", err.Error()))
		}
	}
}

// getSvcMethod extracts the service and method names from a gRPC method string.
// The method string format is typically "/package.Service/Method".
func getSvcMethod(infoMethod string) (string, string) {
	vals := strings.Split(infoMethod, ".")
	if len(vals) == 0 {
		return "unknown", "unknown"
	}

	info := strings.Split(vals[len(vals)-1], "/")
	if len(info) != 2 {
		return "unknown", "unknown"
	}

	return info[0], info[1]
}

// writeCapture writes a captured request/response to the channel.
func writeCapture(ch chan<- Config, infoMethod string, req, resp interface{}, err error, logger *slog.Logger) {
	svc, meth := getSvcMethod(infoMethod)

	c := Config{
		Service:  svc,
		Method:   meth,
		Request:  make(map[string]interface{}),
		Response: make(map[string]interface{}),
	}

	// Safely marshal request
	if reqMsg, ok := req.(proto.Message); ok {
		reqData, marshalErr := protojson.Marshal(reqMsg)
		if marshalErr != nil {
			logger.Warn("failed to marshal request",
				slog.String("service", svc),
				slog.String("method", meth),
				slog.String("error", marshalErr.Error()))
		} else {
			if unmarshalErr := json.Unmarshal(reqData, &c.Request); unmarshalErr != nil {
				logger.Warn("failed to unmarshal request JSON",
					slog.String("service", svc),
					slog.String("method", meth),
					slog.String("error", unmarshalErr.Error()))
			}
		}
	} else {
		logger.Warn("request is not a proto.Message",
			slog.String("service", svc),
			slog.String("method", meth),
			slog.String("type", fmt.Sprintf("%T", req)))
	}

	// Safely marshal response
	if respMsg, ok := resp.(proto.Message); ok {
		respData, marshalErr := protojson.Marshal(respMsg)
		if marshalErr != nil {
			logger.Warn("failed to marshal response",
				slog.String("service", svc),
				slog.String("method", meth),
				slog.String("error", marshalErr.Error()))
		} else {
			if unmarshalErr := json.Unmarshal(respData, &c.Response); unmarshalErr != nil {
				logger.Warn("failed to unmarshal response JSON",
					slog.String("service", svc),
					slog.String("method", meth),
					slog.String("error", unmarshalErr.Error()))
			}
		}
	} else {
		logger.Warn("response is not a proto.Message",
			slog.String("service", svc),
			slog.String("method", meth),
			slog.String("type", fmt.Sprintf("%T", resp)))
	}

	if err != nil {
		c.Error = err.Error()
	}

	ch <- c
}

// ErrRequestNotProtoMessage is returned when the request is not a proto.Message.
var ErrRequestNotProtoMessage = errors.New("request is not a proto.Message")

// ErrResponseNotProtoMessage is returned when the response is not a proto.Message.
var ErrResponseNotProtoMessage = errors.New("response is not a proto.Message")

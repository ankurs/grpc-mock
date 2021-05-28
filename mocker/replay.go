package mocker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"

	"github.com/go-coldbrew/log"
	"google.golang.org/grpc"
)

const (
	sep = "\n------##------\n"
)

type mockClient struct {
	cfg map[string][]Config
}

func lookupKey(svc, met string) string {
	return svc + ":" + met
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (m *mockClient) Invoke(ctx context.Context, infoMethod string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	req, _ := json.Marshal(args)
	resp, err := m.matchRequest(ctx, infoMethod, req)
	if err != nil {
		return err
	}
	return json.Unmarshal(resp, reply)
}

func (m *mockClient) matchRequest(ctx context.Context, infoMethod string, request []byte) ([]byte, error) {
	svc, met := getSvcMethod(infoMethod)
	key := lookupKey(svc, met)
	req := make(map[string]interface{})
	json.Unmarshal(request, &req)
	//log.Info(ctx, "msg", "checking for", "key", key, "info", infoMethod)
	if list, ok := m.cfg[key]; ok {
		//log.Info(ctx, "msg", "found", "key", key, "info", infoMethod)
		for _, l := range list {
			if reflect.DeepEqual(l.Request, req) {
				log.Info(ctx, "Yay DeepEqual", "req", req, "req", l.Request)
			}
			if l.Error != "" {
				return []byte{}, errors.New(l.Error)
			}
			d, _ := json.Marshal(l.Response)
			return d, nil
		}
	}
	return []byte{}, errors.New("could not find requst")
}

func (m *mockClient) Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error) {
	return m.matchRequest(ctx, method, request)
}

// NewStream begins a streaming RPC.
func (m *mockClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("not implemented") // TODO: Implement
}

func NewMocker(filePath string) (Mocker, error) {
	if !strings.HasPrefix(filePath, string(os.PathSeparator)) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		filePath = dir + string(os.PathSeparator) + filePath
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(f)
	s.Split(scanSep)

	c := &mockClient{
		cfg: make(map[string][]Config),
	}

	for s.Scan() {
		cfg := Config{}
		data := s.Bytes()
		err := json.Unmarshal(data, &cfg)
		if err != nil {
			return nil, err
		}
		key := lookupKey(cfg.Service, cfg.Method)
		c.cfg[key] = append(c.cfg[key], cfg)
		//log.Info(context.Background(), "loaded", key, "data", cfg)
	}
	return c, nil
}

func scanSep(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte(sep)); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(sep), dropCR(data[0:i]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

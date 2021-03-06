package mocker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-coldbrew/log"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	sep = "\n------##------\n"
)

type mockClient struct {
	cfg     map[string][]Config
	options options
}

func lookupKey(svc, met string) string {
	return svc + "/" + met
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (m *mockClient) Invoke(ctx context.Context, infoMethod string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	req, err := protojson.Marshal(args.(proto.Message))
	if err != nil {
		return err
	}
	resp, err := m.matchRequest(ctx, infoMethod, req)
	if err != nil {
		return err
	}
	// []byte{110, 117, 108, 108} == "null"
	if bytes.Compare(resp, []byte{110, 117, 108, 108}) == 0 {
		return nil
	}
	return protojson.Unmarshal(resp, reply.(proto.Message))
}

func (m *mockClient) matchRequest(ctx context.Context, infoMethod string, request []byte) ([]byte, error) {
	svc, met := getSvcMethod(infoMethod)
	key := lookupKey(svc, met)
	req := make(map[string]interface{})
	err := json.Unmarshal(request, &req)
	if err != nil {
		return []byte{}, err
	}
	//log.Info(ctx, "msg", "checking for", "key", key, "info", infoMethod)
	if list, ok := m.cfg[key]; ok {
		//log.Info(ctx, "msg", "found", "key", key, "info", infoMethod)
		for _, l := range list {
			ignore := l.Ignore
			opt := cmp.FilterPath(func(path cmp.Path) bool {
				s := path.GoString()
				s = strings.ReplaceAll(s, "root[\"", "root.")
				s = strings.ReplaceAll(s, "[\"", "")
				s = strings.ReplaceAll(s, "\"]", "")
				s = strings.ReplaceAll(s, "([]interface {})", "")
				s = strings.ReplaceAll(s, "(map[string]interface {})", "")
				//log.Info(ctx, "msg", "matching path", "s", s, "ignore", ignore)
				for _, p := range ignore {
					if p == s {
						//log.Info(ctx, "msg", "ignoring path", "path", s)
						return true
					}
				}
				return false
			}, cmp.Ignore())
			if l.Request == nil || cmp.Equal(l.Request, req, opt) {
				if l.Error != "" {
					return []byte{}, errors.New(l.Error)
				}
				d, _ := json.Marshal(l.Response)
				//log.Info(ctx, "returning", string(d))
				return d, nil
			} else {
				log.Error(ctx, "diff", cmp.Diff(l.Request, req, opt))
			}
			//log.Info(ctx, "not found", key, "config", l.Request, "req", req)
		}
	}
	return []byte{}, errors.New("could not find requst for: " + key)
}

func (m *mockClient) Serve(ctx context.Context, svc, method string, request []byte) ([]byte, error) {
	if m.options.MaxDelay > m.options.MinDelay {
		// do request delay
		min := int64(m.options.MinDelay / time.Millisecond)
		max := int64(m.options.MaxDelay / time.Millisecond)
		if max > min {
			r := rand.Int63n(max-min) + min
			time.Sleep(time.Duration(r) * time.Millisecond)
		}
	}
	return m.matchRequest(ctx, lookupKey(svc, method), request)
}

// NewStream begins a streaming RPC.
func (m *mockClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("not implemented") // TODO: Implement
}

func NewMocker(filePath string, opts ...option) (Mocker, error) {
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
		log.Info(context.Background(), "loaded", key, "data", cfg)
	}
	for _, opt := range opts {
		opt(&c.options)
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

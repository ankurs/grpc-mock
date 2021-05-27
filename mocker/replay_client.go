package mocker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
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
	svc, met := getSvcMethod(infoMethod)
	key := lookupKey(svc, met)

	//log.Info(ctx, "msg", "checking for", "key", key, "info", infoMethod)
	if list, ok := m.cfg[key]; ok {
		//log.Info(ctx, "msg", "found", "key", key, "info", infoMethod)
		for _, l := range list {
			if l.Error != "" {
				return errors.New(l.Error)
			}
			if reply == nil {
				return nil
			}
			if r, ok := reply.(proto.Message); ok {
				d, _ := json.Marshal(l.Response)
				err := json.Unmarshal(d, r)
				//log.Info(ctx, "req", args, "reply", r, "d", len(d), "err", err, "type", reflect.TypeOf(reply), "kind", reflect.TypeOf(reply).Kind())
				if err != nil {
					continue
				}
				return nil
			}
		}
	}
	return errors.New("could not find a mocked request")
}

// NewStream begins a streaming RPC.
func (m *mockClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("not implemented") // TODO: Implement
}

func NewMockClient(filePath string) (MockClient, error) {
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
	s.Split(bufio.ScanLines)

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

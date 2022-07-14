package mocker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-coldbrew/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Config struct {
	Request  map[string]interface{} `json:"request,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"`
	Service  string                 `json:"service"`
	Method   string                 `json:"method"`
	Error    string                 `json:"error"`
	Ignore   []string               `json:"ignore"`
}

var (
	f    *os.File
	once sync.Once
)

func MockingInterceptor(filePath string) grpc.UnaryClientInterceptor {
	if f == nil && filePath != "" {
		once.Do(func() {
			var err error
			f, err = os.Create(filePath)
			if err != nil {
				log.Error(context.Background(), "err", "could not create file "+filePath)
			}
		})
	}
	if f == nil {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}
	ch := make(chan Config)
	go writer(f, ch)
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		go write(ch, method, req, reply, err) // write it to the file
		return err
	}
}

func writer(f *os.File, ch <-chan Config) {
	w := bufio.NewWriter(f)
	for c := range ch {
		cData, _ := json.MarshalIndent(c, "", "  ")
		_, err := w.WriteString(string(cData) + sep)
		if err != nil {
			log.Error(context.Background(), err)
		}
		w.Flush() // force flush
	}
}

func getSvcMethod(infoMethod string) (string, string) {
	vals := strings.Split(infoMethod, ".")
	info := strings.Split(vals[len(vals)-1], "/")
	if len(info) != 2 {
		return "error", "error"
	}
	return info[0], info[1]
}

func write(ch chan<- Config, infoMethod string, req, resp interface{}, err error) {
	svc, meth := getSvcMethod(infoMethod)

	c := Config{
		Service:  svc,
		Method:   meth,
		Request:  make(map[string]interface{}),
		Response: make(map[string]interface{}),
	}

	reqData, _ := protojson.Marshal(req.(proto.Message))
	json.Unmarshal(reqData, &c.Request)

	respData, _ := protojson.Marshal(resp.(proto.Message))
	json.Unmarshal(respData, &c.Response)

	if err != nil {
		c.Error = err.Error()
	}
	ch <- c
}

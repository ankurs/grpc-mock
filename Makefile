default: generate

install:
	go install ./protoc-gen-grpc-mock

generate: install
	buf generate

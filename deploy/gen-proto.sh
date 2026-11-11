#!/bin/sh
# Runs inside the gotest container (see `make gen`). Generates Go stubs from api/proto into gen/go.
set -e
apt-get update -qq
apt-get install -y -qq protobuf-compiler >/dev/null
export GOBIN=/tmp/bin
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.1
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
export PATH="/tmp/bin:$PATH"
mkdir -p gen/go
protoc -I api/proto \
  --go_out=gen/go --go_opt=paths=source_relative \
  --go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
  $(find api/proto -name '*.proto')

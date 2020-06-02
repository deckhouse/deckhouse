#!/bin/bash

set -e

if ! [[ "$0" =~ "scripts/gen_proto.sh" ]]; then
  echo "must be run from repository root"
  exit 255
fi

if ! [[ $(protoc --version) =~ "3.12.0" ]]; then
  echo "could not find protoc 3.12.0, is it installed + in PATH?"
  exit 255
fi

echo "Installing gogo/protobuf..."
GOGOPROTO_ROOT="$GOPATH/src/github.com/gogo/protobuf"
GO111MODULE="off" go get -v github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto,protoc-gen-gofast}
GO111MODULE="off" go get -v golang.org/x/tools/cmd/goimports

ln -s $GOPATH/bin/protoc-gen-gofast /usr/local/bin/protoc-gen-gogofast || true

echo "Generating message"
protoc --proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. --gogofast_out=. ./pkg/proto/*.proto

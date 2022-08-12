package proto

//go:generate /bin/sh -c "protoc -I. -I=$GOPATH/src -I=$GOPATH/src/github.com/gogo/protobuf/protobuf --gogofaster_out=. message.proto"

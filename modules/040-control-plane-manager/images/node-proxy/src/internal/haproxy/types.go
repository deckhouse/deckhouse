package haproxy

import (
	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/runtime"
)

type Client struct {
	client        clientnative.HAProxyClient
	runtimeClient runtime.Runtime
}

type Server struct {
	Name    string
	Address string
	Port    int64
}

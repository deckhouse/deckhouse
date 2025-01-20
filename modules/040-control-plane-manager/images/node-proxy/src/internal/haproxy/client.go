package haproxy

import (
	"context"

	runtime_api "github.com/haproxytech/client-native/v6/runtime"
	runtime_options "github.com/haproxytech/client-native/v6/runtime/options"
	log "github.com/sirupsen/logrus"

	"node-proxy-sidecar/internal/config"
)

func (c *Client) NewClient(cfg config.Config) *Client {
	opt := runtime_options.MasterSocket(cfg.SocketPath)

	runtimeClient, err := runtime_api.New(context.Background(), opt)
	if err != nil {
		log.Fatalf("error setting up runtime client: %s", err.Error())
	}
	c.client = runtimeClient
	return c
}

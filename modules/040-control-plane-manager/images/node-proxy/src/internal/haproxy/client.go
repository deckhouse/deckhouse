package haproxy

import (
	"context"

	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/configuration"
	cfg_opt "github.com/haproxytech/client-native/v6/configuration/options"
	"github.com/haproxytech/client-native/v6/options"
	runtimeapi "github.com/haproxytech/client-native/v6/runtime"
	runtimeoptions "github.com/haproxytech/client-native/v6/runtime/options"
	log "github.com/sirupsen/logrus"

	"node-proxy-sidecar/internal/config"
)

func (c *Client) NewClient(cfg config.Config) (*Client, error) {
	runtimeClient, err := runtimeapi.New(context.Background(), runtimeoptions.Socket(cfg.SocketPath))
	if err != nil {
		log.Fatalf("error setting up runtime client: %s", err.Error())
	}
	confClient, err := configuration.New(context.Background(),
		cfg_opt.ConfigurationFile(cfg.HAProxyConfigurationFile),
		cfg_opt.UseModelsValidation,
		cfg_opt.HAProxyBin(cfg.HAProxyHAProxyBin),
		cfg_opt.TransactionsDir(cfg.HAProxyTransactionsDir),
	)
	if err != nil {
		return nil, err
	}

	opt := []options.Option{
		options.Configuration(confClient),
		options.Runtime(runtimeClient),
	}

	cnHAProxyClient, err := clientnative.New(context.Background(), opt...)
	if err != nil {
		log.Fatalf("Error initializing configuration client: %v", err)
	}

	c.client = cnHAProxyClient
	return c, nil
}

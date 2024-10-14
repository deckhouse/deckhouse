// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"registry-packages-proxy/internal/app"
	"registry-packages-proxy/internal/credentials"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

func main() {

	config, err := app.InitFlags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	logger := app.InitLogger(config)

	// init listener. Another listener is used in dhctl
	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		logger.Fatal(err)
	}
	defer listener.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// init kube clients
	client, err := app.InitClient(config)
	if err != nil {
		logger.Fatal(err)
	}

	dynamicClient, err := app.InitDynamicClient(config)
	if err != nil {
		logger.Fatal(err)
	}

	// watch resources
	watcher := credentials.NewWatcher(client, dynamicClient, time.Hour, logger)
	go watcher.Watch(ctx)

	// init cache
	cache := app.NewCache(logger, config, app.RegisterMetrics())
	if !config.DisableCache {
		go cache.Reconcile(ctx)
	}
	// init http server
	server := app.BuildServer()

	var opts []proxy.ProxyOption
	if !config.DisableCache {
		opts = append(opts, proxy.WithCache(cache))
	}
	rp := proxy.NewProxy(server, listener, watcher, logger, &registry.DefaultClient{}, opts...)
	if err != nil {
		logger.Fatal(err)
	}

	go rp.Serve()

	<-ctx.Done()

	rp.Stop()
}

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
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"

	"registry-packages-proxy/internal/app"
	"registry-packages-proxy/internal/credentials"
)

func main() {

	config, err := app.InitFlags()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cacheMetrics := app.RegisterMetrics()

	logger := app.InitLogger(config)
	client := app.InitClient(config, logger)
	dynamicClient := app.InitDynamicClient(config, logger)

	cache, err := app.NewCache(ctx, config, cacheMetrics)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if cache == nil {
		logger.Info("Cache is disabled")
	}
	defer cache.Close()

	watcher := credentials.NewWatcher(client, dynamicClient, time.Hour, logger)

	go watcher.Watch(ctx)

	server := app.BuildServer(config)
	registryProxy, err := proxy.NewProxy(server, watcher, proxy.Options{
		Cache:  cache,
		Logger: logger,
	})

	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	go registryProxy.Serve()
	<-ctx.Done()
	registryProxy.StopServe()
}

/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"geodownloader"
)

const (
	LeaseLockName      = "geoproxy-leader"
	LeaseLockNamespace = "d8-ingress-nginx"
	secretName         = "geoip-license-editions"
	secretNamespace    = "d8-ingress-nginx"
)

var (
	serverPort     string
	prometheusPort string
)

func main() {
	flag.StringVar(&serverPort, "server-port", "127.0.0.1:8080", "server port")
	flag.StringVar(&prometheusPort, "prometheus-port", "127.0.0.1:9090", "prometheus port")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	leader := geodownloader.NewLeaderElection(LeaseLockName, LeaseLockNamespace)
	watcher := geodownloader.NewGeoUpdaterSecret()
	cfg := geodownloader.NewConfig()
	downloader := geodownloader.NewDownloader(watcher, leader)

	// start leader election
	go func() {
		if err := leader.AcquireLeaderElection(ctx); err != nil {
			log.Error(fmt.Sprintf("leader.AcquireLeaderElection: %v", err))
			stop()
		}
	}()

	server := geodownloader.NewServer()
	// http server
	go func() {
		if err := server.Start(ctx, serverPort, prometheusPort); err != nil {
			log.Error(fmt.Sprintf("Failed to start server: %v", err))
			stop()
		}
	}()

	// Start secret watcher
	go func() {
		if err := watcher.RunWatcher(ctx, secretName, secretNamespace); err != nil {
			log.Error(fmt.Sprintf("watcher.RunWatcher: %v", err))
			stop()
		}
	}()

	// interval update
	go func() {
		log.Info(fmt.Sprintf("Start cron in %s interval", cfg.MaxmindIntervalUpdate))

		if err := downloader.Download(ctx, geodownloader.PathDb, cfg, false); err != nil {
			log.Error(fmt.Sprintf("Failed to download db: %v", err))
		}

		ticker := time.NewTicker(cfg.MaxmindIntervalUpdate)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := downloader.Download(ctx, geodownloader.PathDb, cfg, false); err != nil {
					log.Error(fmt.Sprintf("Failed to download db: %v", err))
				}
			case <-watcher.Updated:
				if err := downloader.Download(ctx, geodownloader.PathDb, cfg, true); err != nil {
					log.Error(fmt.Sprintf("Failed to download db: %v", err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
}

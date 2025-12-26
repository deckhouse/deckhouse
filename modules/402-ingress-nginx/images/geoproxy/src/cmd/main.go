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
	"geodownloader"
	"os/signal"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
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
	grpcPort       string
)

func main() {
	klog.InitFlags(nil)

	flag.StringVar(&serverPort, "server-port", "127.0.0.1:8080", "server port")
	flag.StringVar(&prometheusPort, "prometheus-port", "127.0.0.1:9090", "prometheus port")
	flag.StringVar(&grpcPort, "grpc-port", ":50051", "grpc server port")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	leader := geodownloader.NewLeaderElection(LeaseLockName, LeaseLockNamespace)
	watcher := geodownloader.NewGeoUpdaterSecret()
	cfg := geodownloader.NewConfig()
	downloader := geodownloader.NewDownloader(watcher, leader)

	geodb, err := geodownloader.NewGeoDB(geodownloader.PathRawMMDB)
	if err != nil {
		log.Error(fmt.Sprintf("failed init GeoDB service: %v", err))
		return
	}
	defer geodb.Close()

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(fmt.Sprintf("Failed init fs notify: %v", err))
		return
	}
	defer fsWatcher.Close()

	grpcServer := geodownloader.NewGRPCServer(geodb)

	if err := fsWatcher.Add(geodownloader.PathRawMMDB); err != nil {
		log.Error(fmt.Sprintf("Failed init fs notify: %v", err))
		return
	}

	// fs notify triger reinit GeoDB if raw MMDB was changed
	go func() {
		for {
			select {
			case <-fsWatcher.Events:
				if err := geodb.Reload(geodownloader.PathRawMMDB); err != nil {
					log.Error(fmt.Sprintf("failed init GeoDB service: %v", err))
				}

			case err, ok := <-fsWatcher.Errors:
				if !ok {
					return
				}
				log.Error(fmt.Sprintf("fs watcher err: %v", err))

			case <-ctx.Done():
				return
			}
		}
	}()
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

	// Start GRPC Server
	go func() {
		if err := grpcServer.StartGRPCGeoIPService(grpcPort); err != nil {
			log.Error(fmt.Sprintf("Failed start GRPC server: %v", err))
			stop()
		}
	}()

	downloadAndReload := func(force bool) {
		if err := downloader.Download(ctx, geodownloader.PathDb, cfg, force); err != nil {
			log.Error(fmt.Sprintf("Failed to download db: %v", err))
			return
		}
		if err := geodb.Reload(geodownloader.PathRawMMDB); err != nil {
			log.Error(fmt.Sprintf("Failed to reload mmdb: %v", err))
			return
		}
	}

	// interval update
	go func() {
		log.Info(fmt.Sprintf("Start cron in %s interval", cfg.MaxmindIntervalUpdate))

		downloadAndReload(false)

		ticker := time.NewTicker(cfg.MaxmindIntervalUpdate)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				downloadAndReload(false)
			case <-watcher.Updated:
				downloadAndReload(true)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
}

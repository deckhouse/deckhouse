/*
Copyright 2026 Flant JSC

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
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/json"

	"github.com/deckhouse/deckhouse/pkg/log"

	apilb "kubernetes-api-proxy/internal/apiserver"
	"kubernetes-api-proxy/internal/app"
	"kubernetes-api-proxy/internal/config"
	uopts "kubernetes-api-proxy/internal/upstream"
	"kubernetes-api-proxy/pkg/kubernetes"
)

func main() {
	cfg := config.Parse()

	logger := log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithLevel(cfg.SLogLevel()),
		log.WithHandlerType(log.JSONHandlerType),
	)

	ctx, cancel := context.WithCancel(context.Background())

	{
		cfgJSON, _ := json.Marshal(cfg)
		logger.Debug("config", slog.String("config", string(cfgJSON)))
	}

	fallbackList, err := configureFallbackList(cfg, logger)
	if err != nil {
		logger.Error("failed to create fallback list", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := fallbackList.Start(ctx); err != nil {
		logger.Error("failed to start fallback list", slog.String("error", err.Error()))
		os.Exit(1)
	}

	configGetter := kubernetes.BuildGetter(cfg, fallbackList)

	mainUpstreamList, err := configureMainUpstreamList(cfg, logger, configGetter)
	if err != nil {
		logger.Error("failed to configure upstream list", slog.String("error", err.Error()))
		os.Exit(1)
	}

	lb, err := apilb.NewLoadBalancer(
		cfg.ListenAddress,
		cfg.ListenPort,
		logger,
		apilb.WithDialTimeout(cfg.DialTimeout),
		apilb.WithKeepAlivePeriod(cfg.KeepAlivePeriod),
		apilb.WithTCPUserTimeout(cfg.TCPUserTimeout),
		apilb.WithMainUpstreamList(mainUpstreamList),
		apilb.WithFallbackUpstreamList(fallbackList),
	)
	if err != nil {
		logger.Error("failed to create load balancer", slog.String("error", err.Error()))
		os.Exit(1)
	}

	srv := app.NewHealthServer(cfg.ProxyHealthListen, lb)
	go func() {
		if cfg.ProxyHealthListen == "" {
			return
		}

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server error", slog.String("error", err.Error()))
		}
	}()

	if err := lb.Start(); err != nil {
		logger.Error("failed to start load balancer", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("apiserver-lb started", slog.String("listen", lb.Endpoint()))

	go app.StartDiscovery(ctx, cfg, logger, mainUpstreamList, fallbackList)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)

	_ = srv.Shutdown(shutdownCtx)
	if err := lb.Shutdown(); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
		shutdownCancel()
		os.Exit(1)
	}

	shutdownCancel()
	logger.Info("stopped")
}

func configureMainUpstreamList(
	cfg config.Config,
	logger *log.Logger,
	kubernetesConfigGetter kubernetes.ClusterConfigGetter,
) (*uopts.List, error) {
	listOptions := []uopts.ListOption{
		uopts.WithHealthcheckInterval(cfg.HealthInterval),
		uopts.WithHealthcheckTimeout(cfg.HealthTimeout),
		uopts.WithKubernetesConfigGetter(kubernetesConfigGetter),
		uopts.WithLogger(logger),
	}
	if cfg.HealthJitter > 0 {
		listOptions = append(listOptions, uopts.WithHealthCheckJitter(cfg.HealthJitter))
	}

	mainUpstreamList, err := uopts.NewList([]*uopts.Upstream{}, listOptions...)
	if err != nil {
		return nil, err
	}

	return mainUpstreamList, nil
}

func configureFallbackList(
	cfg config.Config,
	logger *log.Logger,
) (*uopts.FallbackList, error) {
	opts := []uopts.FallbackListOption{
		uopts.WithFallbackLogger(logger),
	}
	if len(cfg.FallbackEndpoints) > 0 {
		opts = append(opts, uopts.WithUpstreamsFromArgs(cfg.FallbackEndpoints))
	}

	if cfg.FallbackFile != "" {
		opts = append(opts, uopts.WithFileWatcher(cfg.FallbackFile))
	}

	return uopts.NewFallbackList(
		opts...,
	)
}

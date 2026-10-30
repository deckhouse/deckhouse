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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/pkg/log"

	apilb "kubernetes-api-proxy/internal/apiserver"
	"kubernetes-api-proxy/internal/app"
	"kubernetes-api-proxy/internal/config"
	uopts "kubernetes-api-proxy/internal/upstream"
	"kubernetes-api-proxy/pkg/kubernetes"
	"kubernetes-api-proxy/pkg/utils"
)

func main() {
	cfg := config.Parse()

	logger := log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithLevel(cfg.SLogLevel()),
		log.WithHandlerType(log.JSONHandlerType),
	)
	// client-go writes through klog. Send these logs to the JSON logger.
	klog.SetLogger(logr.FromSlogHandler(logger.Handler()))

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

	mainUpstreamList, err := configureMainUpstreamList(cfg, logger)
	if err != nil {
		logger.Error("failed to configure upstream list", slog.String("error", err.Error()))
		os.Exit(1)
	}

	probeConfig := kubernetes.BuildProbeConfigGetter(cfg)

	// Seed the main list with the fallback upstreams before the port opens. Until the
	// first discovery round a connection would otherwise go through the fallback list:
	// no dial retry, no first-byte retry, and it is never closed when its apiserver
	// stalls. kubelet, kube-controller-manager and kube-scheduler reconnect exactly then,
	// after every proxy restart, and keep that connection for hours. Seeded upstreams
	// wait for their first 200 like discovered ones; discovery replaces the set.
	mainUpstreamList.Reconcile(utils.Map(fallbackList.ListFullAddresses(), func(addr string) *uopts.Upstream {
		return uopts.NewUpstream(addr, probeConfig)
	}))

	lb, err := apilb.NewLoadBalancer(
		cfg.ListenAddress,
		cfg.ListenPort,
		logger,
		apilb.WithDialTimeout(cfg.DialTimeout),
		apilb.WithFirstByteTimeout(cfg.FirstByteTimeout),
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

	go app.StartDiscovery(ctx, cfg, logger, mainUpstreamList, fallbackList, probeConfig)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if err := stop(sigCh, cfg.ShutdownDelay, logger, cancel, srv, lb); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("stopped")
}

// stop waits for a stop signal, then stops discovery, the health server and
// the load balancer.
//
// After the first signal the proxy continues to serve for delay. On a node
// shutdown kubelet sends SIGTERM to the proxy and to other node-critical pods
// at the same time. These pods and kubelet write their last state through the
// proxy. A second signal ends the delay at once.
func stop(
	sigCh <-chan os.Signal,
	delay time.Duration,
	logger *log.Logger,
	cancel context.CancelFunc,
	srv *http.Server,
	lb *apilb.LoadBalancer,
) error {
	sig := <-sigCh

	if delay > 0 {
		logger.Info("stop signal received, serve until the shutdown delay ends",
			slog.String("signal", sig.String()),
			slog.Duration("delay", delay),
		)

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case sig = <-sigCh:
			timer.Stop()
			logger.Info("second stop signal received, end the shutdown delay", slog.String("signal", sig.String()))
		}
	}

	logger.Info("shutting down...")
	cancel()

	// The health server waits for a connection without a request for up to
	// 5s. Do not wait so long: the proxy must exit soon after the shutdown delay.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()

	_ = srv.Shutdown(shutdownCtx)

	return lb.Shutdown()
}

func configureMainUpstreamList(
	cfg config.Config,
	logger *log.Logger,
) (*uopts.List, error) {
	listOptions := []uopts.ListOption{
		uopts.WithHealthcheckInterval(cfg.HealthInterval),
		uopts.WithHealthcheckTimeout(cfg.HealthTimeout),
		uopts.WithFailStep(cfg.FailStepInterval, cfg.FailStepWindow),
		uopts.WithDownHold(cfg.DialTimeout),
		uopts.WithStallTimeout(cfg.StallTimeout),
		uopts.WithLogger(logger),
	}
	if cfg.HealthJitter > 0 {
		listOptions = append(listOptions, uopts.WithHealthCheckJitter(cfg.HealthJitter))
	}

	return uopts.NewList([]*uopts.Upstream{}, listOptions...)
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

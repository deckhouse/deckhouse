/*
Copyright 2024 Flant JSC

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
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/watchdog"
	"fencing-agent/internal/controllers/event"
	"fencing-agent/internal/controllers/grpc"
	"fencing-agent/internal/controllers/http"
	"fmt"
	"os"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/lib/logger/sl"
	"fencing-agent/internal/usecase"
	"os/signal"
	"syscall"

	"fencing-agent/internal/config"
	"fencing-agent/internal/lib/logger"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/sync/errgroup"
)

const (
	Dryrun   = "Dryrun"
	Watchdog = "Watchdog"
)

func main() {
	var cfg config.Config
	cfg.MustLoad()

	// logging
	log := logger.NewLogger(cfg.LogLevel)

	err := AppRun(cfg, log)
	if err != nil {
		log.Error("failed to run application", sl.Err(err))
		os.Exit(1)
	}
}

func AppRun(cfg config.Config, log *log.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	// create kubernetes client
	kubeClient, err := kubeclient.New(cfg.KubeClient, log, cfg.NodeName, cfg.NodeGroup)
	if err != nil {
		return fmt.Errorf("failed to create KubernetesAPI client: %w", err)
	}

	err = kubeClient.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start KubernetesAPI client: %w", err)
	}
	defer kubeClient.Stop()

	ip, err := kubeClient.GetCurrentNodeIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current node IP: %w", err)
	}
	log.Info("current node IP", ip)

	ips, err := kubeClient.GetNodesIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get nodes IPs: %w", err)
	}

	totalNodes := len(ips)
	log.Info("total nodes", "totalNodes", totalNodes)

	quorumDecider := domain.NewQuorumDecider(totalNodes)

	eventBus := usecase.NewEventsBus()

	eventHandler := event.NewEventHandler(log, eventBus)

	mblist, err := memberlist.New(cfg.Memberlist, log, ip, cfg.NodeName, totalNodes, eventHandler, quorumDecider)
	if err != nil {
		return fmt.Errorf("failed to create memberlist: %w", err)
	}

	// always have to start memberlist before all components
	err = mblist.Start(ctx, ips)
	if err != nil {
		return fmt.Errorf("failed to start memberlist: %w", err)
	}
	defer mblist.Stop()

	mblist.BroadcastNodesNumber(totalNodes)

	// -------- clear mblist functionality over -------
	if cfg.FencingMode == Watchdog {
		log.Info("Watchdog enabled, starting health monitor")
		softdog := watchdog.New(cfg.Watchdog.Device)

		fencingAgent := usecase.NewHealthMonitor(
			kubeClient,
			kubeClient,
			mblist,
			softdog,
			quorumDecider,
			kubeClient,
			log,
		)

		err = fencingAgent.Start(ctx, cfg.Watchdog.Timeout)
		if err != nil {
			return fmt.Errorf("failed to start health monitor: %w", err)
		}
		defer fencingAgent.Stop()
	} else {
		log.Info("Dryrun mode enabled, no fencing will be performed")
	}

	// ------- clear fencingAgent functionality over -------

	// get_nodes usecase
	nodesGetter := usecase.NewGetNodes(mblist)

	grpcSrv := grpc.NewServer(log, eventBus, nodesGetter)

	grpcSrvRunner, err := grpc.NewRunner(cfg.GRPC, log, grpcSrv)
	if err != nil {
		return fmt.Errorf("failed to create grpc server runner: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(grpcSrvRunner.Run)

	// ------- clear grpcSrvRunner functionality over -------

	healthzSrv := http.New(log, cfg.HealthProbeBindAddress)

	g.Go(healthzSrv.Run)

	g.Go(func() error {
		<-ctx.Done()
		healthzSrv.Stop()
		grpcSrvRunner.Stop()
		return nil
	})

	return g.Wait()
}

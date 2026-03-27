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
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/watchdog"
	"fencing-agent/internal/config"
	"fencing-agent/internal/controllers/grpc"
	"fencing-agent/internal/controllers/http"
	"fencing-agent/internal/lib/logger"
	"fencing-agent/internal/lib/logger/sl"
	"fencing-agent/internal/usecase"
)

const (
	Notify               = "Notify"
	Watchdog             = "Watchdog"
	fencingNodeLabel     = "node-manager.deckhouse.io/fencing-enabled"
	fencingNodeLabelMode = "node-manager.deckhouse.io/fencing-mode"
)

func main() {
	var cfg config.Config
	cfg.MustLoad()

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

	ip, err := kubeClient.GetCurrentNodeIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current node IP: %w", err)
	}
	log.Info("current node IP", "ip", ip)

	ips, err := kubeClient.GetNodesIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get nodes IPs: %w", err)
	}

	totalNodes := len(ips)
	log.Info("total nodes", "total_nodes", totalNodes)

	quorumDecider := usecase.NewQuorumDecider(totalNodes)

	eventBus := usecase.NewEventsBus()

	eventHandler := usecase.NewNotifier(log, eventBus)

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

	err = setNodeLabels(ctx, kubeClient, cfg.FencingMode)
	if err != nil {
		return fmt.Errorf("failed to set node labels: %w", err)
	}
	defer removeNodeLabels(kubeClient)

	if cfg.FencingMode == Watchdog {
		log.Info("Watchdog enabled, starting health monitor")

		if infErr := kubeClient.StartInformer(ctx); infErr != nil {
			return fmt.Errorf("failed to start informer: %w", infErr)
		}
		defer kubeClient.StopInformer()

		softdog := watchdog.New(cfg.Watchdog.Device)

		fallback := usecase.NewFallback(log, kubeClient)

		fencingAgent := usecase.NewHealthMonitor(
			kubeClient,
			mblist,
			softdog,
			quorumDecider,
			fallback,
			log,
		)

		err = fencingAgent.Start(ctx, cfg.Watchdog.Timeout)
		if err != nil {
			return fmt.Errorf("failed to start health monitor: %w", err)
		}
		defer fencingAgent.Stop()
	} else {
		log.Info("Notify mode enabled, no fencing will be performed")
	}

	nodesGetter := usecase.NewGetNodes(mblist)

	grpcSrv := grpc.NewServer(log, eventBus, nodesGetter)

	grpcSrvRunner, err := grpc.NewRunner(cfg.GRPC, log, grpcSrv)
	if err != nil {
		return fmt.Errorf("failed to create grpc server runner: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(grpcSrvRunner.Run)

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

func setNodeLabels(ctx context.Context, kubeClient *kubeclient.Client, fencingNodeModeValue string) error {
	if labelErr := kubeClient.SetNodeLabel(ctx, fencingNodeLabel, ""); labelErr != nil {
		kubeClient.Logger.Error("unable to set node label, disarming watchdog for safety", sl.Err(labelErr))
		return labelErr
	}

	if labelErr := kubeClient.SetNodeLabel(ctx, fencingNodeLabelMode, fencingNodeModeValue); labelErr != nil {
		kubeClient.Logger.Error("unable to set node label, disarming watchdog for safety", sl.Err(labelErr))
		if err := kubeClient.RemoveNodeLabel(ctx, fencingNodeLabel); err != nil {
			kubeClient.Logger.Error("unable to remove node label", sl.Err(err))
		}
		return labelErr
	}
	return nil
}

func removeNodeLabels(kubeClient *kubeclient.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if labelErr := kubeClient.RemoveNodeLabel(ctx, fencingNodeLabel); labelErr != nil {
		kubeClient.Logger.Error("unable to remove node label", sl.Err(labelErr))
	}

	if labelErr := kubeClient.RemoveNodeLabel(ctx, fencingNodeLabelMode); labelErr != nil {
		kubeClient.Logger.Error("unable to remove node label", sl.Err(labelErr))
	}
}

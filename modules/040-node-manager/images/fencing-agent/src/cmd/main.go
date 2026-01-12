package main

import (
	"context"
	"fencing-controller/internal/adapters/api/grpc"
	"fencing-controller/internal/adapters/kubeapi"
	"fencing-controller/internal/adapters/memberlist"
	"fencing-controller/internal/adapters/watchdog/softdog"
	fencing_config "fencing-controller/internal/config"
	"fencing-controller/internal/core/service"
	"fencing-controller/internal/infrastructures/kubernetes"
	"fencing-controller/internal/infrastructures/logging"

	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cfg fencing_config.Config
	if err := cfg.Load(); err != nil {
		panic(err) // TODO decide: panic or logging
	}

	logger := logging.NewLogger() // TODO decide: configure outside?
	defer func() { _ = logger.Sync() }()

	logger.Debug("Ver: 0.0.1")

	kubeClient, err := kubernetes.GetClientset(cfg.KubernetesAPITimeout)
	if err != nil {
		logger.Fatal("Unable to create a kubernetes clientSet", zap.Error(err))
	}
	eventBus := memberlist.NewEventsBus()
	memberlistProvider, err := memberlist.NewProvider(cfg.MemberlistConfig, logger, eventBus)
	if err != nil {
		logger.Fatal("Unable to create a memberlist provider", zap.Error(err))
	}
	wd := softdog.NewWatchdog(cfg.WatchdogConfig.WatchdogDevice) // TODO cfg WatchdogFeedInterval

	clusterProvider := kubeapi.NewProvider(kubeClient, logger, cfg.KubernetesAPICheckInterval, cfg.NodeName, cfg.NodeGroup)
	healthService := service.NewHealthMonitor(clusterProvider, memberlistProvider, wd, logger)

	go healthService.Run(ctx, cfg.KubernetesAPICheckInterval)

	go func() {
		_ = grpc.Run(logger, cfg.SocketpPath, eventBus)
	}()
	// init healthcheck

	// init memberlist

	// init grpc server

	// graceful shutdown
}

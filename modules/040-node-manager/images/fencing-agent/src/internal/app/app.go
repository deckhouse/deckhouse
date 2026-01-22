package app

import (
	"context"
	"errors"
	"fencing-agent/internal/adapters/api/grpc"
	"fencing-agent/internal/adapters/api/healthz"
	"fencing-agent/internal/adapters/infrastructure/kubeclient"
	"fencing-agent/internal/adapters/kubeapi"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/memberlist/eventbus"
	"fencing-agent/internal/adapters/memberlist/eventhandler"
	"fencing-agent/internal/adapters/watchdog/softdog"
	fencingconfig "fencing-agent/internal/config"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/core/service"
	"fencing-agent/internal/lib/logger/sl"
	"fmt"
	"time"

	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
)

type Application struct {
	config fencingconfig.Config
	logger *log.Logger

	clusterProvider    *kubeapi.Provider
	membershipProvider *memberlist.Provider
	watchDogController *softdog.WatchDog
	eventBus           *eventbus.EventsBus

	healthMonitor  *service.HealthMonitor
	statusProvider *service.StatusProvider

	grpcRunner    *grpc.Runner
	healthzServer *healthz.Server
}

func NewApplication(
	ctx context.Context,
	logger *log.Logger,
	config fencingconfig.Config,
) (*Application, error) {
	unaryRateLimit := rate.NewLimiter(rate.Limit(config.GRPC.UnaryRPS), config.GRPC.UnaryBurst)
	streamRateLimit := rate.NewLimiter(rate.Limit(config.GRPC.StreamRPS), config.GRPC.StreamBurst)

	kubeClient, err := kubeclient.NewClient(config.KubeAPI.KubeConfigPath, config.KubeAPI.KubernetesAPITimeout, float32(config.GRPC.UnaryRPS), config.GRPC.UnaryBurst)
	if err != nil {
		logger.Fatal("Unable to create a kube-client", sl.Err(err))
	}

	eventBus := eventbus.NewEventsBus()
	eventHandler := eventhandler.NewEventHandler(logger, eventBus)

	clusterProvider := kubeapi.NewProvider(
		kubeClient,
		logger,
		config.NodeName,
		config.NodeGroup,
	)

	nodeIP, err := clusterProvider.GetCurrentNodeIP(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current node IP: %w", err)
	}

	memberlistProvider, err := memberlist.NewProvider(config.Memberlist, logger, eventHandler, nodeIP, config.NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist provider: %w", err)
	}

	watchdogController := softdog.NewWatchdog(config.Watchdog.WatchdogDevice)

	healthMonitor := service.NewHealthMonitor(clusterProvider, memberlistProvider, watchdogController, logger)

	statusProvider := service.NewStatusProvider(clusterProvider, memberlistProvider)

	grpcServer := grpc.NewServer(eventBus, statusProvider)

	grpcRunner, err := grpc.NewRunner(config.GRPC.GRPCSocketPath, logger, grpcServer, unaryRateLimit, streamRateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC runner: %w", err)
	}

	healthServer := healthz.New(logger, config.HealthProbeBindAddress)

	logger.Info("application components initialized")

	return &Application{
		config:             config,
		logger:             logger,
		clusterProvider:    clusterProvider,
		membershipProvider: memberlistProvider,
		watchDogController: watchdogController,
		eventBus:           eventBus,
		healthMonitor:      healthMonitor,
		statusProvider:     statusProvider,
		grpcRunner:         grpcRunner,
		healthzServer:      healthServer,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	peers, err := a.discoverPeersIps(ctx)
	if err != nil {
		return err
	}
	go a.startMemberlistWithBackoff(ctx, peers)

	go func() {
		a.logger.Debug("Starting Health Monitor")
		a.healthMonitor.Run(ctx, a.config.KubeAPI.KubernetesAPICheckInterval)
		a.logger.Debug("Health Monitor stopped")
	}()

	grpcErrChan := make(chan error, 1)
	go func() {
		a.logger.Debug("starting gRPC server", slog.String("address", a.config.GRPC.GRPCSocketPath))
		if grpcErr := a.grpcRunner.Run(); grpcErr != nil {
			grpcErrChan <- grpcErr
		}
	}()

	if a.healthzServer != nil {
		go a.healthzServer.StartHealthzServer()
	}

	select {
	case grpcErr := <-grpcErrChan:
		return fmt.Errorf("gRPC server failed: %w", grpcErr)
	case <-ctx.Done():
		a.logger.Debug("main context done, starting graceful shutdown")
		return a.stop()
	}
}

func (a *Application) stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var stopErr error

	a.logger.Debug("stopping health monitor")
	if err := a.healthMonitor.Stop(ctx); err != nil {
		a.logger.Error("failed to stop health monitor", sl.Err(err))
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to stop health monitor: %w", err))
	}

	a.logger.Debug("shutting down gRPC server")
	if err := a.grpcRunner.Stop(ctx); err != nil {
		a.logger.Error("failed to shutdown gRPC server", sl.Err(err))
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown gRPC server: %w", err))
	}

	a.logger.Debug("shutting down healthz server")
	if err := a.healthzServer.StopHealthzServer(ctx); err != nil {
		a.logger.Error("failed to shutdown healthz server", sl.Err(err))
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown healthz server: %w", err))
	}

	a.logger.Debug("shutting down memberlist")
	if err := a.membershipProvider.Stop(ctx); err != nil {
		a.logger.Error("failed to stop memberlist", sl.Err(err))
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to stop memberlist: %w", err))
	}
	return stopErr
}

func (a *Application) startMemberlistWithBackoff(ctx context.Context, peers []string) {
	memberErr := a.membershipProvider.Start(peers)
	base, mx := time.Second, time.Minute
	for backoff := base; memberErr != nil; backoff <<= 1 {
		if backoff > mx {
			backoff = mx
		}
		a.logger.Warn("failed to start memberlist", sl.Err(memberErr), slog.String("backoff", backoff.String()))

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			a.logger.Debug("memberlist start aborted: context canceled")
			return
		case <-timer.C:
			memberErr = a.membershipProvider.Start(peers)
		}
	}
	a.logger.Info("memberlist started successfully")
}

func (a *Application) discoverPeersIps(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.config.KubeAPI.KubernetesAPICheckInterval)
	defer cancel()

	nodes, err := a.clusterProvider.GetNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover peers: %w", err)
	}
	peersIps := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node.Name == a.config.NodeName {
			continue
		}
		peersIps = append(peersIps, node.Addresses[domain.InterfaceName])
	}
	a.logger.Debug("Discovered peers", slog.Any("peers", peersIps))
	return peersIps, nil
}

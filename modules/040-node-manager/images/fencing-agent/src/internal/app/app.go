package app

import (
	"context"
	"fencing-agent/internal/adapters/api/grpc"
	"fencing-agent/internal/adapters/kubeapi"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/watchdog/softdog"
	fencing_config "fencing-agent/internal/config"
	"fencing-agent/internal/core/service"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Applicaion struct {
	config fencing_config.Config
	logger *zap.Logger

	clusterProvider    *kubeapi.Provider
	membershipProvider *memberlist.Provider
	watchDogController *softdog.WatchDog
	eventBus           *memberlist.EventsBus

	healthMonitor  *service.HealthMonitor
	statusProvider *service.StatusProvider

	grpcServer    *grpc.Server
	healthzServer *http.Server
}

func NewApplication(
	logger *zap.Logger,
	kubeClient kubernetes.Interface,
	config fencing_config.Config,
) (*Applicaion, error) {
	eventBus := memberlist.NewEventsBus()

	clusterProvider := kubeapi.NewProvider(
		kubeClient,
		logger,
		config.KubernetesAPITimeout,
		config.NodeName,
		config.NodeGroup,
	)

	nodeIP, err := getCurrentNodeIP(kubeClient, config.NodeName, config.KubernetesAPITimeout)
	if err != nil {
		return nil, fmt.Errorf("Failed to get current node IP: %w", err)
	}

	memberlistProvider, err := memberlist.NewProvider(config.MemberlistConfig, logger, eventBus, nodeIP, config.NodeName)
	if err != nil {
		return nil, fmt.Errorf("Failed to create memberlist provider: %w", err)
	}

	watchdogController := softdog.NewWatchdog(config.WatchdogConfig.WatchdogDevice)

	healthMonitor := service.NewHealthMonitor(clusterProvider, memberlistProvider, watchdogController, logger)

	statusProvider := service.NewStatusProvider(clusterProvider, memberlistProvider)

	grpcServer := grpc.NewServer(eventBus, statusProvider)

	var healthServer *http.Server
	if config.HealthProbeBindAddress != "" {
		healthServer = createHealthzServer(config.HealthProbeBindAddress)
	}
	logger.Info("Application components initialized")

	return &Applicaion{
		config:             config,
		logger:             logger,
		clusterProvider:    clusterProvider,
		membershipProvider: memberlistProvider,
		watchDogController: watchdogController,
		eventBus:           eventBus,
		healthMonitor:      healthMonitor,
		statusProvider:     statusProvider,
		grpcServer:         grpcServer,
		healthzServer:      healthServer,
	}, nil
}

func (a *Applicaion) Run(ctx context.Context) error {
	a.logger.Debug("Start v0.0.1")

	if a.healthzServer != nil {
		go a.startHealthzServer(ctx)
	}

	peers, err := a.discoverPeersIps(ctx)

	go func() {
		err = a.membershipProvider.Start(peers)
		for err != nil {
			a.logger.Warn("failed to start memberlist", zap.Error(err))
			err = a.membershipProvider.Start(peers)
			time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		a.logger.Debug("Starting Health Monitor")
		a.healthMonitor.Run(ctx, a.config.KubernetesAPICheckInterval)
		a.logger.Debug("Health Monitor stopped")
	}()

	grpcErrChan := make(chan error, 1)
	go func() {
		a.logger.Debug("Starting GRPC server")
		if err = grpc.Run(a.config.GRPCAddress, a.grpcServer); err != nil {
			grpcErrChan <- err
		}
	}()

	select {
	case err = <-grpcErrChan:
		return fmt.Errorf("Failed to run GRPC server: %w", err)
	case <-ctx.Done():
		return a.Stop()
	}
}

func (a *Applicaion) Stop() error {
	if a.watchDogController.IsArmed() {
		if err := a.watchDogController.Stop(); err != nil {
			a.logger.Error("Unable to disarm watchdog", zap.Error(err))
		}
	}

	if a.healthzServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.healthzServer.Shutdown(ctx)
	}
	return nil
}

func (a *Applicaion) discoverPeersIps(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.config.KubernetesAPICheckInterval)
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
		peersIps = append(peersIps, node.Addresses["eth0"])
	}
	a.logger.Debug("Discovered peers", zap.Strings("peers", peersIps))
	return peersIps, nil
}

// TODO unused context
func (a *Applicaion) startHealthzServer(ctx context.Context) {
	a.logger.Info("Stating healthz server", zap.String("bindAddress", a.config.HealthProbeBindAddress))

	if err := a.healthzServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		a.logger.Error("Healthz server failed", zap.Error(err))
	}
}
func createHealthzServer(bindAddress string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return &http.Server{Addr: bindAddress, Handler: mux}
}
func getCurrentNodeIP(kubeClient kubernetes.Interface, nodeName string, timeout time.Duration) (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node=%s InternalIp for memberlist: %w", nodeName, err)
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {

			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("node %s has no InternalIP address", nodeName)
}

package app

import (
	"context"
	"fencing-agent/internal/adapters/api/grpc"
	"fencing-agent/internal/adapters/api/healthz"
	"fencing-agent/internal/adapters/kubeapi"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/memberlist/event_handler"
	"fencing-agent/internal/adapters/memberlist/eventbus"
	"fencing-agent/internal/adapters/watchdog/softdog"
	fencingconfig "fencing-agent/internal/config"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/core/service"
	"fencing-agent/internal/lib/logger/sl"
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
	kubeClient, err := getClientset(config.KubernetesAPITimeout)
	if err != nil {
		logger.Fatal("Unable to create a kube-client", sl.Err(err))
	}

	eventBus := eventbus.NewEventsBus()
	eventHandler := event_handler.NewEventHandler(logger, eventBus)

	clusterProvider := kubeapi.NewProvider(
		kubeClient,
		logger,
		config.KubernetesAPITimeout,
		config.NodeName,
		config.NodeGroup,
	)

	nodeIP, err := getCurrentNodeIP(ctx, kubeClient, config.NodeName, config.KubernetesAPITimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get current node IP: %w", err)
	}

	memberlistProvider, err := memberlist.NewProvider(config.MemberlistConfig, logger, eventHandler, nodeIP, config.NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist provider: %w", err)
	}

	watchdogController := softdog.NewWatchdog(config.WatchdogConfig.WatchdogDevice)

	healthMonitor := service.NewHealthMonitor(clusterProvider, memberlistProvider, watchdogController, logger)

	statusProvider := service.NewStatusProvider(clusterProvider, memberlistProvider)

	grpcServer := grpc.NewServer(eventBus, statusProvider)

	unaryRateLimit := rate.NewLimiter(rate.Limit(config.GRPSRateLimit.UnaryRPS), config.GRPSRateLimit.UnaryBurst)
	streamRateLimit := rate.NewLimiter(rate.Limit(config.GRPSRateLimit.StreamRPS), config.GRPSRateLimit.StreamBurst)

	grpcRunner, err := grpc.NewRunner(config.GRPCAddress, logger, grpcServer, unaryRateLimit, streamRateLimit)
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
	go func() {
		memberErr := a.membershipProvider.Start(peers)
		base, mx := time.Second, time.Minute
		for backoff := base; memberErr != nil; backoff <<= 1 {
			if backoff > mx {
				backoff = mx
			}
			select {
			case <-ctx.Done():
				a.logger.Debug("memberlist start aborted: context canceled")
				return
			default:
				a.logger.Warn("failed to start memberlist", sl.Err(memberErr), slog.String("backoff", backoff.String()))

				select {
				case <-ctx.Done():
					a.logger.Debug("memberlist start aborted: context canceled")
					return
				case <-time.After(backoff):
					memberErr = a.membershipProvider.Start(peers)
				}
			}
		}
		a.logger.Info("memberlist started successfully")
	}()

	go func() {
		a.logger.Debug("Starting Health Monitor")
		a.healthMonitor.Run(ctx, a.config.KubernetesAPICheckInterval)
		a.logger.Debug("Health Monitor stopped")
	}()

	grpcErrChan := make(chan error, 1)
	go func() {
		a.logger.Debug("starting gRPC server", slog.String("address", a.config.GRPCAddress))
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

	a.logger.Debug("stopping health monitor")
	if err := a.healthMonitor.Stop(ctx); err != nil {
		a.logger.Error("failed to stop health monitor", sl.Err(err))
	}

	a.logger.Debug("shutting down gRPC server")
	if err := a.grpcRunner.Shutdown(ctx); err != nil {
		a.logger.Error("failed to shutdown gRPC server", sl.Err(err))
	}

	a.logger.Debug("shutting down healthz server")
	if err := a.healthzServer.StopHealthzServer(ctx); err != nil {
		a.logger.Error("failed to shutdown healthz server", sl.Err(err))
	}

	a.logger.Debug("shutting down memberlist")
	if err := a.membershipProvider.Stop(ctx); err != nil {
		a.logger.Error("failed to stop memberlist", sl.Err(err))
	}
	return nil
}

func (a *Application) discoverPeersIps(ctx context.Context) ([]string, error) {
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
		peersIps = append(peersIps, node.Addresses[domain.InterfaceName])
	}
	a.logger.Debug("Discovered peers", slog.Any("peers", peersIps))
	return peersIps, nil
}

func getCurrentNodeIP(ctx context.Context, kubeClient kubernetes.Interface, nodeName string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
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

// Reimplementation of clientcmd.buildConfig to avoid default warn message
func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		kubeconfig, err := rest.InClusterConfig()
		if err == nil {
			return kubeconfig, nil
		}
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
}

func getClientset(timeout time.Duration) (*kubernetes.Clientset, error) {
	var restConfig *rest.Config
	var kubeClient *kubernetes.Clientset
	var err error

	restConfig, err = buildConfig(os.Getenv("KUBECONFIG"))
	if err != nil {
		return nil, err
	}

	restConfig.Timeout = timeout

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

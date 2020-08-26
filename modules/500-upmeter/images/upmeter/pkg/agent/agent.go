package agent

import (
	"context"
	shapp "github.com/flant/shell-operator/pkg/app"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/app"
	"upmeter/pkg/probe/executor"
	"upmeter/pkg/probe/manager"
	"upmeter/pkg/probe/sender"
)

type Agent struct {
	ctx    context.Context
	cancel context.CancelFunc

	KubernetesClient kube.KubernetesClient
	MetricStorage    *metric_storage.MetricStorage

	Executor      *executor.ProbeExecutor
	Manager       *manager.ProbeManager
	Sender        *sender.Sender
	UpmeterClient *sender.UpmeterClient
}

func NewAgent(ctx context.Context) *Agent {
	a := &Agent{}
	a.ctx, a.cancel = context.WithCancel(ctx)
	return a
}

// Return agent with wired dependencies
func NewDefaultAgent(ctx context.Context) *Agent {
	a := NewAgent(ctx)

	// Metric storage
	a.MetricStorage = metric_storage.NewMetricStorage()

	// Kubernetes client
	a.KubernetesClient = kube.NewKubernetesClient()
	a.KubernetesClient.WithContextName(shapp.KubeContext)
	a.KubernetesClient.WithConfigPath(shapp.KubeConfig)
	a.KubernetesClient.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)
	a.KubernetesClient.WithMetricStorage(a.MetricStorage)

	// Probe manager
	a.Manager = manager.NewProbeManager()
	a.Manager.Init() // Create instances for each prober.

	a.UpmeterClient = sender.CreateUpmeterClient(app.UpmeterHost, app.UpmeterPort)

	a.Sender = sender.NewSender(context.Background())
	a.Sender.WithUpmeterClient(a.UpmeterClient)

	a.Executor = executor.NewProbeExecutor(context.Background())
	a.Executor.WithProbeManager(a.Manager)
	a.Executor.WithDowntimeEpisodesCh(a.Sender.DowntimeEpisodesCh)
	a.Executor.WithKubernetesClient(a.KubernetesClient)

	return a
}

func (a *Agent) Start() error {
	// Initialize kube client from kubeconfig.
	err := a.KubernetesClient.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}

	a.Sender.Start()
	a.Executor.Start()

	// block
	var ch = make(chan struct{})
	<-ch

	// ProbeResultStorage.Start()
	return nil
}

func (a *Agent) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}

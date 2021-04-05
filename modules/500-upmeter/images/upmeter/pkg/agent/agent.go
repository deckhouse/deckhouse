package agent

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/agent/executor"
	"upmeter/pkg/agent/manager"
	"upmeter/pkg/agent/sender"
	"upmeter/pkg/app"
	"upmeter/pkg/check"
	"upmeter/pkg/kubernetes"
)

type Agent struct {
	ctx    context.Context
	cancel context.CancelFunc

	executor      *executor.ProbeExecutor
	sender        *sender.Sender
	upmeterClient *sender.UpmeterClient

	access *kubernetes.Access
}

func NewAgent(ctx context.Context) *Agent {
	a := &Agent{}
	a.ctx, a.cancel = context.WithCancel(ctx)
	return a
}

// Return agent with wired dependencies
func NewDefaultAgent(ctx context.Context) *Agent {
	a := NewAgent(ctx)

	// Probe manager
	a.access = &kubernetes.Access{}
	probeManager := manager.New(a.access)
	for _, probe := range probeManager.Runners() {
		log.Infof("Register probe %s", probe.Id())
	}
	for _, calc := range probeManager.Calculators() {
		log.Infof("Register calculated probe %s", calc.Id())
	}

	timeout := 10 * time.Second
	a.upmeterClient = sender.NewUpmeterClient(app.UpmeterHost, app.UpmeterPort, timeout)
	// TODO move context to Start methods
	ch := make(chan []check.DowntimeEpisode)
	a.sender = sender.NewSender(context.Background(), a.upmeterClient, ch)
	a.executor = executor.NewProbeExecutor(context.Background(), probeManager, ch)

	return a
}

func (a *Agent) Start() error {
	// Initialize kube client from kubeconfig and service account token from filesystem.
	err := a.access.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: %s\n", err)
		return err
	}

	a.sender.Start()
	a.executor.Start()

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

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

func NewAgent() *Agent {
	a := &Agent{}
	return a
}

// Return agent with wired dependencies
func NewDefaultAgent() *Agent {
	a := NewAgent()

	// Probe manager
	a.access = &kubernetes.Access{}
	probeManager := manager.New(a.access)
	for _, probe := range probeManager.Runners() {
		log.Infof("Register probe %s", probe.ProbeRef().Id())
	}
	for _, calc := range probeManager.Calculators() {
		log.Infof("Register calculated probe %s", calc.ProbeRef().Id())
	}

	clientTimeout := 10 * time.Second
	a.upmeterClient = sender.NewUpmeterClient(app.ServiceHost, app.ServicePort, clientTimeout)

	ch := make(chan []check.Episode)
	a.sender = sender.New(a.upmeterClient, ch)
	a.executor = executor.New(probeManager, ch)

	return a
}

func (a *Agent) Start(ctx context.Context) error {
	// Initialize kube client from kubeconfig and service account token from filesystem.
	err := a.access.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: %s\n", err)
		return err
	}

	a.sender.Start(ctx)
	a.executor.Start(ctx)

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

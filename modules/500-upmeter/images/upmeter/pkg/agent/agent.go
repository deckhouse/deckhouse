package agent

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/agent/executor"
	"d8.io/upmeter/pkg/agent/manager"
	"d8.io/upmeter/pkg/agent/sender"
	"d8.io/upmeter/pkg/app"
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/migrations"
	"d8.io/upmeter/pkg/kubernetes"
)

type Agent struct {
	period          time.Duration
	dbPath          string
	dbMigrationPath string
}

// Return agent with magic configuration
func New() *Agent {
	return &Agent{
		period: time.Second,

		dbPath:          app.DatabasePath,
		dbMigrationPath: app.DatabaseMigrationsPath,
	}
}

func (a *Agent) Start(ctx context.Context) error {
	// Initialize kube client from kubeconfig and service account token from filesystem.
	kubeAcceess := &kubernetes.Access{}
	err := kubeAcceess.Init()
	if err != nil {
		return fmt.Errorf("cannot init access to Kubernetes cluster: %v", err)
	}

	// Probe registry
	registry := manager.New(kubeAcceess)
	for _, probe := range registry.Runners() {
		log.Infof("Register probe %s", probe.ProbeRef().Id())
	}
	for _, calc := range registry.Calculators() {
		log.Infof("Register calculated probe %s", calc.ProbeRef().Id())
	}

	// Database connection with pool
	dbctx, err := migrations.GetMigratedDatabase(a.dbPath, a.dbMigrationPath)
	if err != nil {
		return fmt.Errorf("cannot connect to database: %v", err)
	}

	storage := sender.NewStorage(dbctx)
	scheduler, sender := initSenderAndScheduler(storage, registry, a.period)

	sender.Start(ctx)
	scheduler.Start(ctx)

	// block
	ch := make(chan struct{})
	<-ch

	// ProbeResultStorage.Start()
	return nil
}

func initSenderAndScheduler(storage *sender.ListStorage, registry *manager.Manager, sendPeriod time.Duration) (*executor.ProbeExecutor, *sender.Sender) {
	ch := make(chan []check.Episode)

	client := sender.NewClient(sendPeriod)
	sender := sender.New(client, ch, storage, sendPeriod)

	scheduler := executor.New(registry, ch)

	return scheduler, sender
}

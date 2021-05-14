package agent

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/agent/executor"
	"d8.io/upmeter/pkg/agent/manager"
	"d8.io/upmeter/pkg/agent/sender"
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/migrations"
	"d8.io/upmeter/pkg/kubernetes"
)

type Agent struct {
	config     *Config
	kubeConfig *kubernetes.Config

	logger *log.Logger

	sender    *sender.Sender
	scheduler *executor.ProbeExecutor
}

type Config struct {
	Period time.Duration

	Namespace string

	ClientConfig *sender.ClientConfig

	DatabasePath           string
	DatabaseMigrationsPath string
}

// Return agent with magic configuration
func New(config *Config, kubeConfig *kubernetes.Config, logger *log.Logger) *Agent {
	return &Agent{
		config:     config,
		kubeConfig: kubeConfig,
		logger:     logger,
	}
}

func (a *Agent) Start(ctx context.Context) error {
	// Initialize kube client from kubeconfig and service account token from filesystem.
	kubeAccess := &kubernetes.Accessor{}
	err := kubeAccess.Init(a.kubeConfig)
	if err != nil {
		return fmt.Errorf("cannot init access to Kubernetes cluster: %v", err)
	}

	// Probe registry
	registry := manager.New(kubeAccess)
	for _, probe := range registry.Runners() {
		a.logger.Infof("Register probe %s", probe.ProbeRef().Id())
	}
	for _, calc := range registry.Calculators() {
		a.logger.Infof("Register calculated probe %s", calc.ProbeRef().Id())
	}

	// Database connection with pool
	dbctx, err := migrations.GetMigratedDatabase(ctx, a.config.DatabasePath, a.config.DatabaseMigrationsPath)
	if err != nil {
		return fmt.Errorf("cannot connect to database: %v", err)
	}

	ch := make(chan []check.Episode)

	client := sender.NewClient(a.config.ClientConfig, a.config.Period) // use period as timeout
	storage := sender.NewStorage(dbctx)

	a.sender = sender.New(client, ch, storage, a.config.Period)
	a.scheduler = executor.New(registry, ch)

	a.sender.Start()
	a.scheduler.Start()

	return nil
}

func (a *Agent) Stop() error {
	a.scheduler.Stop()
	a.sender.Stop()
	return nil
}

package appload

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/installer"
	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "appLoad"
)

type DependencyContainer interface {
	Installer() *installer.Installer
	PackageManager() *packagemanager.Manager
}

type Config struct {
	AppName    string
	Package    string
	Version    string
	Settings   map[string]interface{}
	Repository *v1alpha1.PackageRepository
}

type task struct {
	appName     string
	packageName string
	version     string
	settings    map[string]interface{}
	repository  *v1alpha1.PackageRepository

	dc DependencyContainer

	logger *log.Logger
}

func New(conf Config, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		appName:     conf.AppName,
		packageName: conf.Package,
		version:     conf.Version,
		repository:  conf.Repository,
		settings:    conf.Settings,
		dc:          dc,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Application:%s:Load", t.appName)
}

func (t *task) Execute(ctx context.Context) error {
	if err := t.dc.Installer().Ensure(ctx, t.repository, t.appName, t.packageName, t.version); err != nil {
		return fmt.Errorf("ensure application: %w", err)
	}

	if err := t.dc.PackageManager().LoadApplication(ctx, t.appName, t.settings); err != nil {
		return fmt.Errorf("load application: %w", err)
	}

	return nil
}

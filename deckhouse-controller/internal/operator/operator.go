package operator

import (
	"context"
	"fmt"

	addonapp "github.com/flant/addon-operator/pkg/app"
	klient "github.com/flant/kube-client/client"
	shapp "github.com/flant/shell-operator/pkg/app"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/operator/taskevent"
	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// TODO(ipaqsa): tmp solution
	appsDir = "/deckhouse/packages/apps"

	operatorTracer = "operator"

	// TODO(ipaqsa): tmp solution
	namespace = "d8-system"
)

type Operator struct {
	eventHandler   *taskevent.Handler
	packageManager *packagemanager.Manager
	queueService   *queue.Service
	nelmService    *nelm.Service

	objectPatcher     *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	logger *log.Logger
}

func New(ctx context.Context, logger *log.Logger) (*Operator, error) {
	o := new(Operator)

	o.queueService = queue.NewService(ctx, logger)
	o.scheduleManager = schedulemanager.NewScheduleManager(ctx, logger.Named("schedule-manager"))
	o.logger = logger.Named(operatorTracer)

	if err := o.buildNelmService(ctx); err != nil {
		return nil, fmt.Errorf("build nelm service: %w", err)
	}

	if err := o.buildObjectPatcher(); err != nil {
		return nil, fmt.Errorf("build object patcher: %w", err)
	}

	if err := o.buildKubeEventsManager(ctx); err != nil {
		return nil, fmt.Errorf("build kube events manager: %w", err)
	}

	o.packageManager = packagemanager.New(packagemanager.Config{
		AppsDir:           appsDir,
		NelmService:       o.nelmService,
		KubeObjectPatcher: o.objectPatcher,
		ScheduleManager:   o.scheduleManager,
		KubeEventsManager: o.kubeEventsManager,
	}, logger)

	o.eventHandler = taskevent.NewHandler(taskevent.Config{
		KubeEventsManager: o.kubeEventsManager,
		ScheduleManager:   o.scheduleManager,
		PackageManager:    o.packageManager,
		QueueService:      o.queueService,
	}, logger)

	o.eventHandler.Start(ctx)

	return o, nil
}

func (o *Operator) Stop() {
	o.logger.Info("stop operator")

	o.queueService.Stop()
	o.eventHandler.Stop()

	o.scheduleManager.Stop()
	o.kubeEventsManager.PauseHandleEvents()

	o.nelmService.StopMonitors()
}

func (o *Operator) KubeEventsManager() kubeeventsmanager.KubeEventsManager {
	return o.kubeEventsManager
}

func (o *Operator) ScheduleManager() schedulemanager.ScheduleManager {
	return o.scheduleManager
}

func (o *Operator) QueueService() *queue.Service {
	return o.queueService
}

func (o *Operator) buildObjectPatcher() error {
	client := klient.New(klient.WithLogger(o.logger.Named("object-patcher-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.ObjectPatcherKubeClientQps, shapp.ObjectPatcherKubeClientBurst)
	client.WithTimeout(shapp.ObjectPatcherKubeClientTimeout)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize object patcher client: %s\n", err)
	}

	o.objectPatcher = objectpatch.NewObjectPatcher(client, o.logger.Named("object-patcher"))
	return nil
}

func (o *Operator) buildKubeEventsManager(ctx context.Context) error {
	client := klient.New(klient.WithLogger(o.logger.Named("kube-events-manager-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize kube events manager client: %s\n", err)
	}

	o.kubeEventsManager = kubeeventsmanager.NewKubeEventsManager(ctx, client, o.logger.Named("kube-events-manager"))
	return nil
}

func (o *Operator) buildNelmService(ctx context.Context) error {
	client := klient.New(klient.WithLogger(o.logger.Named("nelm-monitor-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(addonapp.HelmMonitorKubeClientQps, addonapp.HelmMonitorKubeClientBurst)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize nelm service client: %s\n", err)
	}

	cache, err := runtimecache.New(client.RestConfig(), runtimecache.Options{})
	if err != nil {
		return fmt.Errorf("create runtime cache: %v", err)
	}

	o.nelmService = nelm.NewService(ctx, namespace, cache, o.logger)
	return nil
}

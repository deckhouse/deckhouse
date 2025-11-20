// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/Masterminds/semver/v3"
	addonapp "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	klient "github.com/flant/kube-client/client"
	shapp "github.com/flant/shell-operator/pkg/app"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/cron"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/installer"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/eventhandler"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/disable"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/run"
	taskstartup "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/startup"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	operatorTracer = "operator"

	bootstrappedGlobalValue = "clusterIsBootstrapped"
	kubernetesVersionValue  = "kubernetesVersion"
	deckhouseVersionValue   = "version"
)

type Operator struct {
	eventHandler *eventhandler.Handler // Converts events (Kube/schedule) into tasks
	queueService *queue.Service        // Task queue for hook execution
	nelmService  *nelm.Service         // Helm release management and monitoring
	installer    *installer.Installer  // Erofs installer

	manager     *manager.Manager
	scheduler   *schedule.Scheduler
	debugServer *debug.Server

	objectPatcher     *objectpatch.ObjectPatcher          // Applies resource patches from hooks
	scheduleManager   schedulemanager.ScheduleManager     // Cron-based schedule triggers
	kubeEventsManager kubeeventsmanager.KubeEventsManager // Watches Kubernetes resources

	mu       sync.Mutex
	packages map[string]*Package

	logger *log.Logger
}

type moduleManager interface {
	GetGlobal() *modules.GlobalModule
}

// New creates and initializes a new Operator instance with all subsystems.
//
// Initialization order is important:
//  1. Queue and schedule services (independent)
//  2. NELM service (requires its own client and cache)
//  3. Object patcher (for hook-driven resource modifications)
//  4. Kubernetes events manager (watches cluster resources)
//  5. Package manager (depends on all above services)
//  6. Event handler (coordinates everything, starts immediately)
//
// Each Kubernetes integration gets its own client with specific rate limits:
//   - Object patcher: Higher QPS for batch patching operations
//   - Kube events: Standard QPS for resource watching
//   - NELM monitor: Tuned QPS for Helm resource monitoring
//
// The event handler starts immediately to begin processing events.
func New(moduleManager moduleManager, dc dependency.Container, logger *log.Logger) (*Operator, error) {
	o := new(Operator)

	o.packages = make(map[string]*Package)

	// Initialize foundational services
	o.logger = logger.Named(operatorTracer)
	o.scheduleManager = cron.NewManager(o.logger)
	o.queueService = queue.NewService(o.logger)
	o.installer = installer.New(dc, o.logger)

	// Initialize scheduler with enabling/disabling callbacks
	o.buildScheduler(moduleManager)

	// Build NELM service with its own client and runtime cache for resource monitoring
	if err := o.buildNelmService(); err != nil {
		return nil, fmt.Errorf("build nelm service: %w", err)
	}

	// Build object patcher with optimized rate limits for batch operations
	if err := o.buildObjectPatcher(); err != nil {
		return nil, fmt.Errorf("build object patcher: %w", err)
	}

	// Build Kubernetes events manager for watching cluster resources
	if err := o.buildKubeEventsManager(); err != nil {
		return nil, fmt.Errorf("build kube events manager: %w", err)
	}

	// Initialize package manager with all dependencies
	o.manager = manager.New(manager.Config{
		OnValuesChanged: func(ctx context.Context, name string) {
			o.mu.Lock()
			defer o.mu.Unlock()

			if _, ok := o.packages[name]; !ok {
				return
			}

			if o.packages[name].status.Phase == Running {
				o.queueService.Enqueue(ctx, name, taskrun.NewTask(name, o.manager, o.logger), queue.WithUnique())
			}
		},
		NelmService:       o.nelmService,
		KubeObjectPatcher: o.objectPatcher,
		ScheduleManager:   o.scheduleManager,
		KubeEventsManager: o.kubeEventsManager,
	}, o.logger)

	// Create event handler to orchestrate event processing
	o.eventHandler = eventhandler.New(eventhandler.Config{
		KubeEventsManager: o.kubeEventsManager,
		ScheduleManager:   o.scheduleManager,
		PackageManager:    o.manager,
		QueueService:      o.queueService,
	}, o.logger).Start()

	if err := o.registerDebugServer("/tmp/deckhouse-debug.socket"); err != nil {
		return nil, fmt.Errorf("register debug server: %w", err)
	}

	return o, nil
}

func (o *Operator) registerDebugServer(sockerPath string) error {
	o.debugServer = debug.NewServer(o.logger)
	if err := o.debugServer.Start(sockerPath); err != nil {
		return fmt.Errorf("start debug server: %w", err)
	}

	o.debugServer.Register(http.MethodGet, "/packages/dump", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)

		w.Write(o.Dump()) //nolint:errcheck
	})

	o.debugServer.Register(http.MethodGet, "/packages/queues/dump", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)

		w.Write(o.queueService.Dump()) //nolint:errcheck
	})

	return nil
}

// Scheduler return the scheduler for external access
func (o *Operator) Scheduler() *schedule.Scheduler {
	return o.scheduler
}

// Stop performs graceful shutdown of all operator subsystems.
//
// Shutdown order ensures safe termination:
//  1. Stop NELM monitors (cleanup resource monitoring)
//  2. Pause Kubernetes event handling (no new resource events)
//  3. Stop schedule manager (no new cron triggers)
//  4. Stop event handler (no new task generation)
//  5. Stop queue service (no new task processing)
//
// This order prevents new work from entering the system while allowing
// in-flight operations to complete gracefully where possible.
func (o *Operator) Stop() {
	o.logger.Info("stop operator")

	// Clean up resource monitors
	o.nelmService.StopMonitors()

	// Stop generating new events
	o.scheduleManager.Stop()
	o.kubeEventsManager.PauseHandleEvents()

	// Stop accepting and processing new tasks
	o.eventHandler.Stop()
	o.queueService.Stop()
}

// buildObjectPatcher creates a Kubernetes client optimized for patch operations.
//
// Uses dedicated rate limits (QPS and burst) tuned for batch resource patching.
// Hooks can generate multiple patch operations (create/update/delete resources)
// that need to be applied quickly, so this client has higher throughput limits
// than the general-purpose event watching client.
//
// Also sets a custom timeout for patch operations to prevent hanging on slow API calls.
func (o *Operator) buildObjectPatcher() error {
	client := klient.New(klient.WithLogger(o.logger.Named("object-patcher-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.ObjectPatcherKubeClientQps, shapp.ObjectPatcherKubeClientBurst)
	client.WithTimeout(shapp.ObjectPatcherKubeClientTimeout)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize object patcher client: %w", err)
	}

	o.objectPatcher = objectpatch.NewObjectPatcher(client, o.logger.Named("object-patcher"))
	return nil
}

// buildKubeEventsManager creates a Kubernetes client for watching cluster resources.
//
// This client is used by hooks to watch for resource changes (create/update/delete).
// Uses standard rate limits appropriate for long-running watches and informers.
//
// The KubeEventsManager handles:
//   - Setting up informers/watchers based on hook configurations
//   - Filtering events based on namespaces, labels, and field selectors
//   - Converting Kubernetes events into binding contexts for hook execution
func (o *Operator) buildKubeEventsManager() error {
	client := klient.New(klient.WithLogger(o.logger.Named("kube-events-manager-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize kube events manager client: %w", err)
	}

	o.kubeEventsManager = kubeeventsmanager.NewKubeEventsManager(context.Background(), client, o.logger.Named("kube-events-manager"))

	// Initialize metric storage for the kube events manager
	// This is required to record metrics during hook initialization and execution
	metricStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithLogger(o.logger.Named("kube-events-metrics")),
		metricsstorage.WithNewRegistry(),
	)
	o.kubeEventsManager.WithMetricStorage(metricStorage)

	return nil
}

// buildNelmService creates the NELM (Helm) service with monitoring capabilities.
//
// NELM manages Helm releases and monitors their resources for drift detection.
// This requires:
//  1. A dedicated Kubernetes client with rate limits tuned for monitoring
//  2. A controller-runtime cache for efficient resource queries
//
// The cache must be started and synced before the NELM service can function:
//   - cache.Start() runs the cache informers in the background
//   - cache.WaitForCacheSync() blocks until initial resource listing completes
//
// Resource monitoring detects:
//   - Missing resources (deleted outside of Helm)
//   - Configuration drift between desired and actual state
//
// Rate limits are specific to monitoring workloads (different from patch or watch clients).
func (o *Operator) buildNelmService() error {
	client := klient.New(klient.WithLogger(o.logger.Named("nelm-monitor-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(addonapp.HelmMonitorKubeClientQps, addonapp.HelmMonitorKubeClientBurst)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize nelm service client: %w", err)
	}

	// Create controller-runtime cache for efficient resource queries during monitoring
	cache, err := runtimecache.New(client.RestConfig(), runtimecache.Options{})
	if err != nil {
		return fmt.Errorf("create runtime cache: %w", err)
	}

	// Start cache informers in background
	go func() {
		if err = cache.Start(context.Background()); err != nil {
			o.logger.Error("failed to start cache", "error", err)
		}
	}()

	// Wait for cache to complete initial sync before proceeding
	// This ensures monitors have current resource state from the start
	if !cache.WaitForCacheSync(context.Background()) {
		return fmt.Errorf("cache sync failed")
	}

	o.nelmService = nelm.NewService(cache, func(name string) {
		o.mu.Lock()
		defer o.mu.Unlock()

		if _, ok := o.packages[name]; !ok {
			return
		}

		if o.packages[name].status.Phase == Running {
			o.queueService.Enqueue(context.Background(), name, taskrun.NewTask(name, o.manager, o.logger), queue.WithUnique())
		}
	}, o.logger)

	return nil
}

// buildScheduler creates the package scheduler with version checks and lifecycle callbacks.
//
// The scheduler controls package enable/disable based on:
//   - Kubernetes version constraints (from package metadata)
//   - Deckhouse version constraints (from package metadata)
//   - Bootstrap state (cluster must be fully initialized first)
//
// Version getters extract current versions from global values provided by discovery hooks.
//
// Lifecycle callbacks:
//   - onEnable: Runs startup hooks when package becomes enabled (Loaded -> Running)
//   - onDisable: Stops hooks and transitions package back to Loaded state
//
// The scheduler starts paused and is resumed after initial package loading completes.
func (o *Operator) buildScheduler(moduleManager moduleManager) {
	deckhouseVersionGetter := func() (*semver.Version, error) {
		discovery := moduleManager.GetGlobal().GetValues(false).GetKeySection("discovery")
		if len(discovery) == 0 {
			return nil, fmt.Errorf("discovery section not found in global values")
		}

		value, ok := discovery[deckhouseVersionValue]
		if !ok {
			return nil, fmt.Errorf("deckhouse version not found in global values")
		}

		version, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid deckhouse version")
		}

		return semver.NewVersion(version)
	}

	kubernetesVersionGetter := func() (*semver.Version, error) {
		discovery := moduleManager.GetGlobal().GetValues(false).GetKeySection("discovery")
		if len(discovery) == 0 {
			return nil, fmt.Errorf("discovery section not found in global values")
		}

		value, ok := discovery[kubernetesVersionValue]
		if !ok {
			return nil, fmt.Errorf("kubernetes version not found in global values")
		}

		version, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid kubernetes version")
		}

		return semver.NewVersion(version)
	}

	// Bootstrap condition checks if cluster initialization is complete
	bootstrapCondition := func() bool {
		value, ok := moduleManager.GetGlobal().GetValues(false)[bootstrappedGlobalValue]
		if !ok {
			return false
		}

		bootstrapped, ok := value.(bool)
		if !ok {
			return false
		}

		return bootstrapped
	}

	// onEnable transitions package from Loaded to Running by executing startup hooks
	onEnable := func(ctx context.Context, name string) {
		o.mu.Lock()
		defer o.mu.Unlock()

		if o.packages[name].status.Phase == Loaded {
			o.queueService.Enqueue(ctx, name, taskstartup.NewTask(name, o.manager, o.queueService, o.logger), queue.WithOnDone(func() {
				o.mu.Lock()
				defer o.mu.Unlock()

				if _, ok := o.packages[name]; ok {
					o.packages[name].status.Phase = Running
				}
			}), queue.WithUnique())
		}
	}

	// onDisable stops package hooks and transitions from Running back to Loaded
	onDisable := func(ctx context.Context, name string) {
		o.mu.Lock()
		defer o.mu.Unlock()

		if o.packages[name].status.Phase == Running {
			o.queueService.Enqueue(ctx, name, taskdisable.NewTask(name, o.manager, true, o.logger), queue.WithOnDone(func() {
				o.mu.Lock()
				defer o.mu.Unlock()

				if _, ok := o.packages[name]; ok {
					o.packages[name].status.Phase = Loaded
				}
			}), queue.WithUnique())
		}
	}

	o.scheduler = schedule.NewScheduler(
		schedule.WithBootstrapCondition(bootstrapCondition),
		schedule.WithDeckhouseVersionGetter(deckhouseVersionGetter),
		schedule.WithKubeVersionGetter(kubernetesVersionGetter),
		schedule.WithOnEnable(onEnable),
		schedule.WithOnDisable(onDisable))
}

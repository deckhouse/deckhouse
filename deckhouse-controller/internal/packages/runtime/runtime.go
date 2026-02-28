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

package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/Masterminds/semver/v3"
	addonapp "github.com/flant/addon-operator/pkg/app"
	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	klient "github.com/flant/kube-client/client"
	shapp "github.com/flant/shell-operator/pkg/app"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/go-chi/chi/v5"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/cron"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	erofsinstaller "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/installer/erofs"
	symlinkinstaller "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/installer/symlink"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/hookevent"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskapplysettings "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/applysettings"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/run"
	taskstartup "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/startup"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	bootstrappedGlobalValue = "clusterIsBootstrapped"
	kubernetesVersionValue  = "kubernetesVersion"
	deckhouseVersionValue   = "version"

	runtimeTracer = "package-runtime"
)

// Runtime orchestrates the full lifecycle of application packages: discovery,
// installation, hook execution, Helm release management, and removal.
//
// State is split across three structures, all protected by r.mu:
//   - packages (lifecycle.Store): contexts and pending settings — the cancellation
//     and change-detection layer, type-agnostic
//   - apps: loaded Application instances, keyed by name
//   - modules: loaded Module instances, keyed by name
//
// Task execution is delegated to queue.Service (one queue per package), and
// cluster coordination to the scheduler, NELM, and hook event systems.
type Runtime struct {
	hookEventHandler *hookevent.Handler // Routes Kube/schedule events into hook tasks
	queueService     *queue.Service     // Per-package task queues with retry
	nelmService      *nelm.Service      // Helm release management and drift monitoring
	installer        installerI         // Downloads and mounts package images

	status      *status.Service     // Tracks per-package condition chain
	scheduler   *schedule.Scheduler // Evaluates enable/disable based on version constraints
	debugServer *debug.Server       // Unix socket debug API

	objectPatcher     *objectpatch.ObjectPatcher          // Applies resource patches from hooks
	scheduleManager   schedulemanager.ScheduleManager     // Cron-based schedule triggers
	kubeEventsManager kubeeventsmanager.KubeEventsManager // Watches Kubernetes resources for hooks

	mu       sync.RWMutex
	packages *lifecycle.Store
	apps     map[string]*apps.Application
	modules  map[string]*modules.Module

	addonModuleManager moduleManagerI

	logger *log.Logger
}

// installerI abstracts package image operations (download, mount, unmount).
type installerI interface {
	Download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error
	Install(ctx context.Context, downloaded, deployed, name, version string) error
	Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error
}

// moduleManagerI provides access to global values for version getters and bootstrap checks.
type moduleManagerI interface {
	GetGlobal() *addonmodules.GlobalModule
}

// New creates and initializes a Runtime with all subsystems wired together.
// Blocks until the NELM cache completes its initial sync.
func New(moduleManager moduleManagerI, dc dependency.Container, logger *log.Logger) (*Runtime, error) {
	r := new(Runtime)

	r.apps = make(map[string]*apps.Application)
	r.modules = make(map[string]*modules.Module)
	r.packages = lifecycle.NewStore()

	// Initialize foundational services
	r.addonModuleManager = moduleManager
	r.logger = logger.Named("package-runtime")
	r.scheduleManager = cron.NewManager(r.logger)
	r.queueService = queue.NewService(logger)
	r.status = status.NewService()

	reg := registry.NewService(dc, logger)

	// Default to symlink backend (works everywhere, including MacOS)
	r.installer = symlinkinstaller.NewInstaller(reg, logger)

	// Prefer erofs backend when dm-verity is supported (better integrity guarantees)
	if verity.IsSupported() {
		logger.Info("erofs supported")
		r.installer = erofsinstaller.NewInstaller(reg, logger)
	}

	// Initialize scheduler with enabling/disabling callbacks
	r.buildScheduler(moduleManager)

	// Build NELM service with its own client and runtime cache for resource monitoring
	if err := r.buildNelmService(); err != nil {
		return nil, fmt.Errorf("build nelm service: %w", err)
	}

	// Build object patcher with optimized rate limits for batch operations
	if err := r.buildObjectPatcher(); err != nil {
		return nil, fmt.Errorf("build object patcher: %w", err)
	}

	// Build Kubernetes events manager for watching cluster resources
	if err := r.buildKubeEventsManager(); err != nil {
		return nil, fmt.Errorf("build kube events manager: %w", err)
	}

	r.hookEventHandler = hookevent.NewHandler(hookevent.Config{
		KubeEventsManager: r.kubeEventsManager,
		ScheduleManager:   r.scheduleManager,
		TaskBuilder:       r,
		QueueService:      r.queueService,
	}, r.logger).Start()

	if err := r.registerDebugServer("/tmp/deckhouse-debug.socket"); err != nil {
		return nil, fmt.Errorf("register debug server: %w", err)
	}

	return r, nil
}

// registerDebugServer starts a Unix socket HTTP server exposing debug endpoints
// for package state introspection (/packages/dump, /packages/queues/dump, /packages/render/{name}).
func (r *Runtime) registerDebugServer(sockerPath string) error {
	r.debugServer = debug.NewServer(r.logger)
	if err := r.debugServer.Start(sockerPath); err != nil {
		return fmt.Errorf("start debug server: %w", err)
	}

	r.debugServer.Register(http.MethodGet, "/packages/dump", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)

		w.Write(r.Dump()) //nolint:errcheck
	})

	r.debugServer.Register(http.MethodGet, "/packages/queues/dump", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)

		w.Write(r.queueService.Dump()) //nolint:errcheck
	})

	r.debugServer.Register(http.MethodGet, "/packages/scheduler/dump", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)

		w.Write(r.scheduler.Dump()) //nolint:errcheck
	})

	r.debugServer.Register(http.MethodGet, "/packages/render/{name}", func(w http.ResponseWriter, req *http.Request) {
		packageName := chi.URLParam(req, "name")
		if packageName == "" {
			http.Error(w, "package name is required", http.StatusBadRequest)
			return
		}

		rendered, err := r.renderManifests(req.Context(), packageName)
		if err != nil {
			if errors.Is(err, nelm.ErrPackageNotHelm) {
				http.Error(w, fmt.Sprintf("package %s is not a Helm chart", packageName), http.StatusBadRequest)
				return
			}
			http.Error(w, fmt.Sprintf("render failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rendered)) //nolint:errcheck
	})

	return nil
}

// buildObjectPatcher creates a Kubernetes client optimized for patch operations.
//
// Uses dedicated rate limits (QPS and burst) tuned for batch resource patching.
// Hooks can generate multiple patch operations (create/update/delete resources)
// that need to be applied quickly, so this client has higher throughput limits
// than the general-purpose event watching client.
//
// Also sets a custom timeout for patch operations to prevent hanging on slow API calls.
func (r *Runtime) buildObjectPatcher() error {
	client := klient.New(klient.WithLogger(r.logger.Named("object-patcher-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.ObjectPatcherKubeClientQps, shapp.ObjectPatcherKubeClientBurst)
	client.WithTimeout(shapp.ObjectPatcherKubeClientTimeout)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize object patcher client: %w", err)
	}

	r.objectPatcher = objectpatch.NewObjectPatcher(client, r.logger.Named("object-patcher"))
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
func (r *Runtime) buildKubeEventsManager() error {
	client := klient.New(klient.WithLogger(r.logger.Named("kube-events-manager-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize kube events manager client: %w", err)
	}

	r.kubeEventsManager = kubeeventsmanager.NewKubeEventsManager(context.Background(), client, r.logger.Named("kube-events-manager"))

	// Initialize metric storage for the kube events manager
	// This is required to record metrics during hook initialization and execution
	metricStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithLogger(r.logger.Named("kube-events-metrics")),
		metricsstorage.WithNewRegistry(),
	)
	r.kubeEventsManager.WithMetricStorage(metricStorage)

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
func (r *Runtime) buildNelmService() error {
	client := klient.New(klient.WithLogger(r.logger.Named("nelm-monitor-client")))
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
			r.logger.Error("failed to start cache", "error", err)
		}
	}()

	// Wait for cache to complete initial sync before proceeding
	// This ensures monitors have current resource state from the start
	if !cache.WaitForCacheSync(context.Background()) {
		return fmt.Errorf("cache sync failed")
	}

	r.nelmService = nelm.NewService(cache, r.scheduler.Reschedule, r.logger)

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
func (r *Runtime) buildScheduler(moduleManager moduleManagerI) {
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

	r.scheduler = schedule.NewScheduler(
		schedule.WithBootstrapCondition(bootstrapCondition),
		schedule.WithDeckhouseVersionGetter(deckhouseVersionGetter),
		schedule.WithKubeVersionGetter(kubernetesVersionGetter))
}

// Run starts the scheduler event loop in a background goroutine. It listens for
// schedule and disable events from the scheduler and dispatches them to the
// appropriate handler, driving the enable/disable lifecycle for all packages.
func (r *Runtime) Run() {
	go func() {
		for event := range r.scheduler.Ch() {
			switch event.Kind {
			case schedule.EventSchedule:
				r.schedulePackage(event.Name)
			case schedule.EventDisable:
				r.disablePackage(event.Name)
			default:
			}
		}
	}()
}

// schedulePackage handles scheduler enable events by enqueueing
// ApplySettings → Startup → Run tasks for the named package.
//
// ApplySettings reads the latest pending settings from the Store and validates/applies
// them to the loaded package instance. This is the single point where settings reach
// the runtime — both initial load and settings-only changes flow through here.
//
// The Run task carries an onDone callback that calls scheduler.Complete, letting the
// scheduler know the package has finished its run cycle.
//
// Both schedulePackage and disablePackage use EventSchedule, so enqueueing here
// implicitly cancels any in-flight disable context for the same package.
func (r *Runtime) schedulePackage(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	onDone := queue.WithOnDone(func() {
		r.scheduler.Complete(name)
	})

	ctx := r.packages.HandleEvent(lifecycle.EventSchedule, name)
	settings := r.packages.GetPendingSettings(name)

	if pkg := r.apps[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskapplysettings.NewTask(pkg, settings, r.status, r.logger))
		r.queueService.Enqueue(ctx, name, taskstartup.NewTask(pkg, r.nelmService, r.queueService, r.status, r.logger))
		r.queueService.Enqueue(ctx, name, taskrun.NewTask(pkg, pkg.GetNamespace(), r.nelmService, r.status, r.logger), onDone)
	}

	if pkg := r.apps[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskapplysettings.NewTask(pkg, settings, r.status, r.logger))
		r.queueService.Enqueue(ctx, name, taskstartup.NewTask(pkg, r.nelmService, r.queueService, r.status, r.logger))
		r.queueService.Enqueue(ctx, name, taskrun.NewTask(pkg, modulesNamespace, r.nelmService, r.status, r.logger), onDone)
	}
}

// disablePackage handles scheduler disable events by enqueueing a Disable task that
// tears down the package's hooks and Helm release.
//
// Both disablePackage and schedulePackage use EventSchedule, so enqueueing the disable
// task here implicitly cancels any in-flight startup/run context for the same package.
func (r *Runtime) disablePackage(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx := r.packages.HandleEvent(lifecycle.EventSchedule, name)

	if pkg := r.modules[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, "", true, r.nelmService, r.queueService, r.status, r.logger))
	}

	if pkg := r.apps[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, "", true, r.nelmService, r.queueService, r.status, r.logger))
	}
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
func (r *Runtime) Stop() {
	r.logger.Info("stop operator")

	// Clean up resource monitors
	r.nelmService.StopMonitors()

	// Stop generating new events
	r.scheduleManager.Stop()
	r.kubeEventsManager.Stop()

	// Stop accepting and processing new tasks
	r.hookEventHandler.Stop()
	r.queueService.Stop()

	// Close scheduler event channel
	r.scheduler.Stop()
}

// Status returns package status service for external access
func (r *Runtime) Status() *status.Service {
	return r.status
}

// Scheduler returns package scheduler for external access
func (r *Runtime) Scheduler() *schedule.Scheduler {
	return r.scheduler
}

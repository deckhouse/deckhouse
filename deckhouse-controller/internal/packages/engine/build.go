// Copyright 2026 Flant JSC
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

package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/cron"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/core"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/current"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/grants"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/health"
	globalmodule "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// embeddedModulesDir is the deckhouse image directory of embedded modules, relative to the
	// working directory; the loader resolves the weight prefix (modules/002-deckhouse).
	embeddedModulesDir = "modules"

	// liveTicksPeriod is addon-operator's tick, so the Heartbeat panel and alert keep thresholds.
	liveTicksPeriod = 10 * time.Second
)

// Config is the option the callers decide on.
type Config struct {
	// EnableModuleV2 runs modules and Global in the engine (app.ModuleV2Enabled). Off: modules stay
	// in addon-operator, the engine runs applications only, reads the gate inputs from
	// addon-operator's global values and waits for Resume after addon-operator's converge.
	EnableModuleV2 bool
}

// moduleManager is addon-operator's module manager while EnableModuleV2 is off: the global values
// and the modules it enabled, for the dependency rules.
type moduleManager interface {
	GetGlobal() *addonmodules.GlobalModule
	IsModuleEnabled(name string) bool
}

// diskBackend is a disk backend as the composition root uses it: what lifecycle.Disk takes plus
// the cleanup before boot.
type diskBackend interface {
	Attach(ctx context.Context, repo registry.Remote, instance, packageName, version string) error
	Detach(ctx context.Context, instance string) error
	Cleanup(ctx context.Context, preserve []disk.PreservePackage) error
}

// Engine is the package engine as deckhouse-controller uses it: the core with every service it
// runs on. The facade (facade.go) translates controller terms into the core's inputs and keeps no
// lifecycle state.
type Engine struct {
	cfg    Config
	core   *core.Engine
	logger *log.Logger

	global *globalmodule.Module // global is nil without EnableModuleV2.

	observed *observed.Store
	sink     *statusSink

	// scheduler serves the admission webhooks (admission.go), beside the core's activated objects.
	scheduler *schedule.Scheduler

	// Services the facade calls directly.
	nelm        *nelm.Service
	appsDisk    diskBackend
	modulesDisk diskBackend
	metrics     metricsstorage.Storage

	// Services started by Start and stopped by Stop.
	health            *health.Service
	scheduleManager   *cron.Manager
	kubeEventsManager kubeeventsmanager.KubeEventsManager
	// hookRouter delivers hook events to the lanes the packages' and Global's hooks run in; its
	// event loop (Serve) is one of the goroutines Start runs.
	hookRouter *hooklane.Router
	// ownsLiveTicks: without addon-operator the engine emits deckhouse_live_ticks.
	ownsLiveTicks bool

	// stopOnce: the core's Shutdown must run exactly once. done ends the goroutines Start runs;
	// stopped waits for them.
	started  atomic.Bool
	stopOnce sync.Once
	done     chan struct{}
	stopped  sync.WaitGroup
}

// Build creates the engine with every service it needs. It starts nothing but the core's
// dispatcher; the scheduler is paused until Boot or Resume. moduleManager is required without
// EnableModuleV2 and must be nil with it.
func Build(cli kclient.Client, moduleManager moduleManager, metricStorage metricsstorage.Storage, logger *log.Logger, cfg Config) (*Engine, error) {
	// With EnableModuleV2 the global values are the engine's own; a module manager would be a
	// second, disagreeing source of them.
	if !cfg.EnableModuleV2 && moduleManager == nil {
		return nil, errors.New("applications without EnableModuleV2 read addon-operator's global values: a module manager is required")
	}

	if cfg.EnableModuleV2 && moduleManager != nil {
		return nil, errors.New("with EnableModuleV2 the global values are the engine's own: no module manager is expected")
	}

	logger = logger.Named("package-engine")

	e := &Engine{
		cfg:             cfg,
		logger:          logger,
		metrics:         metricStorage,
		scheduleManager: cron.NewManager(logger),
		sink:            newStatusSink(),
		ownsLiveTicks:   moduleManager == nil,
		done:            make(chan struct{}),
	}

	// The observations wake the status side through the same sink as the core's Notify, so a
	// reconcile sees both halves of a package's status.
	e.observed = observed.New(e.sink.add)

	reg := registry.NewService(dependency.NewDependencyContainer(), logger)
	e.appsDisk, e.modulesDisk = buildDisks(reg, logger)

	patcher, err := buildObjectPatcher(logger)
	if err != nil {
		return nil, fmt.Errorf("build object patcher: %w", err)
	}

	if e.kubeEventsManager, err = buildKubeEventsManager(logger); err != nil {
		return nil, fmt.Errorf("build kube events manager: %w", err)
	}

	shared := sharedManagers{
		patcher:           patcher,
		scheduleManager:   e.scheduleManager,
		kubeEventsManager: e.kubeEventsManager,
		metricStorage:     metricStorage,
	}

	if cfg.EnableModuleV2 {
		// A global hook's "<module>Enabled" signal changes no values checksum, so nothing would
		// compute; the global module reports it and the scheduler rechecks. e.core is read when it
		// fires: hooks run only after Build returned.
		recheck := func() { e.core.Recheck() }

		if e.global, err = loadGlobal(context.Background(), shared, recheck, logger); err != nil {
			return nil, fmt.Errorf("load global: %w", err)
		}
	}

	edit, err := edition.Parse()
	if err != nil {
		return nil, fmt.Errorf("new edition: %w", err)
	}

	e.scheduler = buildScheduler(cli, moduleManager, e.global, edit, logger)

	// Drift reaches the Current through the callback each Upgrade hands the release's monitor.
	if e.nelm, err = buildNelmService(e.observed, logger); err != nil {
		return nil, fmt.Errorf("build nelm service: %w", err)
	}

	orphanService, err := buildOrphanService(logger)
	if err != nil {
		return nil, fmt.Errorf("build orphan service: %w", err)
	}

	crdService, err := buildCRDService(logger)
	if err != nil {
		return nil, fmt.Errorf("build crd service: %w", err)
	}

	// Health is an observation, never a lifecycle input.
	if e.health, err = buildHealthService(e.observeHealth, logger); err != nil {
		return nil, fmt.Errorf("build health service: %w", err)
	}

	// The Router delivers kube and schedule events to the hook lanes of the started packages and
	// Global, and starts a package's lanes, with its Current's or Global's changed(), when
	// its hooks start; their stop ends them, so events reach exactly the running ones. Hook runs
	// record their failures in the observations.
	e.hookRouter = hooklane.NewRouter(e.nelm, e.observed, nil, logger)

	pkgLoader := &packageLoader{
		shared:        shared,
		grantResolver: grants.NewResolver(cli),
		appGlobals:    appGlobalValues(moduleManager, e.global),
		global:        e.global,
		logger:        logger,
	}

	downloaded := app.DownloadedModulesDir()

	deps := core.Deps{
		Reconciler: lifecycle.Deps{
			Disks: lifecycle.Disks{
				Apps:        lifecycle.Disk{Backend: e.appsDisk, Dir: filepath.Join(downloaded, "apps")},
				Modules:     lifecycle.Disk{Backend: e.modulesDisk, Dir: filepath.Join(downloaded, "modules")},
				EmbeddedDir: embeddedModulesDir,
			},
			Load: pkgLoader.load,
			Current: current.Deps{
				Nelm:             e.nelm,
				Hooks:            e.hookRouter,
				Orphans:          orphanPurge{service: orphanService, logger: logger},
				ModulesNamespace: app.NamespaceDeckhouse,
				Logger:           logger,
			},
			Releases: e.nelm,
			Logger:   logger,
		},
		Global: global.Deps{
			Hooks:    e.hookRouter,
			CRDs:     crdService,
			Webhooks: newConversionWebhooks(e.nelm, patcher, logger),
			Logger:   logger,
		},
	}

	e.core = core.New(core.Config{Notify: e.sink.add}, e.scheduler, deps)

	return e, nil
}

// Start starts the services that run independently of the lifecycle. Called once, before Cleanup
// and Boot. Non-blocking. The hook event loop starts before Boot, during which Global's hooks
// start (the restored Currents' follow in their first runs), and before the cron manager, whose
// channel only the loop drains. The hook lanes start with their package's hooks.
func (e *Engine) Start() {
	e.health.Start()

	e.stopped.Add(2)

	go func() {
		defer e.stopped.Done()
		e.hookRouter.Serve(e.done, e.kubeEventsManager.Ch(), e.scheduleManager.Ch())
	}()

	e.scheduleManager.Start()

	go e.runSink()

	e.started.Store(true)
}

// Stop stops the engine and every service. Blocking; safe to call more than once and before Start
// (a lost election), but only after Boot returned. Order:
//  1. the core: stopping Currents and Global stop their hook lanes while the event loop still runs;
//  2. the drift monitors: only a running Current adds one;
//  3. the health service;
//  4. the event sources;
//  5. the goroutines Start runs — the hook event loop (the hook lanes stopped with the core, so no
//     run is left to send an event) and the sink —, then the status queues, which nothing feeds
//     any more.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		e.logger.Info("stop package engine")

		e.core.Shutdown()

		if !e.started.Load() {
			return
		}

		e.nelm.StopMonitors()
		e.health.Stop()

		e.scheduleManager.Stop()
		e.kubeEventsManager.Stop()

		close(e.done)
		e.stopped.Wait()
		e.sink.shutdown()
	})
}

func (e *Engine) observeHealth(name string, event health.Event) {
	e.observed.UpdateHealth(name, observed.Health{
		State:   healthState(event.Health.State),
		Message: event.Health.Message,
	})
}

func healthState(state health.State) observed.HealthState {
	switch state {
	case health.StateScaled:
		return observed.HealthReady
	case health.StateReconciling:
		return observed.HealthProgressing
	case health.StateDegraded:
		return observed.HealthDegraded
	default:
		return observed.HealthUnknown
	}
}

// runSink is the one goroutine behind the status sink: it publishes the settings and the
// maintenance gauge when Notify woke them, re-enqueues every seen name on the resync tick and,
// without addon-operator, counts the live ticks. Each job is best effort and off the Notify path;
// none needs an order of its own, so a slow one only delays the others.
func (e *Engine) runSink() {
	defer e.stopped.Done()

	resync := time.NewTicker(resyncPeriod)
	defer resync.Stop()

	// A nil channel never fires: addon-operator counts the live ticks itself.
	var liveTicks <-chan time.Time
	if e.ownsLiveTicks {
		ticker := time.NewTicker(liveTicksPeriod)
		defer ticker.Stop()

		liveTicks = ticker.C
	}

	published := make(map[string]string, 2)

	for {
		select {
		case <-e.done:
			return
		case <-e.sink.settingsWake:
			e.sink.publishSettings(e.core, published)
		case <-e.sink.maintenanceWake:
			e.publishMaintenance()
		case <-resync.C:
			e.sink.resync()
		case <-liveTicks:
			e.metrics.CounterAdd(metrics.ShellLiveTicks, 1, map[string]string{})
		}
	}
}

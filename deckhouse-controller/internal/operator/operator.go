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
	eventHandler   *taskevent.Handler      // Converts events (Kube/schedule) into tasks
	packageManager *packagemanager.Manager // Manages application packages and hooks
	queueService   *queue.Service          // Task queue for hook execution
	nelmService    *nelm.Service           // Helm release management and monitoring

	objectPatcher     *objectpatch.ObjectPatcher          // Applies resource patches from hooks
	scheduleManager   schedulemanager.ScheduleManager     // Cron-based schedule triggers
	kubeEventsManager kubeeventsmanager.KubeEventsManager // Watches Kubernetes resources

	logger *log.Logger
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
func New(ctx context.Context, logger *log.Logger) (*Operator, error) {
	o := new(Operator)

	// Initialize foundational services
	o.queueService = queue.NewService(ctx, logger)
	o.scheduleManager = schedulemanager.NewScheduleManager(ctx, logger.Named("schedule-manager"))
	o.logger = logger.Named(operatorTracer)

	// Build NELM service with its own client and runtime cache for resource monitoring
	if err := o.buildNelmService(ctx); err != nil {
		return nil, fmt.Errorf("build nelm service: %w", err)
	}

	// Build object patcher with optimized rate limits for batch operations
	if err := o.buildObjectPatcher(); err != nil {
		return nil, fmt.Errorf("build object patcher: %w", err)
	}

	// Build Kubernetes events manager for watching cluster resources
	if err := o.buildKubeEventsManager(ctx); err != nil {
		return nil, fmt.Errorf("build kube events manager: %w", err)
	}

	// Initialize package manager with all dependencies
	o.packageManager = packagemanager.New(packagemanager.Config{
		AppsDir:           appsDir,
		NelmService:       o.nelmService,
		KubeObjectPatcher: o.objectPatcher,
		ScheduleManager:   o.scheduleManager,
		KubeEventsManager: o.kubeEventsManager,
	}, logger)

	// Create event handler to orchestrate event processing
	o.eventHandler = taskevent.NewHandler(taskevent.Config{
		KubeEventsManager: o.kubeEventsManager,
		ScheduleManager:   o.scheduleManager,
		PackageManager:    o.packageManager,
		QueueService:      o.queueService,
	}, logger)

	// Start the event processing loop immediately
	o.eventHandler.Start(ctx)

	return o, nil
}

// Stop performs graceful shutdown of all operator subsystems.
//
// Shutdown order ensures safe termination:
//  1. Stop queue service (no new task processing)
//  2. Stop event handler (no new task generation)
//  3. Stop schedule manager (no new cron triggers)
//  4. Pause Kubernetes event handling (no new resource events)
//  5. Stop NELM monitors (cleanup resource monitoring)
//
// This order prevents new work from entering the system while allowing
// in-flight operations to complete gracefully where possible.
func (o *Operator) Stop() {
	o.logger.Info("stop operator")

	// Stop accepting and processing new tasks
	o.queueService.Stop()
	o.eventHandler.Stop()

	// Stop generating new events
	o.scheduleManager.Stop()
	o.kubeEventsManager.PauseHandleEvents()

	// Clean up resource monitors
	o.nelmService.StopMonitors()
}

// KubeEventsManager returns the Kubernetes events manager for external access.
func (o *Operator) KubeEventsManager() kubeeventsmanager.KubeEventsManager {
	return o.kubeEventsManager
}

// ScheduleManager returns the schedule manager for external access.
func (o *Operator) ScheduleManager() schedulemanager.ScheduleManager {
	return o.scheduleManager
}

// QueueService returns the queue service for external access.
func (o *Operator) QueueService() *queue.Service {
	return o.queueService
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
func (o *Operator) buildKubeEventsManager(ctx context.Context) error {
	client := klient.New(klient.WithLogger(o.logger.Named("kube-events-manager-client")))
	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)

	if err := client.Init(); err != nil {
		return fmt.Errorf("initialize kube events manager client: %w", err)
	}

	o.kubeEventsManager = kubeeventsmanager.NewKubeEventsManager(ctx, client, o.logger.Named("kube-events-manager"))
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
func (o *Operator) buildNelmService(ctx context.Context) error {
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
		if err = cache.Start(ctx); err != nil {
			o.logger.Error("failed to start cache", "error", err)
		}
	}()

	// Wait for cache to complete initial sync before proceeding
	// This ensures monitors have current resource state from the start
	if !cache.WaitForCacheSync(ctx) {
		return fmt.Errorf("cache sync failed")
	}

	o.nelmService = nelm.NewService(ctx, namespace, cache, o.logger)
	return nil
}

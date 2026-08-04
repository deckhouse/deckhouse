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

package release

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-release-controller"

	// maxConcurrentReconciles allows a few releases to be reconciled at once.
	maxConcurrentReconciles = 3
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute

	// defaultRequeueAfter is the retry delay for a release that cannot advance yet, such as
	// one waiting on a window, an approval or a module that is not ready.
	defaultRequeueAfter = 15 * time.Second
	// disabledByIgnorePolicy is the status message of a release its policy refuses to update.
	disabledByIgnorePolicy = `Update disabled by 'Ignore' update policy`

	// outdatedReleasesKeepCount is how many superseded, suspended or skipped releases per
	// module survive cleanup, so an operator can still see recent history.
	outdatedReleasesKeepCount = 3
)

// MetricsUpdater reports the blocked-release metric for a single release.
type MetricsUpdater interface {
	UpdateReleaseMetric(string, releaseUpdater.MetricLabels)
	PurgeReleaseMetric(string)
}

// packageManager hands a module version to the package runtime, which owns the download, the
// on-disk placement and the reload that follow, and answers whether a release's requirements
// hold against the live cluster.
type packageManager interface {
	UpdateModule(repo registry.Remote, module runtime.Module, force bool)
	RemoveModule(name string)
	CheckConstraints(name string, constraints schedule.Constraints) error
}

// RegisterController registers the ModuleRelease controller with the manager. The preflight wait
// group is owned by the caller and gates reconciliation until the startup sync has finished.
func RegisterController(
	ctrlManager ctrlmanager.Manager,
	manager packageManager,
	preflight *sync.WaitGroup,
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer,
	ms metricsstorage.Storage,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:           preflight,
		client:         ctrlManager.GetClient(),
		manager:        manager,
		embeddedPolicy: embeddedPolicy,
		metricStorage:  ms,
		metricsUpdater: releaseUpdater.NewMetricsUpdater(ms, releaseUpdater.ModuleReleaseBlockedMetricName),
		logger:         logger.Named(controllerName),
	}

	if err := ctrlManager.Add(ctrlmanager.RunnableFunc(r.seedReleaseMetrics)); err != nil {
		return fmt.Errorf("add seed release metrics: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(ctrlManager).
		Named(controllerName).
		For(&v1alpha1.ModuleRelease{}).
		// for reconcile documentation if accidentally removed
		Owns(&v1alpha1.ModuleDocumentation{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      new(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// reconciler reconciles ModuleRelease objects.
type reconciler struct {
	init   *sync.WaitGroup
	client client.Client

	manager packageManager

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	metricStorage  metricsstorage.Storage
	metricsUpdater MetricsUpdater

	logger *log.Logger
}

// seedReleaseMetrics publishes the pull gauges for the releases that already exist, so the
// metrics are not blank until each release happens to reconcile.
func (r *reconciler) seedReleaseMetrics(ctx context.Context) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for _, mr := range releases.Items {
		labels := map[string]string{
			metrics.LabelVersion: mr.GetVersion().String(),
			metrics.LabelModule:  mr.GetModuleName(),
		}

		r.metricStorage.GaugeSet(metrics.ModulePullSecondsTotal, mr.Status.PullDuration.Seconds(), labels)
		r.metricStorage.GaugeSet(metrics.ModuleSizeBytesTotal, float64(mr.Status.Size), labels)
	}

	r.logger.Debug("controller is ready")

	return nil
}

// Reconcile dispatches the release to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Debug("reconcile module release", slog.String("release", req.Name))
	mr := new(v1alpha1.ModuleRelease)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, mr); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module release is not found", slog.String("release", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module release", slog.String("release", req.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
	}

	r.metricsUpdater.PurgeReleaseMetric(mr.GetName())

	// handle delete event
	if !mr.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, mr)
	}

	// handle create/update events
	res, err := r.handleCreateOrUpdate(ctx, mr)
	if err != nil {
		r.logger.Warn("handle release", log.Err(err))
	}

	return res, err
}

// handleRelease routes the release by phase. Terminal phases only need their status label
// resynced; a pending release is skipped entirely while a pull override holds the module.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handleRelease")
	defer span.End()

	span.SetAttributes(attribute.String("release", mr.GetName()))
	span.SetAttributes(attribute.String("module", mr.GetModuleName()))
	span.SetAttributes(attribute.String("source", mr.GetModuleSource()))
	span.SetAttributes(attribute.String("phase", mr.GetPhase()))

	res, err := r.preHandleCheck(ctx, mr)
	if err != nil {
		r.logger.Error("failed to update module release before handling", slog.String("release", mr.GetName()), log.Err(err))

		return ctrl.Result{Requeue: true}, nil
	}

	if !res.IsZero() {
		return res, nil
	}

	// add finalizer for metrics reset on deletion (so the release resource will be deleted only after metrics are reset)
	if !controllerutil.ContainsFinalizer(mr, v1alpha1.ModuleReleaseFinalizerMetricsRegistered) {
		controllerutil.AddFinalizer(mr, v1alpha1.ModuleReleaseFinalizerMetricsRegistered)
		if err := r.client.Update(ctx, mr); err != nil {
			r.logger.Error("failed to add metrics finalizer to module release", slog.String("release", mr.GetName()), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, nil
	}

	switch mr.GetPhase() {
	case "":
		mr.Status.Phase = v1alpha1.ModuleReleasePhasePending
		mr.Status.TransitionTime = metav1.NewTime(time.Now().UTC())
		if err = r.client.Status().Update(ctx, mr); err != nil {
			r.logger.Error("failed to update module release status", slog.String("release", mr.GetName()), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}

		// process to the next phase
		return ctrl.Result{Requeue: true}, nil

	case v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended, v1alpha1.ModuleReleasePhaseSkipped:
		if err = r.syncStatusLabel(ctx, mr, strings.ToLower(mr.GetPhase())); err != nil {
			r.logger.Error("failed to update module release status", slog.String("release", mr.GetName()), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, nil

	case v1alpha1.ModuleReleasePhaseDeployed:
		res, err = r.handleDeployedRelease(ctx, mr)
		if err != nil {
			r.releaseLogger(mr).Debug("result of handle deployed release", log.Err(err))

			return res, err
		}

		return res, nil
	}

	// if module pull override exists, don't process pending release, to avoid fs override
	exists, err := utils.ModulePullOverrideExists(ctx, r.client, mr.GetModuleName())
	if err != nil {
		r.logger.Error("failed to get module pull override", slog.String("module", mr.GetModuleName()), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	if exists {
		r.logger.Info("module is overridden, skip release processing", slog.String("module", mr.GetModuleName()))
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// process only pending releases
	res, err = r.handlePendingRelease(ctx, mr)
	if err != nil {
		r.releaseLogger(mr).Debug("result of handle pending release", log.Err(err))

		return res, err
	}

	return res, nil
}

// preHandleCheck backfills the module label every later lookup selects on, requeueing so the
// rest of the flow sees the labelled object.
func (r *reconciler) preHandleCheck(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	if _, ok := mr.Labels[v1alpha1.ModuleReleaseLabelModule]; ok {
		return ctrl.Result{}, nil
	}

	err := ctrlutils.UpdateWithRetry(ctx, r.client, mr, func() error {
		if len(mr.ObjectMeta.Labels) == 0 {
			mr.ObjectMeta.Labels = make(map[string]string, 1)
		}

		mr.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelModule] = mr.GetModuleName()

		return nil
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("update with retry: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

// syncStatusLabel keeps the status label in step with the phase, so release lists can select
// on it. It is a no-op when the label already matches.
func (r *reconciler) syncStatusLabel(ctx context.Context, mr *v1alpha1.ModuleRelease, status string) error {
	if mr.Labels[v1alpha1.ModuleReleaseLabelStatus] == status {
		return nil
	}

	if len(mr.Labels) == 0 {
		mr.Labels = make(map[string]string, 1)
	}

	mr.Labels[v1alpha1.ModuleReleaseLabelStatus] = status

	if err := r.client.Update(ctx, mr); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	return nil
}

// releaseLogger tags a logger with the release's identity, the triple every release path logs.
// The source key is module_source because sloglint reserves source.
func (r *reconciler) releaseLogger(mr *v1alpha1.ModuleRelease) *log.Logger {
	return r.logger.With(
		slog.String("module_name", mr.GetModuleName()),
		slog.String("release_name", mr.GetName()),
		slog.String("module_source", mr.GetModuleSource()),
	)
}

// retryBackoff is the conflict backoff for release writes. Releases are touched from several
// paths at once, so conflicts are frequent and the default backoff waits far too long.
func retryBackoff() *wait.Backoff {
	return &wait.Backoff{
		Steps: 6,
		// magic number
		Duration: 20 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}
}

// newModuleReleaseWithName returns a release stub carrying only a name, enough for the client
// to address an object the caller does not need to read first.
func newModuleReleaseWithName(name string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// isModuleReady reports whether the module is in the Ready phase. A module that cannot be read
// counts as not ready, which holds the release back rather than deploying onto unknown state.
func (r *reconciler) isModuleReady(ctx context.Context, moduleName string) bool {
	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, types.NamespacedName{Name: moduleName}, module); err != nil {
		r.logger.Warn("cannot find module", slog.String("module_name", moduleName), log.Err(err))

		return false
	}

	return module.Status.Phase == v1alpha1.ModulePhaseReady
}

// updateReleaseStatus writes the release's phase and message, stamping the transition time only
// when the phase actually changes. Reaching a terminal phase purges the blocked-release metric,
// which would otherwise keep alerting on a release that can no longer move.
func (r *reconciler) updateReleaseStatus(ctx context.Context, mr *v1alpha1.ModuleRelease, status *v1alpha1.ModuleReleaseStatus) error {
	r.logger.Debug("refresh release status", slog.String("release", mr.GetName()))

	switch status.Phase {
	case v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended, v1alpha1.ModuleReleasePhaseSkipped, v1alpha1.ModuleReleasePhaseTerminating:
		r.metricsUpdater.PurgeReleaseMetric(mr.GetName())
	}

	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, mr, func() error {
		if mr.GetPhase() != status.Phase {
			mr.Status.TransitionTime = metav1.NewTime(time.Now().UTC())
		}

		mr.Status.Phase = status.Phase
		mr.Status.Message = status.Message

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(retryBackoff()))
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	return nil
}

// updateModuleLastReleaseDeployedStatus records on the module whether its newest release made
// it. A failure points at the release, since that is where the detail lives.
func (r *reconciler) updateModuleLastReleaseDeployedStatus(ctx context.Context, mr *v1alpha1.ModuleRelease, msg, reason string, deployed bool) error {
	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mr.GetModuleName()}, module); err != nil {
		return fmt.Errorf("get module: %w", err)
	}

	r.logger.With(slog.String("module", mr.GetModuleName())).Debug("refresh module status")

	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, module, func() error {
		if deployed {
			module.SetConditionTrue(v1alpha1.ModuleConditionLastReleaseDeployed, v1alpha1.WithTimer(time.Now))

			return nil
		}

		condMessage := fmt.Sprintf("%s: see details in the module release v%s", msg, mr.GetVersion().String())
		module.SetConditionFalse(v1alpha1.ModuleConditionLastReleaseDeployed, reason, condMessage, v1alpha1.WithTimer(time.Now))

		return nil
	})
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	return nil
}

// updateReleaseStatusMessage sets the release's status message, skipping the write when it is
// already set so a steady state does not churn status.
func (r *reconciler) updateReleaseStatusMessage(ctx context.Context, mr *v1alpha1.ModuleRelease, message string) error {
	if mr.GetMessage() == message {
		return nil
	}

	mr.Status.Message = message

	if err := r.client.Status().Update(ctx, mr); err != nil {
		return fmt.Errorf("update the '%s' module release status: %w", mr.GetName(), err)
	}

	return nil
}

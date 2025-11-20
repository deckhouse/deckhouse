// Copyright 2023 Flant JSC
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
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/shell-operator/pkg/metric"
	cp "github.com/otiai10/copy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-release-controller"

	delayTimer = 3 * time.Second

	maxConcurrentReconciles = 3
	cacheSyncTimeout        = 3 * time.Minute

	defaultCheckInterval   = 15 * time.Second
	disabledByIgnorePolicy = `Update disabled by 'Ignore' update policy`

	outdatedReleasesKeepCount = 3
)

func RegisterController(
	runtimeManager manager.Manager,
	mm moduleManager,
	dc dependency.Container,
	exts *extenders.ExtendersStack,

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer,
	ms metric.Storage,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               runtimeManager.GetClient(),
		log:                  logger,
		moduleManager:        mm,
		metricStorage:        ms,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		embeddedPolicy:       embeddedPolicy,
		delayTimer:           time.NewTimer(delayTimer),
		dependencyContainer:  dc,
		exts:                 exts,
		metricsUpdater:       releaseUpdater.NewMetricsUpdater(ms, releaseUpdater.ModuleReleaseBlockedMetricName),
	}

	r.init.Add(1)

	// add preflight
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	releaseController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModuleRelease{}).
		// for reconcile documentation if accidentally removed
		Owns(&v1alpha1.ModuleDocumentation{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(releaseController)
}

type MetricsUpdater interface {
	UpdateReleaseMetric(string, releaseUpdater.MetricLabels)
	PurgeReleaseMetric(string)
}

type reconciler struct {
	init                *sync.WaitGroup
	client              client.Client
	log                 *log.Logger
	dependencyContainer dependency.Container
	exts                *extenders.ExtendersStack

	embeddedPolicy       *helpers.ModuleUpdatePolicySpecContainer
	moduleManager        moduleManager
	metricStorage        metric.Storage
	downloadedModulesDir string
	symlinksDir          string
	restartReason        string
	clusterUUID          string
	mtx                  sync.Mutex
	delayTimer           *time.Timer

	metricsUpdater MetricsUpdater
}

type moduleManager interface {
	DisableModuleHooks(moduleName string)
	GetModule(moduleName string) *addonmodules.BasicModule
	RunModuleWithNewOpenAPISchema(moduleName, moduleSource, modulePath string) error
	GetEnabledModuleNames() []string
	AreModulesInited() bool
}

func (r *reconciler) preflight(ctx context.Context) error {
	defer r.init.Done()

	// wait until module manager init
	r.log.Debug("wait until module manager is inited")
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return r.moduleManager.AreModulesInited(), nil
	}); err != nil {
		return fmt.Errorf("init module manager: %w", err)
	}

	r.clusterUUID = utils.GetClusterUUID(ctx, r.client)

	go r.restartLoop(ctx)

	// register metrics
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for _, release := range releases.Items {
		labels := map[string]string{
			"version": release.GetVersion().String(),
			"module":  release.GetModuleName(),
		}

		r.metricStorage.GaugeSet("{PREFIX}module_pull_seconds_total", release.Status.PullDuration.Seconds(), labels)
		r.metricStorage.GaugeSet("{PREFIX}module_size_bytes_total", float64(release.Status.Size), labels)
	}

	r.log.Debug("controller is ready")

	return nil
}

func (r *reconciler) restartLoop(ctx context.Context) {
	for {
		r.mtx.Lock()
		select {
		case <-r.delayTimer.C:
			if r.restartReason != "" {
				r.log.Info("restart Deckhouse", slog.String("reason", r.restartReason))
				if err := syscall.Kill(1, syscall.SIGUSR2); err != nil {
					r.log.Fatal("send SIGUSR2 signal failed", log.Err(err))
				}
			}
			r.delayTimer.Reset(delayTimer)

		case <-ctx.Done():
			return
		}
		r.mtx.Unlock()
	}
}

func (r *reconciler) emitRestart(msg string) {
	r.mtx.Lock()
	r.delayTimer.Reset(delayTimer)
	r.restartReason = msg
	r.mtx.Unlock()
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.log.Debug("reconciling module release", slog.String("release", req.Name))
	release := new(v1alpha1.ModuleRelease)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, release); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warn("module release is not found", slog.String("release", req.Name))
			return ctrl.Result{}, nil
		}
		r.log.Error("failed to get module release", slog.String("release", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !release.DeletionTimestamp.IsZero() {
		return r.deleteRelease(ctx, release)
	}

	// handle create/update events
	res, err := r.handleRelease(ctx, release)
	if err != nil {
		r.log.Warn("handle release", log.Err(err))
	}

	return res, err
}

// handleRelease handles releases
func (r *reconciler) handleRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handleRelease")
	defer span.End()

	span.SetAttributes(attribute.String("release", release.GetName()))
	span.SetAttributes(attribute.String("module", release.GetModuleName()))
	span.SetAttributes(attribute.String("source", release.GetModuleSource()))
	span.SetAttributes(attribute.String("phase", release.GetPhase()))

	res, err := r.preHandleCheck(ctx, release)
	if err != nil {
		r.log.Error("failed to update module release before handling", slog.String("release", release.GetName()), log.Err(err))

		return ctrl.Result{Requeue: true}, nil
	}

	if !res.IsZero() {
		return res, nil
	}

	switch release.GetPhase() {
	case "":
		release.Status.Phase = v1alpha1.ModuleReleasePhasePending
		release.Status.TransitionTime = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
		if err = r.client.Status().Update(ctx, release); err != nil {
			r.log.Error("failed to update module release status", slog.String("release", release.GetName()), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}
		// process to the next phase
		return ctrl.Result{Requeue: true}, nil

	case v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended, v1alpha1.ModuleReleasePhaseSkipped:
		if len(release.Labels) == 0 || (release.Labels[v1alpha1.ModuleReleaseLabelStatus] != strings.ToLower(release.GetPhase())) {
			if len(release.Labels) == 0 {
				release.Labels = make(map[string]string)
			}
			release.Labels[v1alpha1.ModuleReleaseLabelStatus] = strings.ToLower(release.GetPhase())
			if err = r.client.Update(ctx, release); err != nil {
				r.log.Error("failed to update module release status", slog.String("release", release.GetName()), log.Err(err))
				return ctrl.Result{Requeue: true}, nil
			}
		}

		return ctrl.Result{}, nil

	case v1alpha1.ModuleReleasePhaseDeployed:
		res, err := r.handleDeployedRelease(ctx, release)
		if err != nil {
			r.log.With(
				slog.String("module_name", release.GetModuleName()),
				slog.String("release_name", release.GetName()),
				slog.String("source", release.GetModuleSource()),
			).Debug("result of handle deployed release", log.Err(err))

			return res, err
		}

		return res, nil
	}

	// if module pull override exists, don't process pending release, to avoid fs override
	exists, err := utils.ModulePullOverrideExists(ctx, r.client, release.GetModuleName())
	if err != nil {
		r.log.Error("failed to get module pull override", slog.String("module", release.GetModuleName()), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}
	if exists {
		r.log.Info("module is overridden, skip release processing", slog.String("module", release.GetModuleName()))
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// process only pending releases
	res, err = r.handlePendingRelease(ctx, release)
	if err != nil {
		r.log.With(
			slog.String("module_name", release.GetModuleName()),
			slog.String("release_name", release.GetName()),
			slog.String("source", release.GetModuleSource()),
		).Debug("result of handle pending release", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *reconciler) preHandleCheck(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	// pre-handling check for important values
	if _, ok := release.Labels[v1alpha1.ModuleReleaseLabelModule]; !ok {
		err := ctrlutils.UpdateWithRetry(ctx, r.client, release, func() error {
			if len(release.ObjectMeta.Labels) == 0 {
				release.ObjectMeta.Labels = make(map[string]string, 1)
			}

			release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelModule] = release.GetModuleName()

			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// patchManualRelease modify deckhouse release with approved status
func (r *reconciler) patchManualRelease(ctx context.Context, release *v1alpha1.ModuleRelease, us *releaseUpdater.Settings) error {
	if us.Mode.String() != v1alpha2.UpdateModeManual.String() {
		return nil
	}

	patch := client.MergeFrom(release.DeepCopy())

	release.SetApprovedStatus(release.GetManuallyApproved())

	err := r.client.Status().Patch(ctx, release, patch)
	if err != nil {
		return fmt.Errorf("patch approved status: %w", err)
	}

	return nil
}

// handleDeployedRelease manages the lifecycle and maintenance of successfully deployed module releases.
// This function ensures deployed releases remain in a consistent, operational state while handling
// various post-deployment scenarios including reloads, registry updates, cleanup operations, and
// status synchronization with dependent Kubernetes resources.
//
// Processing Pipeline:
//  1. Pending Release Detection: Check for conflicting pending releases that may affect readiness
//  2. Module Readiness Updates: Update module conditions based on deployment and pending states
//  3. Administrative Actions: Handle reload requests and registry specification changes
//  4. Metadata Maintenance: Ensure proper finalizers, labels, and annotations are present
//  5. Source Finalizer Management: Protect module sources from deletion while releases exist
//  6. Override Detection: Respect module pull overrides that may bypass normal processing
//  7. Documentation Updates: Synchronize module documentation with deployed release version
//  8. Cleanup Operations: Remove outdated releases while preserving required retention count
//  9. Settings Ownership: Maintain proper ownership of ModuleSettingsDefinition resources
//
// Pending Release Impact on Readiness:
//   - If pending releases exist with lower versions: Module readiness remains uncertain
//   - If no conflicting pending releases: Module is considered fully ready
//   - Readiness state affects whether new releases can be deployed safely
//
// Administrative Operations:
//   - Reload Requests: Triggered by 'reload=true' annotation, forces module re-deployment
//   - Registry Updates: Handles changes to registry configuration requiring OpenAPI schema refresh
//   - Both operations trigger immediate Deckhouse restart for module activation
//
// Resource Ownership and Protection:
//   - Deployed releases add finalizers to prevent premature deletion
//   - ModuleSource resources gain finalizers to prevent deletion while releases exist
//   - ModuleSettingsDefinition ownership is established for proper lifecycle management
//   - Documentation resources are linked to releases for coordinated updates
//
// Override Handling:
//   - ModulePullOverride resources can bypass normal release processing
//   - When overrides exist, deployed releases skip most maintenance operations
//   - Override detection prevents conflicts between manual and automated operations
//
// Example Scenarios:
//
//	Scenario 1 - Standard Deployed Release Maintenance:
//	Input: Deployed v1.68.0, No pending releases, No overrides
//	Flow: Readiness✓→Metadata✓→Documentation✓→Cleanup✓→Settings✓
//	Result: RequeueAfter 0s, all maintenance completed
//
//	Scenario 2 - Reload Request Processing:
//	Input: Deployed v1.68.0 with reload=true annotation
//	Flow: Reload Detection→Module Re-deployment→Restart Trigger
//	Result: RequeueAfter 0s, modulesChangedReason set
//
//	Scenario 3 - Registry Update Handling:
//	Input: Deployed v1.68.0 with registrySpecChanged annotation
//	Flow: Registry Detection→OpenAPI Update→Annotation Cleanup→Update
//	Result: RequeueAfter via requeue=true, registry changes applied
//
//	Scenario 4 - Override Bypass:
//	Input: Deployed v1.68.0, ModulePullOverride exists
//	Flow: Override Detection→Early Return
//	Result: RequeueAfter 0s, minimal processing
func (r *reconciler) handleDeployedRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handleDeployedRelease")
	defer span.End()

	res := ctrl.Result{}

	var needsUpdate bool

	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			r.emitRestart(modulesChangedReason)
		}
	}()

	moduleReleases := new(v1alpha1.ModuleReleaseList)
	labelSelector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: release.GetModuleSource(), v1alpha1.ModuleReleaseLabelModule: release.GetModuleName()}

	err := r.client.List(ctx, moduleReleases, labelSelector)
	if err != nil {
		return res, fmt.Errorf("list module releases: %w", err)
	}

	pendingReleaseFound := false
	for _, rel := range moduleReleases.Items {
		// if pending release version is lower than deployed
		// it will be skipped later in reconcile cycle
		if rel.Status.Phase == v1alpha1.ModuleReleasePhasePending && release.GetVersion().GreaterThan(rel.GetVersion()) {
			pendingReleaseFound = true
		}
	}

	r.dependencyContainer.GetClock().Now()

	if !pendingReleaseFound {
		err = r.updateModuleLastReleaseDeployedStatus(ctx, release, "", "", true)
		if err != nil {
			return res, fmt.Errorf("update module last release deployed status: %w", err)
		}
	}

	if release.GetReinstall() {
		err := r.runReleaseDeploy(ctx, release, nil)
		if err != nil {
			return res, fmt.Errorf("run release deploy: %w", err)
		}

		modulesChangedReason = "module release reloaded"

		return res, nil
	}

	if len(release.Annotations) == 0 {
		release.Annotations = make(map[string]string, 1)
	}

	if release.GetIsUpdating() {
		needsUpdate = true

		if r.isModuleReady(ctx, release.GetModuleName()) {
			release.Annotations[v1alpha1.ModuleReleaseAnnotationIsUpdating] = "false"
		}
	}

	// check if RegistrySpecChanged annotation is set process it
	if _, set := release.GetAnnotations()[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged]; set {
		// if module is enabled - push runModule task in the main queue
		r.log.Info("apply new registry settings to module", slog.String("module", release.GetModuleName()))

		modulePath := filepath.Join(r.downloadedModulesDir, release.GetModuleName(), fmt.Sprintf("v%s", release.GetVersion()))
		source := release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelSource]

		if err := r.moduleManager.RunModuleWithNewOpenAPISchema(release.GetModuleName(), source, modulePath); err != nil {
			r.log.Error("failed to run module with new openAPI schema", slog.String("module", release.GetModuleName()), log.Err(err))

			return res, fmt.Errorf("run module with new open api schema: %w", err)
		}

		// delete annotation and requeue
		delete(release.ObjectMeta.Annotations, v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged)
		needsUpdate = true
	}

	// add finalizer
	if !controllerutil.ContainsFinalizer(release, v1alpha1.ModuleReleaseFinalizerExistOnFs) {
		controllerutil.AddFinalizer(release, v1alpha1.ModuleReleaseFinalizerExistOnFs)
		needsUpdate = true
	}

	if len(release.Labels) == 0 || (release.Labels[v1alpha1.ModuleReleaseLabelStatus] != v1alpha1.ModuleReleaseLabelDeployed) {
		if len(release.ObjectMeta.Labels) == 0 {
			release.ObjectMeta.Labels = make(map[string]string)
		}
		release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelStatus] = v1alpha1.ModuleReleaseLabelDeployed
		needsUpdate = true
	}

	if needsUpdate {
		if err := r.client.Update(ctx, release); err != nil {
			r.log.Error("failed to update module release", slog.String("release", release.GetName()), log.Err(err))

			return res, fmt.Errorf("update module release: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// at least one release for module source is deployed, add finalizer to prevent module source deletion
	source := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
		r.log.Error("failed to get module source", slog.String("module_source", release.GetModuleSource()), log.Err(err))

		return res, fmt.Errorf("get module source: %w", err)
	}

	if !controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists) {
		controllerutil.AddFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists)
		if err := r.client.Update(ctx, source); err != nil {
			r.log.Error("failed to add finalizer to module source", slog.String("module_source", release.GetModuleSource()), log.Err(err))

			return res, fmt.Errorf("add finalizer to module source: %w", err)
		}
	}

	// checks if the module release is overridden by modulepulloverride
	exists, err := utils.ModulePullOverrideExists(ctx, r.client, release.GetModuleName())
	if err != nil {
		r.log.Error("failed to get module pull override", slog.String("module", release.GetModuleName()), log.Err(err))

		return res, fmt.Errorf("module pull override exists: %w", err)
	}
	if exists {
		r.log.Debug("module is overridden, skip it", slog.String("module", release.GetModuleName()))

		return res, nil
	}

	modulePath := fmt.Sprintf("/%s/v%s", release.GetModuleName(), release.GetVersion().String())
	moduleVersion := "v" + release.GetVersion().String()

	moduleChecksum := release.Labels[v1alpha1.ModuleReleaseLabelReleaseChecksum]
	if moduleChecksum == "" {
		moduleChecksum = fmt.Sprintf("%x", md5.Sum([]byte(moduleVersion)))
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
		Kind:       v1alpha1.ModuleReleaseGVK.Kind,
		Name:       release.GetName(),
		UID:        release.GetUID(),
		Controller: ptr.To(true),
	}

	// mpo not found - update the docs from the module release version
	if err = utils.EnsureModuleDocumentation(ctx, r.client, release.GetModuleName(), release.GetModuleSource(), moduleChecksum, moduleVersion, modulePath, ownerRef); err != nil {
		r.log.Error("failed to ensure module documentation", slog.String("module", release.GetModuleName()), log.Err(err))

		return res, fmt.Errorf("ensure module documentation: %w", err)
	}

	r.log.Debug("delete outdated releases for module", slog.String("module", release.GetModuleName()))
	if err = r.deleteOutdatedModuleReleases(ctx, release.GetModuleSource(), release.GetModuleName()); err != nil {
		r.log.Error("failed to delete outdated module releases", slog.String("module", release.GetModuleName()), log.Err(err))

		return res, fmt.Errorf("delete outdated module releases: %w", err)
	}

	settings := new(v1alpha1.ModuleSettingsDefinition)
	if err = r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleName()}, settings); err != nil {
		if !apierrors.IsNotFound(err) {
			return res, fmt.Errorf("get module settings: %w", err)
		}
		r.log.Warn("module settings not found", slog.String("module", release.GetModuleName()))

		return res, nil
	}

	settings.OwnerReferences = []metav1.OwnerReference{ownerRef}

	if err = r.client.Update(ctx, settings); err != nil {
		r.log.Warn("failed to update module settings", slog.String("module", release.GetModuleName()), log.Err(err))

		return res, err
	}

	return res, nil
}

// handlePendingRelease orchestrates the processing of pending module releases through a comprehensive
// evaluation pipeline. This function implements the core release deployment logic that balances
// safety, operational windows, approvals, and technical constraints to determine when and how
// a pending release should be deployed.
//
// Processing Pipeline:
//  1. Update Policy Resolution: Determine applicable update policies and validation rules
//  2. Task Calculation: Evaluate release precedence, constraints, and readiness
//  3. Force Release Handling: Process administratively forced releases bypassing normal flow
//  4. Task Type Processing: Handle Skip/Await/Process decisions from task calculator
//  5. Module Readiness Check: Ensure target module is in stable state for updates
//  6. Requirements Validation: Verify technical prerequisites and compatibility
//  7. Pre-Apply Checks: Validate deployment timing, windows, and approvals
//  8. Release Deployment: Execute the actual module deployment process
//
// Update Policy Resolution:
//   - If release has associated policy label: retrieve and validate specified policy
//   - If no policy specified: auto-discover appropriate policy based on module name
//   - Handle missing policies with graceful degradation and user feedback
//   - Support for manual approval workflows and ignore policies
//
// Task Calculation Results:
//   - Process: Release is ready for deployment (passes all checks)
//   - Skip: Release should be bypassed (superseded by newer/force releases)
//   - Await: Release must wait for dependencies (previous releases, constraints)
//
// Force Release Workflow:
//   - Bypasses all safety checks (windows, requirements, approvals)
//   - Intended for emergency deployments and administrative overrides
//   - Logs warnings for audit trail and operational awareness
//   - Triggers immediate Deckhouse restart for rapid deployment
//
// Module Readiness Requirements:
//   - Non-single releases: Must wait for currently deployed module to be ready
//   - Patch releases: Can proceed if target module is available
//   - Major/minor releases: Stricter readiness requirements for stability
//   - Prevents cascading failures during module transitions
//
// Technical Requirements Validation:
//   - Kubernetes version compatibility checks
//   - Cluster resource availability verification
//   - Dependency module status validation
//   - Custom requirement extensions through pluggable checkers
//
// Pre-Apply Deployment Checks:
//   - Maintenance window compliance for disruption minimization
//   - Manual approval workflows for controlled deployments
//   - Notification delivery for stakeholder awareness
//   - Cooldown period enforcement between major releases
//   - Canary deployment scheduling for gradual rollouts
//
// Side Effects:
//   - Module filesystem changes (download, symlink updates)
//   - Kubernetes resource status updates (release, module conditions)
//   - Deckhouse restart triggers for module activation
//   - Notification delivery to configured channels
//   - Metric updates for operational monitoring
//
// Example Scenarios:
//
//	Scenario 1 - Successful Minor Release:
//	Input: Pending v1.68.0, Policy: Auto, Windows: [9-17], Module: Ready
//	Flow: Policy→Task(Process)→Ready✓→Requirements✓→Windows✓→Deploy→Restart
//	Result: RequeueAfter 15s, modulesChangedReason set
//
//	Scenario 2 - Awaiting Previous Release:
//	Input: Pending v1.68.0, Previous v1.67.0 still Pending
//	Flow: Policy→Task(Await)→Status Update
//	Result: RequeueAfter 15s, no deployment
//
//	Scenario 3 - Force Release Emergency:
//	Input: Pending v1.68.0 with force=true annotation
//	Flow: Policy→Task(Process)→Force Detected→Immediate Deploy
//	Result: No requeue, immediate restart triggered
//
//	Scenario 4 - Manual Approval Required:
//	Input: Pending v2.0.0, Policy: Manual, Approved: false
//	Flow: Policy→Task(Process)→Ready✓→Requirements✓→Approval✗
//	Result: RequeueAfter 15s, awaiting approval
func (r *reconciler) handlePendingRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handlePendingRelease")
	defer span.End()

	var res ctrl.Result

	logger := r.log.With(
		slog.String("module_name", release.GetModuleName()),
		slog.String("release_name", release.GetName()),
		slog.String("source", release.GetModuleSource()),
	)

	logger.Debug("handle pending release")

	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			r.emitRestart(modulesChangedReason)
		}
	}()

	var (
		policy *v1alpha2.ModuleUpdatePolicy
		err    error
	)

	// if release has associated update policy
	policyName, found := release.GetObjectMeta().GetLabels()[v1alpha1.ModuleReleaseLabelUpdatePolicy]
	if found {
		policy, err = r.getUpdatePolicy(ctx, policyName)
		if err != nil {
			r.metricStorage.CounterAdd("{PREFIX}module_update_policy_not_found", 1.0, map[string]string{
				"version":        release.GetReleaseVersion(),
				"module_release": release.GetName(),
				"module":         release.GetModuleName(),
			})

			if err := r.updateReleaseStatusMessage(ctx, release, fmt.Sprintf("Update policy %s not found", policyName)); err != nil {
				logger.Error("failed to update release status", log.Err(err))

				return res, err
			}

			logger.Error("failed to get update policy", slog.String("policy", policyName), log.Err(err))

			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		// TODO(ipaqsa): remove it
		if policy.Spec.Update.Mode == v1alpha2.ModuleUpdatePolicyModeIgnore {
			if err := r.updateReleaseStatusMessage(ctx, release, disabledByIgnorePolicy); err != nil {
				logger.Error("failed to update release status", slog.String("release", release.GetName()), log.Err(err))

				return res, err
			}

			return ctrl.Result{RequeueAfter: defaultCheckInterval * 4}, nil
		}
	} else {
		var policyRes *ctrl.Result
		policy, policyRes, err = r.updatePolicy(ctx, release)
		if err != nil {
			return res, err
		}

		if policyRes != nil {
			return *policyRes, nil
		}
	}

	// parse notification config from the deckhouse-discovery secret
	config, err := utils.GetNotificationConfig(ctx, r.client)
	if err != nil {
		logger.Error("failed to parse the notification config", log.Err(err))

		return res, err
	}

	us := &releaseUpdater.Settings{
		NotificationConfig: config,
		Mode:               v1alpha2.ParseUpdateMode(policy.Spec.Update.Mode),
		Windows:            policy.Spec.Update.Windows,
		Subject:            releaseUpdater.SubjectModule,
	}

	err = r.patchManualRelease(ctx, release, us)
	if err != nil {
		return res, err
	}

	taskCalculator := releaseUpdater.NewModuleReleaseTaskCalculator(r.client, logger)

	task, err := taskCalculator.CalculatePendingReleaseTask(ctx, release)
	if err != nil {
		return res, err
	}

	if release.GetForce() {
		logger.Warn("forced release found")

		// deploy forced release without any checks (windows, requirements, approvals and so on)
		if err = r.ApplyRelease(ctx, release, task); err != nil {
			logger.Error("apply forced release", log.Err(err))
			return res, fmt.Errorf("apply forced release: %w", err)
		}

		modulesChangedReason = "a new module release deployed"

		// stop requeue because we restart deckhouse (deployment)
		return ctrl.Result{}, nil
	}

	switch task.TaskType {
	case releaseUpdater.Process:
		// pass
	case releaseUpdater.Skip:
		logger.Debug("skip pending release")

		err = r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSkipped,
			Message: task.Message,
		})
		if err != nil {
			logger.Warn("skip order status update ", slog.String("release", release.GetName()), log.Err(err))
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		return res, nil
	case releaseUpdater.Await:
		logger.Debug("await pending release")

		err = r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: task.Message,
		})
		if err != nil {
			logger.Warn("await order status update ", log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	if !task.IsSingle && !task.IsPatch && !r.isModuleReady(ctx, release.GetModuleName()) {
		logger.Debug("module is not ready, waiting")

		drs := &v1alpha1.ModuleReleaseStatus{
			Phase: v1alpha1.ModuleReleasePhasePending,
		}

		drs.Message = "awaiting for module to be ready"

		if task.DeployedReleaseInfo != nil {
			drs.Message = fmt.Sprintf("awaiting for module v%s to be ready", task.DeployedReleaseInfo.Version.String())
		}

		updateErr := r.updateReleaseStatus(ctx, release, drs)
		if updateErr != nil {
			logger.Warn("module release status update failed", log.Err(err))
		}

		err := r.updateModuleLastReleaseDeployedStatus(ctx, release, "ModuleRelease could not be applied, awaiting for deployed release be ready", "ReleaseDeployedIsNotReady", false)
		if err != nil {
			return res, fmt.Errorf("update module last release deployed status: %w", err)
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	logger.Debug("process pending release")

	checker, err := releaseUpdater.NewModuleReleaseRequirementsChecker(r.exts, releaseUpdater.WithLogger(logger))
	if err != nil {
		updateErr := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: err.Error(),
		})
		if updateErr != nil {
			logger.Warn("create release checker status update ", log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	metricLabels := releaseUpdater.NewReleaseMetricLabels(release)
	defer func() {
		if metricLabels[releaseUpdater.ManualApprovalRequired] == "true" {
			metricLabels[releaseUpdater.ReleaseQueueDepth] = strconv.Itoa(task.QueueDepth.GetReleaseQueueDepth())
		}
		r.metricsUpdater.UpdateReleaseMetric(release.GetName(), metricLabels)
	}()

	reasons := checker.MetRequirements(ctx, release)
	if len(reasons) > 0 {
		metricLabels.SetTrue(releaseUpdater.RequirementsNotMet)
		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
		}

		err = r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: strings.Join(msgs, ";"),
		})
		if err != nil {
			logger.Warn("met requirements status update ", log.Err(err))
		}

		err := r.updateModuleLastReleaseDeployedStatus(ctx, release, "ModuleRelease could not be applied, not met requirements", "ReleaseRequirementsCheck", false)
		if err != nil {
			return res, fmt.Errorf("update module last release deployed status: %w", err)
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	logger.Debug("requirements checks passed")

	// handling error inside function
	err = r.PreApplyReleaseCheck(ctx, release, task, us, metricLabels)
	if err != nil {
		// ignore this err, just requeue because of check failed
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	logger.Debug("pre apply checks passed")

	err = r.ApplyRelease(ctx, release, task)
	if err != nil {
		return res, fmt.Errorf("apply predicted release: %w", err)
	}

	// no deckhouse restart if dryrun
	if release.GetDryRun() {
		return ctrl.Result{}, nil
	}

	modulesChangedReason = "a new module release deployed"

	logger.Debug("module release deployed")

	return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
}

func (r *reconciler) getUpdatePolicy(ctx context.Context, name string) (*v1alpha2.ModuleUpdatePolicy, error) {
	policy := new(v1alpha2.ModuleUpdatePolicy)

	if name != "" {
		// get policy spec
		if err := r.client.Get(ctx, client.ObjectKey{Name: name}, policy); err != nil {
			return nil, fmt.Errorf("get update policy: %w", err)
		}

		return policy, nil
	}

	return &v1alpha2.ModuleUpdatePolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha2.ModuleUpdatePolicyGVK.Kind,
			APIVersion: v1alpha2.ModuleUpdatePolicyGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Spec: *r.embeddedPolicy.Get(),
	}, nil
}

func (r *reconciler) updatePolicy(ctx context.Context, release *v1alpha1.ModuleRelease) (*v1alpha2.ModuleUpdatePolicy, *ctrl.Result, error) {
	policy, err := utils.UpdatePolicy(ctx, r.client, r.embeddedPolicy, release.GetModuleName())
	if err != nil {
		r.log.Error("failed to get update policy", slog.String("release", release.GetName()), log.Err(err))

		if err := r.updateReleaseStatusMessage(ctx, release, "Update policy not set. Create a suitable ModuleUpdatePolicy object"); err != nil {
			r.log.Error("failed to update release status", slog.String("release", release.GetName()), log.Err(err))

			return nil, nil, err
		}

		return nil, &ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	marshalledPatch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				v1alpha1.ModuleReleaseLabelUpdatePolicy: policy.GetName(),
			},
		},
		"status": map[string]string{
			"message": "",
		},
	})

	patch := client.RawPatch(types.MergePatchType, marshalledPatch)
	if err = r.client.Patch(ctx, release, patch); err != nil {
		r.log.Error("failed to patch module release", slog.String("release", release.GetName()), log.Err(err))

		return nil, &ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}
	// also patch status field
	if err = r.client.Status().Patch(ctx, release, patch); err != nil {
		r.log.Error("failed to patch module release status", slog.String("release", release.GetName()), log.Err(err))

		return nil, &ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	return policy, nil, nil
}

// ApplyRelease applies predicted release
func (r *reconciler) ApplyRelease(ctx context.Context, mr *v1alpha1.ModuleRelease, task *releaseUpdater.Task) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "applyRelease")
	defer span.End()

	var dri *releaseUpdater.ReleaseInfo

	if task != nil {
		dri = task.DeployedReleaseInfo
	}

	err := r.runReleaseDeploy(ctx, mr, dri)
	if err != nil {
		return fmt.Errorf("run release deploy: %w", err)
	}

	return nil
}

// runReleaseDeploy executes the complete module release deployment process from download to activation.
// This function coordinates the essential steps required to safely deploy a new module version while
// maintaining system consistency and providing rollback capabilities through proper status transitions.
//
// Core Deployment Pipeline:
//  1. Module Download: Fetch and validate the specified module version from registry
//  2. Status Transition: Mark previously deployed release as superseded for proper lifecycle
//  3. Metadata Update: Apply deployment annotations and finalizers for resource protection
//  4. Status Finalization: Update release status to deployed with deployment metrics
//
// The function implements a transactional approach where each step includes retry mechanisms
// to ensure deployment consistency even under concurrent modifications or temporary failures.
//
// Module Download Process:
//   - Creates isolated temporary directory for download operations
//   - Fetches module artifacts from configured registry using authentication
//   - Validates module configuration against current cluster values
//   - Copies validated module to permanent location with version-specific path
//   - Updates filesystem symlinks to activate the new module version
//   - Disables previous module hooks to prevent execution during transition
//
// Status Management Strategy:
//   - Previously deployed releases are marked as "superseded" to maintain audit trail
//   - Current release transitions through annotated states for tracking deployment progress
//   - Finalizers protect filesystem resources from premature cleanup
//   - Labels enable efficient querying and monitoring of release states
//
// Deployment States and Annotations:
//   - isUpdating=true: Indicates deployment is in progress
//   - notified=false: Tracks notification delivery status
//   - Status labels updated to reflect deployment state
//   - Finalizers added to protect filesystem resources
//   - Administrative annotations cleared (force, reinstall, applyNow)
//
// Module Validation Process:
//   - Configuration validation against current cluster values or ModuleConfig
//   - Schema validation for module structure and dependencies
//   - Compatibility checks for Kubernetes version requirements
//   - Graceful handling of validation failures with informative status updates
//
// Retry and Resilience:
//   - Exponential backoff for Kubernetes API operations
//   - Separate retry logic for metadata and status updates
//   - Idempotent operations where possible to support safe retries
//   - Detailed error context for debugging and operational support
//
// Example Scenarios:
//
//	Scenario 1 - Initial Module Deployment:
//	Input: Pending v1.68.0, No previous deployment
//	Flow: Download→Validate→Install→Status(Deployed)
//	Result: Module active, metrics updated, no superseded release
//
//	Scenario 2 - Module Version Upgrade:
//	Input: Pending v1.69.0, Currently deployed v1.68.0
//	Flow: Download→Validate→Supersede(v1.68.0)→Install→Status(Deployed)
//	Result: v1.69.0 active, v1.68.0 marked superseded
//
//	Scenario 3 - Module Reload (Same Version):
//	Input: Deployed v1.68.0 with reload annotation
//	Flow: Download→Validate→Reinstall→Status(Deployed)
//	Result: Same version redeployed, configuration refreshed
//
//	Scenario 4 - Validation Failure:
//	Input: Pending v1.69.0 with invalid configuration
//	Flow: Download→Validate✗→Status(Suspended/Pending)
//	Result: Deployment halted, detailed error message provided
func (r *reconciler) runReleaseDeploy(ctx context.Context, release *v1alpha1.ModuleRelease, deployedReleaseInfo *releaseUpdater.ReleaseInfo) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "runReleaseDeploy")
	defer span.End()

	r.log.Info("applying release", slog.String("release", release.GetName()))

	downloadStatistic, err := r.loadModule(ctx, release)
	if err != nil {
		return fmt.Errorf("load module: %w", err)
	}

	if deployedReleaseInfo != nil {
		err = r.updateReleaseStatus(ctx, newModuleReleaseWithName(deployedReleaseInfo.Name), &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSuperseded,
			Message: "",
		})
		if err != nil {
			r.log.Error("update status", slog.String("release", deployedReleaseInfo.Name), log.Err(err))
		}
	}

	backoff := &wait.Backoff{
		Steps: 6,
		// magic number
		Duration: 20 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}

	err = ctrlutils.UpdateWithRetry(ctx, r.client, release, func() error {
		annotations := map[string]string{
			v1alpha1.ModuleReleaseAnnotationIsUpdating: "true",
			v1alpha1.ModuleReleaseAnnotationNotified:   "false",
		}

		if len(release.Annotations) == 0 {
			release.Annotations = make(map[string]string, 2)
		}

		for k, v := range annotations {
			release.Annotations[k] = v
		}

		if len(release.ObjectMeta.Labels) == 0 {
			release.ObjectMeta.Labels = make(map[string]string, 1)
		}

		release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelStatus] = v1alpha1.ModuleReleaseLabelDeployed

		if release.GetApplyNow() {
			delete(release.Annotations, v1alpha1.ModuleReleaseAnnotationApplyNow)
		}

		if release.GetForce() {
			delete(release.Annotations, v1alpha1.ModuleReleaseAnnotationForce)
		}

		if release.GetReinstall() {
			delete(release.Annotations, v1alpha1.ModuleReleaseAnnotationReinstall)
		}

		controllerutil.AddFinalizer(release, v1alpha1.ModuleReleaseFinalizerExistOnFs)

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(backoff))
	if err != nil {
		return fmt.Errorf("update with retry: %w", err)
	}

	err = ctrlutils.UpdateStatusWithRetry(ctx, r.client, release, func() error {
		release.Status.Phase = v1alpha1.ModuleReleasePhaseDeployed
		release.Status.Message = ""

		release.Status.Size = downloadStatistic.Size
		release.Status.PullDuration = metav1.Duration{Duration: downloadStatistic.PullDuration}

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(backoff))
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	return nil
}

func (r *reconciler) runDryRunDeploy(mr *v1alpha1.ModuleRelease) {
	r.log.Debug("dryrun start soon...")

	time.Sleep(3 * time.Second)

	r.log.Debug("dryrun started")

	// because we do not know how long is parent context and how long will be update
	// 1 minute - magic constant
	ctxwt, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	releases := new(v1alpha1.ModuleReleaseList)
	err := r.client.List(ctxwt, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: mr.GetModuleName()})
	if err != nil {
		r.log.Error("dryrun list module releases", slog.String("module_name", mr.GetModuleName()), log.Err(err))

		return
	}

	for _, release := range releases.Items {
		release := &release

		if release.GetName() == mr.GetName() {
			continue
		}

		if release.Status.Phase != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		// update releases to trigger their requeue
		err = ctrlutils.UpdateWithRetry(ctxwt, r.client, release, func() error {
			if len(release.Annotations) == 0 {
				release.Annotations = make(map[string]string, 1)
			}

			release.Annotations[v1alpha1.ModuleReleaseAnnotationTriggeredByDryrun] = mr.GetName()

			return nil
		})
		if err != nil {
			r.log.Error("dryrun update release to requeue", log.Err(err))
		}

		r.log.Debug("dryrun release successfully updated", slog.String("release", release.Name))
	}
}

func (r *reconciler) loadModule(ctx context.Context, release *v1alpha1.ModuleRelease) (*downloader.DownloadStatistic, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "loadModule")
	defer span.End()

	logger := r.log.With(slog.String("module", release.GetModuleName()))

	// dryrun for testing purpose
	if release.GetDryRun() {
		go r.runDryRunDeploy(release)

		return &downloader.DownloadStatistic{}, nil
	}

	// download desired module version
	source := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
		return nil, fmt.Errorf("get the '%s' module source: %w", release.GetModuleSource(), err)
	}

	tmpDir, err := os.MkdirTemp("", "module*")
	if err != nil {
		return nil, fmt.Errorf("create tmp directory: %w", err)
	}

	// clear tmp dir
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			logger.Error("failed to remove old module directory", slog.String("directory", tmpDir), log.Err(err))
		}
	}()

	options := utils.GenerateRegistryOptionsFromModuleSource(source, r.clusterUUID, logger)
	md := downloader.NewModuleDownloader(r.dependencyContainer, tmpDir, source, logger.Named("downloader"), options)

	downloadStatistic, err := md.DownloadByModuleVersion(ctx, release.GetModuleName(), release.GetVersion().String())
	if err != nil {
		return nil, fmt.Errorf("download the '%s/%s' module: %w", release.GetModuleName(), release.GetVersion().String(), err)
	}

	def := &moduletypes.Definition{
		Name:   release.GetModuleName(),
		Weight: release.Spec.Weight,
		Path:   path.Join(tmpDir, release.GetModuleName(), "v"+release.GetVersion().String()),
	}

	// get values from module config
	logger.Debug("get module config")
	config := new(v1alpha1.ModuleConfig)
	err = r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleName()}, config)
	if err != nil {
		return nil, fmt.Errorf("get the '%s' module config: %w", release.GetModuleName(), err)
	}
	values := make(addonutils.Values)
	if err == nil {
		values = addonutils.Values(config.Spec.Settings)
	}

	// check conversions
	conversionsDir := filepath.Join(def.Path, "openapi", "conversions")
	_, err = os.Stat(conversionsDir)
	if err == nil {
		logger.Debug("conversions for the module found")
		values, err = r.applyValuesConversions(def, conversionsDir, config.Spec.Version, values)
		if err != nil {
			status := &v1alpha1.ModuleReleaseStatus{
				Phase:   v1alpha1.ModuleReleasePhaseSuspended,
				Message: "apply conversions failed: " + err.Error(),
			}

			if strings.Contains(err.Error(), "is required") || strings.Contains(err.Error(), "is a forbidden property") {
				r.metricStorage.GaugeSet("{PREFIX}module_configuration_error", 1,
					map[string]string{
						"version": release.GetVersion().String(),
						"module":  release.GetModuleName(),
						"error":   err.Error(),
					},
				)

				status.Phase = v1alpha1.ModuleReleasePhasePending
			}

			if err = r.updateReleaseStatus(ctx, release, status); err != nil {
				return nil, fmt.Errorf("update status: the '%s:v%s' module conversion: %w", release.GetModuleName(), release.GetVersion().String(), err)
			}

			logger.Debug("successfully updated module conditions")
			return nil, fmt.Errorf("apply conversions: %w", err)
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
	}

	configConfigurationErrorMetricsLabels := map[string]string{
		"version": release.GetVersion().String(),
		"module":  release.GetModuleName(),
		"error":   "",
	}

	if err = def.Validate(values, logger); err != nil {
		status := &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSuspended,
			Message: "validation failed: " + err.Error(),
		}

		if strings.Contains(err.Error(), "is required") ||
			strings.Contains(err.Error(), "is a forbidden property") {
			configConfigurationErrorMetricsLabels["error"] = err.Error()
			r.metricStorage.GaugeSet("{PREFIX}module_configuration_error",
				1,
				configConfigurationErrorMetricsLabels,
			)

			status.Phase = v1alpha1.ModuleReleasePhasePending
			status.Message = "Initial module config validation failed:\n" + err.Error()

			logger.Debug("successfully updated module conditions")
		}

		if err := r.updateReleaseStatus(ctx, release, status); err != nil {
			return nil, fmt.Errorf("update status: the '%s:v%s' module validation: %w", release.GetModuleName(), release.GetVersion().String(), err)
		}

		moduleErr := r.updateModuleLastReleaseDeployedStatus(ctx, release, "ModuleRelease could not be applied, module config validation failed", "ReleaseConfigValidationCheck", false)
		if moduleErr != nil {
			return nil, fmt.Errorf("update module last release deployed status: %w", moduleErr)
		}

		return nil, fmt.Errorf("the '%s:v%s' module validation: %w", release.GetModuleName(), release.GetVersion().String(), err)
	}

	r.metricStorage.GaugeSet("{PREFIX}module_configuration_error",
		0,
		configConfigurationErrorMetricsLabels,
	)

	moduleVersionPath := path.Join(r.downloadedModulesDir, release.GetModuleName(), "v"+release.GetVersion().String())
	if err = os.RemoveAll(moduleVersionPath); err != nil {
		return nil, fmt.Errorf("remove the '%s' old module dir: %w", moduleVersionPath, err)
	}

	if err = cp.Copy(def.Path, moduleVersionPath); err != nil {
		return nil, fmt.Errorf("copy module dir: %w", err)
	}

	// search symlink for module by regexp
	// module weight for a new version of the module may be different from the old one,
	// we need to find a symlink that contains the module name without looking at the weight prefix.
	currentModuleSymlink, err := utils.GetModuleSymlink(r.symlinksDir, release.GetModuleName())
	if err != nil {
		r.log.Warn("failed to find the current module symlink", slog.String("module", release.GetModuleName()), log.Err(err))

		currentModuleSymlink = "900-" + release.GetModuleName() // fallback
	}

	newModuleSymlink := path.Join(r.symlinksDir, fmt.Sprintf("%d-%s", def.Weight, release.GetModuleName()))

	relativeModulePath := path.Join("../", release.GetModuleName(), "v"+release.GetVersion().String())

	if err = utils.EnableModule(r.downloadedModulesDir, currentModuleSymlink, newModuleSymlink, relativeModulePath); err != nil {
		return nil, fmt.Errorf("enable the '%s' module: %w", release.GetModuleName(), err)
	}

	// disable target module hooks so as not to invoke them before restart
	if r.moduleManager.GetModule(release.GetModuleName()) != nil {
		r.moduleManager.DisableModuleHooks(release.GetModuleName())
	}

	return downloadStatistic, nil
}

func (r *reconciler) applyValuesConversions(def *moduletypes.Definition, pathToConversions string, fromVersion int, values addonutils.Values) (addonutils.Values, error) {
	// create a temporary store to avoid writing not valid conversions to the main store
	tmpStore := conversion.NewConversionsStore()
	err := tmpStore.Add(def.Name, pathToConversions)
	if err != nil {
		return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
	}
	// apply conversions to values
	r.log.Debug("apply conversions to values",
		slog.Int("from_version", fromVersion),
		slog.Int("to_version", tmpStore.Get(def.Name).LatestVersion()),
		slog.String("module", def.Name))
	_, newValues, err := tmpStore.Get(def.Name).ConvertToLatest(fromVersion, values)
	if err != nil {
		return nil, fmt.Errorf("convert values to latest version: %w", err)
	}
	return newValues, nil
}

var ErrPreApplyCheckIsFailed = errors.New("pre apply check is failed")

// PreApplyReleaseCheck checks final conditions before apply
//
// - Calculating deploy time (if zero - deploy)
func (r *reconciler) PreApplyReleaseCheck(ctx context.Context, mr *v1alpha1.ModuleRelease, task *releaseUpdater.Task, us *releaseUpdater.Settings, metricLabels releaseUpdater.MetricLabels) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "preApplyReleaseCheck")
	defer span.End()

	timeResult := r.DeployTimeCalculate(ctx, mr, task, us, metricLabels)

	if timeResult == nil {
		return nil
	}

	err := r.updateReleaseStatus(ctx, mr, &v1alpha1.ModuleReleaseStatus{
		Phase:   v1alpha1.ModuleReleasePhasePending,
		Message: timeResult.Message,
	})
	if err != nil {
		r.log.Warn("met release conditions status update ", slog.String("release", mr.GetName()), log.Err(err))
	}

	err = r.updateModuleLastReleaseDeployedStatus(ctx, mr, "ModuleRelease could not be applied, release postponed", "ReleaseDeployTimeCheck", false)
	if err != nil {
		return fmt.Errorf("update module last release deployed status: %w", err)
	}

	backoff := &wait.Backoff{
		Steps: 6,
		// magic number
		Duration: 20 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}

	err = ctrlutils.UpdateWithRetry(ctx, r.client, mr, func() error {
		if len(mr.Annotations) == 0 {
			mr.Annotations = make(map[string]string, 2)
		}

		mr.Annotations[v1alpha1.ModuleReleaseAnnotationIsUpdating] = "false"
		mr.Annotations[v1alpha1.ModuleReleaseAnnotationNotified] = strconv.FormatBool(timeResult.Notified)

		if !timeResult.ReleaseApplyAfterTime.IsZero() {
			mr.Spec.ApplyAfter = &metav1.Time{Time: timeResult.ReleaseApplyAfterTime.UTC()}

			mr.Annotations[v1alpha1.ModuleReleaseAnnotationNotificationTimeShift] = "true"
		}

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(backoff))
	if err != nil {
		r.log.Warn("met release conditions resource update ", slog.String("release", mr.GetName()), log.Err(err))
	}

	return ErrPreApplyCheckIsFailed
}

const (
	msgReleaseIsBlockedByNotification = "Release is blocked, failed to send release notification"
)

type TimeResult struct {
	*releaseUpdater.ProcessedDeployTimeResult
	Notified bool
}

// DeployTimeCalculate performs comprehensive timing analysis and notification coordination to determine
// the optimal deployment window for a module release. This function implements differentiated timing
// logic based on release type (patch vs minor) while handling notification delivery, policy compliance,
// and disruption approval requirements.
//
// Processing Pipeline:
//  1. Release Type Analysis: Determine if release is patch or minor/major version
//  2. Disruption Check: For minor releases, validate disruption approval requirements
//  3. Timing Calculation: Calculate deployment timing using specialized time services
//  4. Notification Delivery: Send appropriate notifications based on release type
//  5. Result Processing: Apply policy-specific timing adjustments and scheduling
//
// Patch Release Workflow:
//   - Lower risk profile allows more flexible deployment timing
//   - Evaluated conditions: Canary settings, notifications, maintenance windows, manual approvals
//   - Simplified notification workflow with patch-specific messaging
//   - Immediate deployment possible if all conditions are satisfied
//
// Minor Release Workflow:
//   - Higher risk profile requires additional safety measures
//   - Evaluated conditions: Cooldown periods, canary settings, notifications, windows, approvals
//   - Disruption approval validation through specialized checker
//   - Enhanced notification workflow with detailed change communication
//   - Extended validation period before deployment authorization
//
// Disruption Approval System:
//   - Minor releases undergo disruption impact assessment
//   - Configurable approval requirements based on organizational policies
//   - Blocks deployment until explicit approval is granted
//   - Provides detailed reasoning for approval requirements
//
// Notification Integration:
//   - Patch notifications: Lightweight, focused on immediate changes
//   - Minor notifications: Comprehensive, includes impact assessment and timing
//   - Notification delivery failure blocks deployment for safety
//
// Example Scenarios:
//
//	Scenario 1 - Immediate Patch Deployment:
//	Input: Patch release, within window, notifications enabled
//	Flow: Patch Check→Notify→Calculate→Process
//	Result: nil (immediate deployment approved)
//
//	Scenario 2 - Minor Release with Disruption Block:
//	Input: Minor release, no disruption approval
//	Flow: Minor Check→Disruption✗→Block
//	Result: TimeResult{Message: "disruption approval required"}
//
//	Scenario 3 - Notification Delivery Failure:
//	Input: Any release, notification channel unavailable
//	Flow: Calculate→Notify✗→Block
//	Result: TimeResult{Message: "Release is blocked, failed to send release notification"}
//
//	Scenario 4 - Scheduled Minor Deployment:
//	Input: Minor release, outside window, approved
//	Flow: Minor Check→Disruption✓→Calculate→Notify→Schedule
//	Result: TimeResult{ReleaseApplyAfterTime: next_window_start, Notified: true}
func (r *reconciler) DeployTimeCalculate(ctx context.Context, mr v1alpha1.Release, task *releaseUpdater.Task, us *releaseUpdater.Settings, metricLabels releaseUpdater.MetricLabels) *TimeResult {
	releaseNotifier := releaseUpdater.NewReleaseNotifier(us)
	timeChecker := releaseUpdater.NewDeployTimeService(r.dependencyContainer, us, r.log)

	var deployTimeResult *releaseUpdater.DeployTimeResult

	if task.IsPatch {
		deployTimeResult = timeChecker.CalculatePatchDeployTime(mr, metricLabels)

		notifyErr := releaseNotifier.SendPatchReleaseNotification(ctx, mr, deployTimeResult.ReleaseApplyAfterTime, metricLabels)
		if notifyErr != nil {
			r.log.Warn("send [patch] release notification", log.Err(notifyErr))

			return &TimeResult{
				ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
					Message:               msgReleaseIsBlockedByNotification,
					ReleaseApplyAfterTime: deployTimeResult.ReleaseApplyAfterTime,
				},
			}
		}

		processedDTR := timeChecker.ProcessPatchReleaseDeployTime(mr, deployTimeResult)
		if processedDTR == nil {
			return nil
		}

		return &TimeResult{
			ProcessedDeployTimeResult: processedDTR,
			Notified:                  true,
		}
	}

	// for minor release we must check additional conditions
	checker := releaseUpdater.NewPreApplyChecker(us, r.log)
	reasons := checker.MetRequirements(ctx, &mr)
	if len(reasons) > 0 {
		metricLabels.SetTrue(releaseUpdater.DisruptionApprovalRequired)

		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
		}

		return &TimeResult{
			ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
				Message: fmt.Sprintf("release blocked, disruption approval required: %s", strings.Join(msgs, ", ")),
			},
		}
	}

	deployTimeResult = timeChecker.CalculateMinorDeployTime(mr, metricLabels)

	notifyErr := releaseNotifier.SendMinorReleaseNotification(ctx, mr, deployTimeResult.ReleaseApplyAfterTime, metricLabels)
	if notifyErr != nil {
		r.log.Warn("send minor release notification", log.Err(notifyErr))

		return &TimeResult{
			ProcessedDeployTimeResult: &releaseUpdater.ProcessedDeployTimeResult{
				Message:               msgReleaseIsBlockedByNotification,
				ReleaseApplyAfterTime: deployTimeResult.ReleaseApplyAfterTime,
			},
		}
	}

	processedDTR := timeChecker.ProcessMinorReleaseDeployTime(mr, deployTimeResult)
	if processedDTR == nil {
		return nil
	}

	return &TimeResult{
		ProcessedDeployTimeResult: processedDTR,
		Notified:                  true,
	}
}

func (r *reconciler) updateReleaseStatus(ctx context.Context, mr *v1alpha1.ModuleRelease, status *v1alpha1.ModuleReleaseStatus) error {
	r.log.Debug("refresh release status", slog.String("release", mr.GetName()))

	backoff := &wait.Backoff{
		Steps: 6,
		// magic number
		Duration: 20 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}

	switch status.Phase {
	case v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended, v1alpha1.ModuleReleasePhaseSkipped, v1alpha1.ModuleReleasePhaseTerminating:
		r.metricsUpdater.PurgeReleaseMetric(mr.GetName())
	}

	return ctrlutils.UpdateStatusWithRetry(ctx, r.client, mr, func() error {
		if mr.GetPhase() != status.Phase {
			mr.Status.TransitionTime = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
		}

		mr.Status.Phase = status.Phase
		mr.Status.Message = status.Message

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(backoff))
}

func (r *reconciler) updateModuleLastReleaseDeployedStatus(ctx context.Context, mr *v1alpha1.ModuleRelease, msg, reason string, conditionState bool) error {
	logger := r.log.With(slog.String("module", mr.GetModuleName()))

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mr.GetModuleName()}, module); err != nil {
		return fmt.Errorf("get module: %w", err)
	}

	logger.Debug("refresh module status")

	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, module, func() error {
		condMessage := msg

		// if not successful - see for details in the module release
		if !conditionState {
			condMessage = fmt.Sprintf("%s: see details in the module release v%s", msg, mr.GetVersion().String())
		}

		if conditionState {
			module.SetConditionTrue(v1alpha1.ModuleConditionLastReleaseDeployed, v1alpha1.WithTimer(r.dependencyContainer.GetClock().Now))
		} else {
			module.SetConditionFalse(v1alpha1.ModuleConditionLastReleaseDeployed, reason, condMessage, v1alpha1.WithTimer(r.dependencyContainer.GetClock().Now))
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	return nil
}

// deleteRelease deletes the module from filesystem
func (r *reconciler) deleteRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	if release.GetPhase() != v1alpha1.ModuleReleasePhaseTerminating {
		release.Status.Phase = v1alpha1.ModuleReleasePhaseTerminating
		if err := r.client.Status().Update(ctx, release); err != nil {
			r.log.Warn("failed to set terminating to the release", slog.String("release", release.GetName()), log.Err(err))

			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	modulePath := path.Join(r.downloadedModulesDir, release.GetModuleName(), "v"+release.GetVersion().String())

	if err := os.RemoveAll(modulePath); err != nil {
		r.log.Error("failed to remove module in downloaded dir", slog.String("release", release.GetName()), slog.String("path", modulePath), log.Err(err))
		return ctrl.Result{}, err
	}

	if release.GetPhase() == v1alpha1.ModuleReleasePhaseDeployed {
		r.exts.DeleteConstraints(release.GetModuleName())

		symlinkPath := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", release.GetWeight(), release.GetModuleName()))
		if err := os.RemoveAll(symlinkPath); err != nil {
			r.log.Error("failed to remove module in downloaded symlinks dir", slog.String("release", release.GetName()), slog.String("path", modulePath), log.Err(err))
			return ctrl.Result{}, err
		}
		// TODO(yalosev): we have to disable module here somehow.
		// otherwise, hooks from file system will fail

		// restart controller for completely remove module
		// TODO: we need another solution for remove module from modulemanager
		r.emitRestart("a module release was removed")
	}

	if controllerutil.ContainsFinalizer(release, v1alpha1.ModuleReleaseFinalizerExistOnFs) {
		controllerutil.RemoveFinalizer(release, v1alpha1.ModuleReleaseFinalizerExistOnFs)
		if err := r.client.Update(ctx, release); err != nil {
			r.log.Error("failed to update module release", slog.String("release", release.GetName()), log.Err(err))
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deleteOutdatedModuleReleases finds and deletes all outdated releases of the module in
// Suspend, Skipped or Superseded phases, except for <outdatedReleasesKeepCount> most recent ones
func (r *reconciler) deleteOutdatedModuleReleases(ctx context.Context, moduleSource, module string) error {
	releases := new(v1alpha1.ModuleReleaseList)
	labelSelector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: moduleSource, v1alpha1.ModuleReleaseLabelModule: module}
	if err := r.client.List(ctx, releases, labelSelector); err != nil {
		r.log.Error("failed to list all module releases", log.Err(err))

		return fmt.Errorf("list releases: %w", err)
	}

	type outdatedRelease struct {
		name    string
		version *semver.Version
	}

	outdatedReleases := make(map[string][]outdatedRelease)

	// get all outdated releases by module names
	for _, release := range releases.Items {
		if release.GetPhase() == v1alpha1.ModuleReleasePhaseSuperseded ||
			release.GetPhase() == v1alpha1.ModuleReleasePhaseSuspended ||
			release.GetPhase() == v1alpha1.ModuleReleasePhaseSkipped {
			outdatedReleases[release.Spec.ModuleName] = append(outdatedReleases[release.Spec.ModuleName], outdatedRelease{
				name:    release.GetName(),
				version: release.GetVersion(),
			})
		}
	}

	// sort and delete all outdated releases except for <outdatedReleasesKeepCount> last releases per a module
	for moduleName, outdated := range outdatedReleases {
		r.log.Debug("found the following outdated releases formodule", slog.String("module_name", moduleName), slog.Any("releases_list", releases))

		sort.Slice(outdated, func(i, j int) bool { return outdated[j].version.LessThan(outdated[i].version) })

		if len(outdated) > outdatedReleasesKeepCount {
			for idx := outdatedReleasesKeepCount; idx < len(outdated); idx++ {
				obj := &v1alpha1.ModuleRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name: outdated[idx].name,
					},
				}
				if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
					r.log.Error("failed to delete outdated release", slog.String("outdated_release", outdated[idx].name), log.Err(err))

					return fmt.Errorf("delete outdated release: %w", err)
				}

				r.log.Info("cleaned up outdated release", slog.String("outdated_release", outdated[idx].name), slog.String("module_name", moduleName))
			}
		}
	}

	return nil
}

func (r *reconciler) updateReleaseStatusMessage(ctx context.Context, release *v1alpha1.ModuleRelease, message string) error {
	if release.GetMessage() == message {
		return nil
	}

	release.Status.Message = message

	if err := r.client.Status().Update(ctx, release); err != nil {
		return fmt.Errorf("update the '%s' module release status: %w", release.GetName(), err)
	}

	return nil
}

func newModuleReleaseWithName(name string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func (r *reconciler) isModuleReady(ctx context.Context, moduleName string) bool {
	module := new(v1alpha1.Module)
	err := r.client.Get(ctx, types.NamespacedName{Name: moduleName}, module)
	if err != nil {
		r.log.Warn("cannot find module", slog.String("module-name", moduleName), log.Err(err))

		return false
	}

	return module.Status.Phase == v1alpha1.ModulePhaseReady
}

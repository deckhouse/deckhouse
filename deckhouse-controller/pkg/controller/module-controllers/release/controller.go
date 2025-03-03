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
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	cp "github.com/otiai10/copy"
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
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer,
	ms *metricstorage.MetricStorage,
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
	init                 *sync.WaitGroup
	client               client.Client
	log                  *log.Logger
	dependencyContainer  dependency.Container
	embeddedPolicy       *helpers.ModuleUpdatePolicySpecContainer
	moduleManager        moduleManager
	metricStorage        *metricstorage.MetricStorage
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
	return r.handleRelease(ctx, release)
}

// handleRelease handles releases
func (r *reconciler) handleRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
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
		if err := r.client.Status().Update(ctx, release); err != nil {
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
			if err := r.client.Update(ctx, release); err != nil {
				r.log.Error("failed to update module release status", slog.String("release", release.GetName()), log.Err(err))
				return ctrl.Result{Requeue: true}, nil
			}
		}

		return ctrl.Result{}, nil

	case v1alpha1.ModuleReleasePhaseDeployed:
		return r.handleDeployedRelease(ctx, release)
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
	return r.handlePendingRelease(ctx, release)
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

// handleDeployedRelease handles deployed releases
func (r *reconciler) handleDeployedRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var needsUpdate bool

	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			r.emitRestart(modulesChangedReason)
		}
	}()

	if release.GetReinstall() {
		err := r.runReleaseDeploy(ctx, release, nil)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("run release deploy: %w", err)
		}

		modulesChangedReason = "module release reloaded"

		return ctrl.Result{}, nil
	}

	// check if RegistrySpecChanged annotation is set process it
	if _, set := release.GetAnnotations()[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged]; set {
		// if module is enabled - push runModule task in the main queue
		r.log.Info("apply new registry settings to module", slog.String("module", release.GetModuleName()))

		modulePath := filepath.Join(r.downloadedModulesDir, release.GetModuleName(), fmt.Sprintf("v%s", release.GetVersion()))
		source := release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelSource]

		if err := r.moduleManager.RunModuleWithNewOpenAPISchema(release.GetModuleName(), source, modulePath); err != nil {
			r.log.Error("failed to run module with new openAPI schema", slog.String("module", release.GetModuleName()), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("run module with new open api schema: %w", err)
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

	if len(release.Labels) == 0 || (release.Labels[v1alpha1.ModuleReleaseLabelStatus] != strings.ToLower(v1alpha1.ModuleReleasePhaseDeployed)) {
		if len(release.ObjectMeta.Labels) == 0 {
			release.ObjectMeta.Labels = make(map[string]string)
		}
		release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelStatus] = strings.ToLower(v1alpha1.ModuleReleasePhaseDeployed)
		needsUpdate = true
	}

	if needsUpdate {
		if err := r.client.Update(ctx, release); err != nil {
			r.log.Error("failed to update module release", slog.String("release", release.GetName()), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("update module release: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// at least one release for module source is deployed, add finalizer to prevent module source deletion
	source := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
		r.log.Error("failed to get module source", slog.String("module_source", release.GetModuleSource()), log.Err(err))

		return ctrl.Result{}, fmt.Errorf("get module source: %w", err)
	}

	if !controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists) {
		controllerutil.AddFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists)
		if err := r.client.Update(ctx, source); err != nil {
			r.log.Error("failed to add finalizer to module source", slog.String("module_source", release.GetModuleSource()), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("add finalizer to module source: %w", err)
		}
	}

	// checks if the module release is overridden by modulepulloverride
	exists, err := utils.ModulePullOverrideExists(ctx, r.client, release.GetModuleName())
	if err != nil {
		r.log.Error("failed to get module pull override", slog.String("module", release.GetModuleName()), log.Err(err))

		return ctrl.Result{}, fmt.Errorf("module pull override exists: %w", err)
	}
	if exists {
		r.log.Debug("module is overridden, skip it", slog.String("module", release.GetModuleName()))

		return ctrl.Result{}, nil
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

		return ctrl.Result{}, fmt.Errorf("ensure module documentation: %w", err)
	}

	r.log.Debug("delete outdated releases for module", slog.String("module", release.GetModuleName()))
	err = r.deleteOutdatedModuleReleases(ctx, release.GetModuleSource(), release.GetModuleName())
	if err != nil {
		r.log.Error("failed to delete outdated module releases", slog.String("module", release.GetModuleName()), log.Err(err))

		return ctrl.Result{}, fmt.Errorf("delete outdated module releases: %w", err)
	}

	return ctrl.Result{}, nil
}

// handlePendingRelease handles pending releases
func (r *reconciler) handlePendingRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var res ctrl.Result

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
				r.log.Error("failed to update release status", slog.String("release", release.GetName()), log.Err(err))

				return res, err
			}

			r.log.Error("failed to get update policy", slog.String("policy", policyName), log.Err(err))

			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		// TODO(ipaqsa): remove it
		if policy.Spec.Update.Mode == v1alpha1.ModuleUpdatePolicyModeIgnore {
			if err := r.updateReleaseStatusMessage(ctx, release, disabledByIgnorePolicy); err != nil {
				r.log.Error("failed to update release status", slog.String("release", release.GetName()), log.Err(err))

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
		r.log.Error("failed to parse the notification config", log.Err(err))

		return res, err
	}

	taskCalculator := releaseUpdater.NewModuleReleaseTaskCalculator(r.client, r.log)

	task, err := taskCalculator.CalculatePendingReleaseTask(ctx, release)
	if err != nil {
		return res, err
	}

	if release.GetForce() {
		r.log.Warn("forced release found")

		// deploy forced release without any checks (windows, requirements, approvals and so on)
		err := r.ApplyRelease(ctx, release, task)
		if err != nil {
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
		err := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSkipped,
			Message: task.Message,
		})
		if err != nil {
			r.log.Warn("skip order status update ", slog.String("release", release.GetName()), log.Err(err))
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		return res, nil
	case releaseUpdater.Await:
		err := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: task.Message,
		})
		if err != nil {
			r.log.Warn("await order status update ", slog.String("release", release.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	checker, err := releaseUpdater.NewModuleReleaseRequirementsChecker(r.log)
	if err != nil {
		updateErr := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: err.Error(),
		})
		if updateErr != nil {
			r.log.Warn("create release checker status update ", slog.String("release", release.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	reasons := checker.MetRequirements(release)
	if len(reasons) > 0 {
		msgs := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			msgs = append(msgs, reason.Message)
		}

		err := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhasePending,
			Message: strings.Join(msgs, ";"),
		})
		if err != nil {
			r.log.Warn("met requirements status update ", slog.String("release", release.GetName()), log.Err(err))
		}

		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	us := &releaseUpdater.Settings{
		NotificationConfig: config,
		Mode:               v1alpha1.ParseUpdateMode(policy.Spec.Update.Mode),
		Windows:            policy.Spec.Update.Windows,
		Subject:            releaseUpdater.SubjectModule,
	}

	// handling error inside function
	err = r.PreApplyReleaseCheck(ctx, release, task, us)
	if err != nil {
		// ignore this err, just requeue because of check failed
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	err = r.ApplyRelease(ctx, release, task)
	if err != nil {
		return res, fmt.Errorf("apply predicted release: %w", err)
	}

	modulesChangedReason = "a new module release deployed"

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

		if err = r.updateReleaseStatusMessage(ctx, release, "Update policy not set. Create a suitable ModuleUpdatePolicy object"); err != nil {
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

// runReleaseDeploy
//
// 1) download module
// 2) bump previous deployment status superseded (retry if error)
// 3) bump release annotations (retry if error)
// 3) bump release status to deployed (retry if error)
func (r *reconciler) runReleaseDeploy(ctx context.Context, release *v1alpha1.ModuleRelease, deployedReleaseInfo *releaseUpdater.ReleaseInfo) error {
	r.log.Info("applying release", slog.String("release", release.GetName()))

	downloadStatistic, err := r.loadModule(ctx, release)
	if err != nil {
		return fmt.Errorf("load module: %w", err)
	}

	if deployedReleaseInfo != nil {
		err := r.updateReleaseStatus(ctx, newModuleReleaseWithName(deployedReleaseInfo.Name), &v1alpha1.ModuleReleaseStatus{
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

		release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelStatus] = strings.ToLower(v1alpha1.ModuleReleasePhaseDeployed)

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
		release.Status.Phase = v1alpha1.DeckhouseReleasePhaseDeployed

		release.Status.Size = downloadStatistic.Size
		release.Status.PullDuration = metav1.Duration{Duration: downloadStatistic.PullDuration}

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(backoff))
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	return nil
}

func (r *reconciler) loadModule(ctx context.Context, release *v1alpha1.ModuleRelease) (*downloader.DownloadStatistic, error) {
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
			r.log.Error("failed to remove old module directory", slog.String("directory", tmpDir), log.Err(err))
		}
	}()

	options := utils.GenerateRegistryOptionsFromModuleSource(source, r.clusterUUID, r.log)
	md := downloader.NewModuleDownloader(r.dependencyContainer, tmpDir, source, options)

	downloadStatistic, err := md.DownloadByModuleVersion(release.GetModuleName(), release.GetVersion().String())
	if err != nil {
		return nil, fmt.Errorf("download the '%s/%s' module: %w", release.GetModuleName(), release.GetVersion().String(), err)
	}

	def := &moduletypes.Definition{
		Name:   release.GetModuleName(),
		Weight: release.Spec.Weight,
		Path:   path.Join(tmpDir, release.GetModuleName(), "v"+release.GetVersion().String()),
	}

	values := make(addonutils.Values)
	if module := r.moduleManager.GetModule(release.GetModuleName()); module != nil {
		values = module.GetConfigValues(false)
	}

	err = def.Validate(values, r.log)
	if err != nil {
		err := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSuspended,
			Message: "validation failed: " + err.Error(),
		})
		if err != nil {
			r.log.Error("update status", slog.String("release", release.Name), log.Err(err))

			return nil, fmt.Errorf("update status: the '%s:v%s' module validation: %w", release.GetModuleName(), release.GetVersion().String(), err)
		}

		return nil, fmt.Errorf("the '%s:v%s' module validation: %w", release.GetModuleName(), release.GetVersion().String(), err)
	}

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

var ErrPreApplyCheckIsFailed = errors.New("pre apply check is failed")

// PreApplyReleaseCheck checks final conditions before apply
//
// - Calculating deploy time (if zero - deploy)
func (r *reconciler) PreApplyReleaseCheck(ctx context.Context, mr *v1alpha1.ModuleRelease, task *releaseUpdater.Task, us *releaseUpdater.Settings) error {
	metricLabels := releaseUpdater.NewReleaseMetricLabels(mr)

	timeResult := r.DeployTimeCalculate(ctx, mr, task, us, metricLabels)

	if metricLabels[releaseUpdater.ManualApprovalRequired] == "true" {
		metricLabels[releaseUpdater.ReleaseQueueDepth] = strconv.Itoa(task.QueueDepth)
	}

	// if the predicted release has an index less than the number of awaiting releases
	// calculate and set releaseDepthQueue label
	r.metricsUpdater.UpdateReleaseMetric(mr.GetName(), metricLabels)

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

// DeployTimeCalculate calculate time for release deploy
//
// If patch, calculate by checking this conditions:
// - Canary
// - Notify
// - Window
// - ManualApproved
//
// If minor, calculate by checking this conditions:
// - Cooldown
// - Canary
// - Notify
// - Window
// - Manual Approved
func (r *reconciler) DeployTimeCalculate(ctx context.Context, mr v1alpha1.Release, task *releaseUpdater.Task, us *releaseUpdater.Settings, metricLabels releaseUpdater.MetricLabels) *TimeResult {
	releaseNotifier := releaseUpdater.NewReleaseNotifier(us)
	timeChecker := releaseUpdater.NewDeployTimeService(r.dependencyContainer, us, func(_ context.Context) bool { return true }, r.log)

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
	reasons := checker.MetRequirements(&mr)
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

	processedDTR := timeChecker.ProcessMinorReleaseDeployTime(ctx, mr, deployTimeResult, task.DeployedReleaseInfo)
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

	return ctrlutils.UpdateStatusWithRetry(ctx, r.client, mr, func() error {
		if mr.GetPhase() != status.Phase {
			mr.Status.TransitionTime = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
		}

		mr.Status.Phase = status.Phase
		mr.Status.Message = status.Message

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(backoff))
}

// deleteRelease deletes the module from filesystem
func (r *reconciler) deleteRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	modulePath := path.Join(r.downloadedModulesDir, release.GetModuleName(), "v"+release.GetVersion().String())

	if err := os.RemoveAll(modulePath); err != nil {
		r.log.Error("failed to remove module in downloaded dir", slog.String("release", release.GetName()), slog.String("path", modulePath), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	if release.GetPhase() == v1alpha1.ModuleReleasePhaseDeployed {
		extenders.DeleteConstraints(release.GetModuleName())
		symlinkPath := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", release.GetWeight(), release.GetModuleName()))
		if err := os.RemoveAll(symlinkPath); err != nil {
			r.log.Error("failed to remove module in downloaded symlinks dir", slog.String("release", release.GetName()), slog.String("path", modulePath), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
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
			return ctrl.Result{Requeue: true}, nil
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
	for _, outdated := range releases.Items {
		if outdated.GetPhase() == v1alpha1.ModuleReleasePhaseSuperseded ||
			outdated.GetPhase() == v1alpha1.ModuleReleasePhaseSuspended ||
			outdated.GetPhase() == v1alpha1.ModuleReleasePhaseSkipped {
			outdatedReleases[outdated.Spec.ModuleName] = append(outdatedReleases[outdated.Spec.ModuleName], outdatedRelease{
				name:    outdated.GetName(),
				version: outdated.GetVersion(),
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

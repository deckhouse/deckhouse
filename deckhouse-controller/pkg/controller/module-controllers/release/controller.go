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
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/updater"
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
			"version": release.Spec.Version.String(),
			"module":  release.Spec.ModuleName,
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
				r.log.Infof("restart Deckhouse because %s", r.restartReason)
				if err := syscall.Kill(1, syscall.SIGUSR2); err != nil {
					r.log.Fatalf("send SIGUSR2 signal failed: %s", err)
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

	r.log.Debugf("reconciling the '%s' module release", req.Name)
	release := new(v1alpha1.ModuleRelease)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, release); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warnf("the '%s' module release not found", req.Name)
			return ctrl.Result{}, nil
		}
		r.log.Errorf("failed to get the '%s' module release: %v", req.Name, err)
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
	switch release.Status.Phase {
	case "":
		release.Status.Phase = v1alpha1.ModuleReleasePhasePending
		release.Status.TransitionTime = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
		if err := r.client.Status().Update(ctx, release); err != nil {
			r.log.Errorf("failed to update the '%s' module release status: %v", release.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
		// process to the next phase
		return ctrl.Result{Requeue: true}, nil

	case v1alpha1.ModuleReleasePhaseSuperseded, v1alpha1.ModuleReleasePhaseSuspended, v1alpha1.ModuleReleasePhaseSkipped:
		if len(release.Labels) == 0 || (release.Labels[v1alpha1.ModuleReleaseLabelStatus] != strings.ToLower(release.Status.Phase)) {
			if len(release.Labels) == 0 {
				release.Labels = make(map[string]string)
			}
			release.Labels[v1alpha1.ModuleReleaseLabelStatus] = strings.ToLower(release.Status.Phase)
			if err := r.client.Update(ctx, release); err != nil {
				r.log.Errorf("failed to update the '%s' module release status: %v", release.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
		}

		return ctrl.Result{}, nil

	case v1alpha1.ModuleReleasePhaseDeployed:
		return r.handleDeployedRelease(ctx, release)
	}

	// if module pull override exists, don't process pending release, to avoid fs override
	exists, err := utils.ModulePullOverrideExists(ctx, r.client, release.GetModuleSource(), release.Spec.ModuleName)
	if err != nil {
		r.log.Errorf("failed to get the '%s' module pull override: %v", release.Spec.ModuleName, err)
		return ctrl.Result{Requeue: true}, nil
	}

	if exists {
		r.log.Infof("the %q module is overridden, skip release processing", release.Spec.ModuleName)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// process only pending releases
	return r.handlePendingRelease(ctx, release)
}

// handleRelease handles deployed releases
func (r *reconciler) handleDeployedRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var needsUpdate bool

	// check if RegistrySpecChanged annotation is set process it
	if _, set := release.GetAnnotations()[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged]; set {
		// if module is enabled - push runModule task in the main queue
		r.log.Infof("apply new registry settings to the '%s' module", release.Spec.ModuleName)
		modulePath := filepath.Join(r.downloadedModulesDir, release.Spec.ModuleName, fmt.Sprintf("v%s", release.Spec.Version))
		source := release.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelSource]
		if err := r.moduleManager.RunModuleWithNewOpenAPISchema(release.Spec.ModuleName, source, modulePath); err != nil {
			r.log.Errorf("failed to run the '%s' module with new openAPI schema: %v", release.Spec.ModuleName, err)
			return ctrl.Result{Requeue: true}, nil
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
			r.log.Errorf("failed to update the '%s' module release: %v", release.Name, err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// at least one release for module source is deployed, add finalizer to prevent module source deletion
	source := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
		r.log.Errorf("failed to get the '%s' module source: %v", release.GetModuleSource(), err)
		return ctrl.Result{Requeue: true}, nil
	}

	if !controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists) {
		controllerutil.AddFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists)
		if err := r.client.Update(ctx, source); err != nil {
			r.log.Errorf("failed to add finalizer to the '%s' module source: %v", release.GetModuleSource(), err)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// checks if the module release is overridden by modulepulloverride
	mpo := new(v1alpha1.ModulePullOverride)
	err := r.client.Get(ctx, client.ObjectKey{Name: release.GetModuleName()}, mpo)
	if err == nil {
		// mpo has been found and mpo version must be used as the source of the documentation
		return ctrl.Result{}, nil
	}

	// some other error apart from IsNotFound
	if !apierrors.IsNotFound(err) {
		r.log.Errorf("failed to get the '%s' module pull override: %v", release.GetModuleName(), err)
		return ctrl.Result{Requeue: true}, nil
	}

	modulePath := fmt.Sprintf("/%s/v%s", release.GetModuleName(), release.Spec.Version.String())
	moduleVersion := "v" + release.Spec.Version.String()

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
		r.log.Errorf("failed to ensure the '%s' module documentation: %v", release.GetModuleName(), err)
		return ctrl.Result{Requeue: true}, nil
	}

	r.log.Debugf("delete outdated releases for the '%s' module", release.GetModuleName())
	return r.deleteOutdatedModuleReleases(ctx, release.GetModuleSource(), release.GetModuleName())
}

// handleRelease handles pending releases
func (r *reconciler) handlePendingRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			r.emitRestart(modulesChangedReason)
		}
	}()

	policy := new(v1alpha2.ModuleUpdatePolicy)
	// if release has associated update policy
	if policyName, found := release.GetObjectMeta().GetLabels()[v1alpha1.ModuleReleaseLabelUpdatePolicy]; found {
		if policyName == "" {
			policy = &v1alpha2.ModuleUpdatePolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha2.ModuleUpdatePolicyGVK.Kind,
					APIVersion: v1alpha2.ModuleUpdatePolicyGVK.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
				Spec: *r.embeddedPolicy.Get(),
			}
		} else {
			// get policy spec
			if err := r.client.Get(ctx, client.ObjectKey{Name: policyName}, policy); err != nil {
				r.metricStorage.CounterAdd("{PREFIX}module_update_policy_not_found", 1.0, map[string]string{
					"version":        release.GetReleaseVersion(),
					"module_release": release.GetName(),
					"module":         release.GetModuleName(),
				})

				if uerr := r.updateReleaseStatusMessage(ctx, release, fmt.Sprintf("Update policy %s not found", policyName)); uerr != nil {
					r.log.Errorf("failed to update the '%s' release status: %v", release.Name, uerr)
					return ctrl.Result{Requeue: true}, nil
				}
				r.log.Errorf("failed to get the '%s' update policy: %v", policyName, err)
				return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			}
		}

		// TODO(ipaqsa): remove it
		if policy.Spec.Update.Mode == v1alpha1.ModuleUpdatePolicyModeIgnore {
			if err := r.updateReleaseStatusMessage(ctx, release, disabledByIgnorePolicy); err != nil {
				r.log.Errorf("failed to update the '%s' release status: %v", release.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{RequeueAfter: defaultCheckInterval * 4}, nil
		}
	} else {
		var err error
		policy, err = utils.UpdatePolicy(ctx, r.client, r.embeddedPolicy, release.GetModuleName())
		if err != nil {
			r.log.Errorf("failed to get update policy for the '%s' release: %v", release.Name, err)
			if err = r.updateReleaseStatusMessage(ctx, release, "Update policy not set. Create a suitable ModuleUpdatePolicy object"); err != nil {
				r.log.Errorf("failed to update the '%s' release status: %v", release.Name, err)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
		marshalledPatch, _ := json.Marshal(map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					v1alpha1.ModuleReleaseLabelUpdatePolicy: policy.Name,
				},
			},
			"status": map[string]string{
				"message": "",
			},
		})
		patch := client.RawPatch(types.MergePatchType, marshalledPatch)
		if err = r.client.Patch(ctx, release, patch); err != nil {
			r.log.Errorf("failed to patch the '%s' module release: %v", release.Name, err)
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
		// also patch status field
		if err = r.client.Status().Patch(ctx, release, patch); err != nil {
			r.log.Errorf("failed to patch the '%s' module release status: %v", release.Name, err)
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
	}

	// parse notification config from the deckhouse-discovery secret
	config, err := utils.GetNotificationConfig(ctx, r.client)
	if err != nil {
		r.log.Errorf("failed to parse the notification config: %v", err)
		return ctrl.Result{Requeue: true}, nil
	}

	settings := &updater.Settings{
		NotificationConfig: config,
		Mode:               updater.ParseUpdateMode(policy.Spec.Update.Mode),
		Windows:            policy.Spec.Update.Windows,
	}

	releaseUpdater := updater.NewUpdater[*v1alpha1.ModuleRelease](
		ctx, r.dependencyContainer, r.log, settings, updater.DeckhouseReleaseData{}, true, false,
		newKubeAPI(r.log, r.client, r.downloadedModulesDir, r.symlinksDir, r.clusterUUID, r.moduleManager, r.dependencyContainer),
		&metricsUpdater{metricStorage: r.metricStorage}, &webhookDataSource{logger: r.log}, r.moduleManager.GetEnabledModuleNames(),
	)

	{
		otherReleases := new(v1alpha1.ModuleReleaseList)
		if err = r.client.List(ctx, otherReleases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: release.GetModuleName()}); err != nil {
			r.log.Errorf("failed to list module releases: %v", err)
			return ctrl.Result{Requeue: true}, nil
		}
		pointerReleases := make([]*v1alpha1.ModuleRelease, 0, len(otherReleases.Items))
		for _, rel := range otherReleases.Items {
			pointerReleases = append(pointerReleases, &rel)
		}
		releaseUpdater.SetReleases(pointerReleases)
	}

	if releaseUpdater.ReleasesCount() == 0 {
		return ctrl.Result{}, nil
	}

	releaseUpdater.PredictNextRelease(release)

	if releaseUpdater.LastReleaseDeployed() {
		r.log.Debug("latest release is deployed")
		return ctrl.Result{}, nil
	}

	if predicted := releaseUpdater.GetPredictedRelease(); predicted != nil {
		if predicted.GetName() != release.GetName() {
			// requeue the release
			r.log.Debugf("processing wrong release (current: %s, predicted: %s)", predicted.Name, predicted.Name)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if releaseUpdater.PredictedReleaseIsPatch() {
		// patch release does not respect update windows or ManualMode
		if err = releaseUpdater.ApplyPredictedRelease(); err != nil {
			r.log.Errorf("failed to apply predicted release: %v", err.Error())
			return r.wrapApplyReleaseError(err), nil
		}

		modulesChangedReason = "a new module release found"
		return ctrl.Result{}, nil
	}

	if err = releaseUpdater.ApplyPredictedRelease(); err != nil {
		r.log.Errorf("failed to apply predicted release: %v", err.Error())
		return r.wrapApplyReleaseError(err), nil
	}

	modulesChangedReason = "a new module release found"

	r.log.Debugf("delete outdated releases for the '%s' module", release.GetModuleName())
	return r.deleteOutdatedModuleReleases(ctx, release.GetModuleSource(), release.GetModuleName())
}

// deleteRelease deletes the module from filesystem
func (r *reconciler) deleteRelease(ctx context.Context, release *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	modulePath := path.Join(r.downloadedModulesDir, release.Spec.ModuleName, "v"+release.Spec.Version.String())

	if err := os.RemoveAll(modulePath); err != nil {
		r.log.Errorf("failed to remove the '%s' module in the '%s' downloaded dir: %v", release.Name, modulePath, err)
		return ctrl.Result{Requeue: true}, nil
	}

	if release.Status.Phase == v1alpha1.ModuleReleasePhaseDeployed {
		extenders.DeleteConstraints(release.GetModuleName())
		symlinkPath := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", release.Spec.Weight, release.Spec.ModuleName))
		if err := os.RemoveAll(symlinkPath); err != nil {
			r.log.Errorf("failed to remove the '%s' module in the '%s' symlinks dir: %v", release.Name, modulePath, err)
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
			r.log.Errorf("failed to update the '%s' module release: %v", release.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

// deleteOutdatedModuleReleases finds and deletes all outdated releases of the module in
// Suspend, Skipped or Superseded phases, except for <outdatedReleasesKeepCount> most recent ones
func (r *reconciler) deleteOutdatedModuleReleases(ctx context.Context, moduleSource, module string) (ctrl.Result, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	labelSelector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: moduleSource, v1alpha1.ModuleReleaseLabelModule: module}
	if err := r.client.List(ctx, releases, labelSelector); err != nil {
		r.log.Errorf("failed to list all module releases: %v", err)
		return ctrl.Result{Requeue: true}, nil
	}

	type outdatedRelease struct {
		name    string
		version *semver.Version
	}

	outdatedReleases := make(map[string][]outdatedRelease)

	// get all outdated releases by module names
	for _, outdated := range releases.Items {
		if outdated.Status.Phase == v1alpha1.ModuleReleasePhaseSuperseded || outdated.Status.Phase == v1alpha1.ModuleReleasePhaseSuspended || outdated.Status.Phase == v1alpha1.ModuleReleasePhaseSkipped {
			outdatedReleases[outdated.Spec.ModuleName] = append(outdatedReleases[outdated.Spec.ModuleName], outdatedRelease{
				name:    outdated.Name,
				version: outdated.Spec.Version,
			})
		}
	}

	// sort and delete all outdated releases except for <outdatedReleasesKeepCount> last releases per a module
	for moduleName, outdated := range outdatedReleases {
		r.log.Debugf("found the following outdated releases for the '%s' module: %v", moduleName, releases)

		sort.Slice(outdated, func(i, j int) bool { return outdated[j].version.LessThan(outdated[i].version) })

		if len(outdated) > outdatedReleasesKeepCount {
			for idx := outdatedReleasesKeepCount; idx < len(outdated); idx++ {
				obj := &v1alpha1.ModuleRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name: outdated[idx].name,
					},
				}
				if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
					r.log.Errorf("failed to delete the '%s' outdated release: %v", outdated[idx].name, err)
					return ctrl.Result{Requeue: true}, nil
				}
				r.log.Infof("cleaned up the %q outdated release of the %q module", outdated[idx].name, moduleName)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) wrapApplyReleaseError(err error) ctrl.Result {
	var notReadyErr *updater.NotReadyForDeployError
	if errors.As(err, &notReadyErr) {
		// TODO: requeue all releases if deckhouse update settings is changed
		// requeueAfter := notReadyErr.RetryDelay()
		// if requeueAfter == 0 {
		// requeueAfter = defaultCheckInterval
		// }
		// r.logger.Infof("%s: retry after %s", err.Error(), requeueAfter)
		// return ctrl.Result{RequeueAfter: requeueAfter}, nil
		return ctrl.Result{RequeueAfter: defaultCheckInterval}
	}

	return ctrl.Result{}
}

func (r *reconciler) updateReleaseStatusMessage(ctx context.Context, release *v1alpha1.ModuleRelease, message string) error {
	if release.Status.Message == message {
		return nil
	}

	release.Status.Message = message

	if err := r.client.Status().Update(ctx, release); err != nil {
		return fmt.Errorf("update the '%s' module release status: %w", release.Name, err)
	}

	return nil
}

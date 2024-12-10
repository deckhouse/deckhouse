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
	result := ctrl.Result{}
	releases := new(v1alpha1.ModuleReleaseList)
	labelSelector := client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: moduleSource, v1alpha1.ModuleReleaseLabelModule: module}
	if err := r.client.List(ctx, releases, labelSelector); err != nil {
		r.log.Errorf("failed to list all module releases: %v", err)
		return ctrl.Result{Requeue: true}, nil
	}

	// at least one release for module source is deployed, add finalizer to prevent module source deletion
	ms := new(v1alpha1.ModuleSource)
	err := r.client.Get(ctx, types.NamespacedName{Name: mr.GetModuleSource()}, ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if !controllerutil.ContainsFinalizer(ms, sourceReleaseFinalizer) {
		controllerutil.AddFinalizer(ms, sourceReleaseFinalizer)
		err = r.client.Update(ctx, ms)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	// checks if the modulerelease is overridden by modulepulloverride
	mpo := new(v1alpha1.ModulePullOverride)
	err = r.client.Get(ctx, types.NamespacedName{Name: mr.GetModuleName()}, mpo)
	// mpo has been found and mpo version must be used as the source of the documentation
	if err == nil {
		return result, nil
	}

	// some other error apart from IsNotFound
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{Requeue: true}, err
	}

	// mpo not found - update the docs from the module release version
	modulePath := fmt.Sprintf("/%s/v%s", mr.GetModuleName(), mr.Spec.Version.String())
	moduleVersion := "v" + mr.Spec.Version.String()
	checksum := mr.Labels["release-checksum"]
	if checksum == "" {
		checksum = fmt.Sprintf("%x", md5.Sum([]byte(moduleVersion)))
	}
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
		Kind:       v1alpha1.ModuleReleaseGVK.Kind,
		Name:       mr.GetName(),
		UID:        mr.GetUID(),
		Controller: ptr.To(true),
	}

	err = createOrUpdateModuleDocumentationCR(ctx, r.client, mr.GetModuleName(), moduleVersion, checksum, modulePath, mr.GetModuleSource(), ownerRef)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return r.cleanUpModuleReleases(ctx, mr)
}

func (r *reconciler) reconcilePendingRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var result ctrl.Result
	moduleName := mr.Spec.ModuleName

	r.log.Debugf("checking requirements of '%s' for module '%s' by extenders", mr.GetName(), mr.GetModuleName())
	if err := extenders.CheckModuleReleaseRequirements(mr.GetName(), mr.Spec.Requirements); err != nil {
		if err = r.updateModuleReleaseStatusMessage(ctx, mr, err.Error()); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// search symlink for module by regexp
	// module weight for a new version of the module may be different from the old one,
	// we need to find a symlink that contains the module name without looking at the weight prefix.
	currentModuleSymlink, err := findExistingModuleSymlink(r.symlinksDir, moduleName)
	if err != nil {
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			r.emitRestart(modulesChangedReason)
		}
	}()

	nConfig, err := r.parseNotificationConfig(ctx)
	if err != nil {
		return result, fmt.Errorf("parse notification config: %w", err)
	}

	policy := new(v1alpha1.ModuleUpdatePolicy)
	// if release has associated update policy
	if policyName, found := mr.GetObjectMeta().GetLabels()[UpdatePolicyLabel]; found {
		if policyName == "" {
			policy = &v1alpha1.ModuleUpdatePolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ModuleUpdatePolicyGVK.Kind,
					APIVersion: v1alpha1.ModuleUpdatePolicyGVK.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
				Spec: *r.deckhouseEmbeddedPolicy.Get(),
			}
		} else {
			// get policy spec
			err = r.client.Get(ctx, types.NamespacedName{Name: policyName}, policy)
			if err != nil {
				r.metricStorage.CounterAdd("{PREFIX}module_update_policy_not_found", 1.0, map[string]string{
					"version":        mr.GetReleaseVersion(),
					"module_release": mr.GetName(),
					"module":         mr.GetModuleName(),
				})

				if err := r.updateModuleReleaseStatusMessage(ctx, mr, fmt.Sprintf("Update policy %s not found", policyName)); err != nil {
					return result, fmt.Errorf("update module release status message: %w", err)
				}
				return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			}
		}

		if policy.Spec.Update.Mode == "Ignore" {
			if err := r.updateModuleReleaseStatusMessage(ctx, mr, disabledByIgnorePolicy); err != nil {
				return result, fmt.Errorf("update module release status message: %w", err)
			}
			return ctrl.Result{RequeueAfter: defaultCheckInterval * 4}, nil
		}
	} else {
		// get all policies regardless of their labels
		policies := new(v1alpha1.ModuleUpdatePolicyList)
		err = r.client.List(ctx, policies)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		policy, err = r.getReleasePolicy(mr.GetModuleSource(), mr.GetModuleName(), policies.Items)
		if err != nil {
			if err := r.updateModuleReleaseStatusMessage(ctx, mr, "Update policy not set. Create a suitable ModuleUpdatePolicy object"); err != nil {
				return result, fmt.Errorf("update module release status message: %w", err)
			}
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
		patch, _ := json.Marshal(map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					UpdatePolicyLabel: policy.Name,
				},
			},
			"status": map[string]string{
				"message": "",
			},
		})
		p := client.RawPatch(types.MergePatchType, patch)

		err = r.client.Patch(ctx, mr, p)
		if err != nil {
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
		// also patch status field
		err = r.client.Status().Patch(ctx, mr, p)
		if err != nil {
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
	}

	k8 := newKubeAPI(ctx, r.log, r.client, r.downloadedModulesDir, r.symlinksDir, r.moduleManager, r.dependencyContainer, r.clusterUUID)
	settings := &updater.Settings{
		NotificationConfig: nConfig,
		Mode:               updater.ParseUpdateMode(policy.Spec.Update.Mode),
		Windows:            policy.Spec.Update.Windows,
	}
	releaseUpdater := newModuleUpdater(r.dependencyContainer, r.log, settings, k8, r.moduleManager.GetEnabledModuleNames(), r.metricStorage)
	{
		otherReleases := new(v1alpha1.ModuleReleaseList)
		err = r.client.List(ctx, otherReleases, client.MatchingLabels{"module": moduleName})
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		pointerReleases := make([]*v1alpha1.ModuleRelease, 0, len(otherReleases.Items))
		for _, r := range otherReleases.Items {
			pointerReleases = append(pointerReleases, &r)
		}
		releaseUpdater.SetReleases(pointerReleases)
	}

	if releaseUpdater.ReleasesCount() == 0 {
		return result, nil
	}

	releaseUpdater.PredictNextRelease()

	if releaseUpdater.LastReleaseDeployed() {
		// latest release deployed
		deployedRelease := *releaseUpdater.DeployedRelease()
		deckhouseconfig.Service().AddModuleNameToSource(deployedRelease.Spec.ModuleName, deployedRelease.GetModuleSource())

		// check symlink exists on FS, relative symlink
		modulePath := generateModulePath(moduleName, deployedRelease.Spec.Version.String())
		if !isModuleExistsOnFS(r.symlinksDir, currentModuleSymlink, modulePath) {
			newModuleSymlink := path.Join(r.symlinksDir, fmt.Sprintf("%d-%s", deployedRelease.Spec.Weight, moduleName))
			r.log.Warnf("Module %q doesn't exist on the filesystem. Restoring", moduleName)
			err = enableModule(r.downloadedModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
			if err != nil {
				r.log.Errorf("Module restore for module %q and release %q failed: %v", moduleName, deployedRelease.Spec.Version.String(), err)

				return ctrl.Result{Requeue: true}, err
			}
			// defer restart
			modulesChangedReason = "one of modules is not enabled"
		}

		return result, nil
	}

	if releaseUpdater.GetPredictedReleaseIndex() == -1 {
		return result, nil
	}

	err = releaseUpdater.ApplyPredictedRelease()
	if err != nil {
		return r.wrapApplyReleaseError(err), nil
	}

	modulesChangedReason = "a new module release found"
	return r.cleanUpModuleReleases(ctx, mr)
}

// getReleasePolicy checks if any update policy matches the module release and if it's so - returns the policy and its release channel.
// if several policies match the module release labels, conflict=true is returned
func (r *reconciler) getReleasePolicy(sourceName, moduleName string, policies []v1alpha1.ModuleUpdatePolicy) (*v1alpha1.ModuleUpdatePolicy, error) {
	var releaseLabelsSet labels.Set = map[string]string{"module": moduleName, "source": sourceName}
	var matchedPolicy v1alpha1.ModuleUpdatePolicy
	var found bool

	for _, policy := range policies {
		if policy.Spec.ModuleReleaseSelector.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.Spec.ModuleReleaseSelector.LabelSelector)
			if err != nil {
				return nil, err
			}
			selectorSourceName, sourceLabelExists := selector.RequiresExactMatch("source")
			if sourceLabelExists && selectorSourceName != sourceName {
				// 'source' label is set, but does not match the given ModuleSource
				continue
			}

			if selector.Matches(releaseLabelsSet) {
				// ModuleUpdatePolicy matches ModuleSource and specified Module
				if found {
					return nil, fmt.Errorf("more than one update policy matches the module: %s and %s", matchedPolicy.Name, policy.Name)
				}
				found = true
				matchedPolicy = policy
			}
		}
	}

	if !found {
		r.log.Infof("ModuleUpdatePolicy for ModuleSource: %q, Module: %q not found, using Embedded policy: %+v", sourceName, moduleName, *r.deckhouseEmbeddedPolicy.Get())
		return &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleUpdatePolicyGVK.Kind,
				APIVersion: v1alpha1.ModuleUpdatePolicyGVK.GroupVersion().String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "", // special empty default policy, inherits Deckhouse settings for update mode
			},
			Spec: *r.deckhouseEmbeddedPolicy.Get(),
		}, nil
	}

	return &matchedPolicy, nil
}

func (r *reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	// Get the ModuleRelease resource with this name
	mr := new(v1alpha1.ModuleRelease)
	err := r.client.Get(ctx, types.NamespacedName{Name: request.Name}, mr)
	if err != nil {
		// The ModuleRelease resource may no longer exist, in which case we stop
		// processing.
		return result, client.IgnoreNotFound(err)
	}

	if !mr.DeletionTimestamp.IsZero() {
		return r.deleteReconcile(ctx, mr)
	}

	return r.createOrUpdateReconcile(ctx, mr)
}

func enableModule(downloadedModulesDir, oldSymlinkPath, newSymlinkPath, modulePath string) error {
	if oldSymlinkPath != "" {
		if _, err := os.Lstat(oldSymlinkPath); err == nil {
			err = os.Remove(oldSymlinkPath)
			if err != nil {
				return errors.Wrapf(err, "delete old symlink %s", oldSymlinkPath)
			}
		}
	}

	if _, err := os.Lstat(newSymlinkPath); err == nil {
		err = os.Remove(newSymlinkPath)
		if err != nil {
			return errors.Wrapf(err, "delete new symlink %s", newSymlinkPath)
		}
	}

	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(downloadedModulesDir, strings.TrimPrefix(modulePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return errors.Wrapf(err, "module absolute path %s not found", moduleAbsPath)
	}

	return os.Symlink(modulePath, newSymlinkPath)
}

func findExistingModuleSymlink(rootPath, moduleName string) (string, error) {
	var symlinkPath string

	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)
	walkDir := func(path string, d os.DirEntry, _ error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}

		symlinkPath = path
		return filepath.SkipDir
	}

	err := filepath.WalkDir(rootPath, walkDir)

	return symlinkPath, err
}

func generateModulePath(moduleName, version string) string {
	return path.Join("../", moduleName, "v"+version)
}

func isModuleExistsOnFS(symlinksDir, symlinkPath, modulePath string) bool {
	targetPath, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return false
	}

	if filepath.IsAbs(targetPath) {
		targetPath, err = filepath.Rel(symlinksDir, targetPath)
		if err != nil {
			return false
		}
	}

	return targetPath == modulePath
}

func addLabels(mr *v1alpha1.ModuleRelease, labels map[string]string) {
	lb := mr.GetLabels()
	if len(lb) == 0 {
		mr.SetLabels(labels)
	} else {
		for l, v := range labels {
			lb[l] = v
		}
	}
}

// updateModuleReleaseStatusMessage updates module release's `.status.message field
func (r *reconciler) updateModuleReleaseStatusMessage(ctx context.Context, mr *v1alpha1.ModuleRelease, message string) error {
	if mr.Status.Message == message {
		return nil
	}

	mr.Status.Message = message

	err := r.client.Status().Update(ctx, mr)
	if err != nil {
		return err
	}

	return nil
}

// PreflightCheck start a few checks and synchronize deckhouse filesystem with ModuleReleases
//   - Download modules, which have status=deployed on ModuleRelease but have no files on Filesystem
//   - Delete modules, that don't have ModuleRelease presented in the cluster
func (r *reconciler) PreflightCheck(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			r.preflightCountDown.Done()
		}
	}()
	if r.downloadedModulesDir == "" {
		return nil
	}

	r.clusterUUID = r.getClusterUUID(ctx)

	// Check if controller's dependencies have been initialized
	_ = wait.PollUntilContextCancel(ctx, utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			// TODO: add modulemanager initialization check r.moduleManager.AreModulesInited() (required for reloading modules without restarting deckhouse)
			return deckhouseconfig.IsServiceInited(), nil
		})

	go r.restartLoop(ctx)
	err = r.restoreAbsentModulesFromReleases(ctx)
	if err != nil {
		return fmt.Errorf("modules restoration from releases failed: %w", err)
	}

	err = r.deleteModulesWithAbsentRelease(ctx)
	if err != nil {
		return fmt.Errorf("absent modules cleanup failed: %w", err)
	}

	return r.registerMetrics(ctx)
}

func (r *reconciler) getClusterUUID(ctx context.Context) string {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	err := r.client.Get(ctx, key, &secret)
	if err != nil {
		r.log.Warnf("Read clusterUUID from secret %s failed: %v. Generating random uuid", key, err)
		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	return uuid.Must(uuid.NewV4()).String()
}

func (r *reconciler) deleteModulesWithAbsentRelease(ctx context.Context) error {
	symlinksDir := filepath.Join(r.downloadedModulesDir, "modules")

	fsModulesLinks, err := r.readModulesFromFS(symlinksDir)
	if err != nil {
		return fmt.Errorf("read source modules from the filesystem failed: %w", err)
	}

	var releasesList v1alpha1.ModuleReleaseList
	err = r.client.List(ctx, &releasesList)
	if err != nil {
		return fmt.Errorf("fetch ModuleReleases failed: %w", err)
	}
	releases := releasesList.Items

	r.log.Debugf("%d ModuleReleases found", len(releases))

	for _, release := range releases {
		delete(fsModulesLinks, release.Spec.ModuleName)
	}

	for module, moduleLinkPath := range fsModulesLinks {
		var mpo v1alpha1.ModulePullOverride
		err = r.client.Get(ctx, types.NamespacedName{Name: module}, &mpo)
		if err != nil && apierrors.IsNotFound(err) {
			r.log.Warnf("Module %q has neither ModuleRelease nor ModuleOverride. Purging from FS", module)
			_ = os.RemoveAll(moduleLinkPath)
		}
	}

	return nil
}

func (r *reconciler) readModulesFromFS(dir string) (map[string]string, error) {
	moduleLinks, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	modules := make(map[string]string, len(moduleLinks))

	for _, moduleLink := range moduleLinks {
		index := strings.Index(moduleLink.Name(), "-")
		if index == -1 {
			continue
		}

		moduleName := moduleLink.Name()[index+1:]
		modules[moduleName] = path.Join(dir, moduleLink.Name())
	}

	return modules, nil
}

// restoreAbsentModulesFromReleases checks ModuleReleases with Deployed status and restore them on the FS
func (r *reconciler) restoreAbsentModulesFromReleases(ctx context.Context) error {
	var releaseList v1alpha1.ModuleReleaseList
	err := r.client.List(ctx, &releaseList)
	if err != nil {
		return err
	}

	// TODO: add labels to list only Deployed releases
	for _, item := range releaseList.Items {
		if item.Status.Phase != "Deployed" {
			continue
		}

		// ignore deleted Releases
		if !item.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		moduleWeight := item.Spec.Weight
		moduleVersion := "v" + item.Spec.Version.String()
		moduleName := item.Spec.ModuleName
		moduleSource := item.GetModuleSource()

		// if ModulePullOverride is set, don't check and restore overridden release
		exists, err := r.isModulePullOverrideExists(ctx, moduleSource, moduleName)
		if err != nil {
			r.log.Errorf("Couldn't check module pull override for module %s: %s", moduleName, err)
		}

		if exists {
			r.log.Infof("ModulePullOverride for module %q exists. Skipping release restore", moduleName)
			continue
		}

		// get relevant module source
		ms := new(v1alpha1.ModuleSource)
		err = r.client.Get(ctx, types.NamespacedName{Name: moduleSource}, ms)
		if err != nil {
			return fmt.Errorf("ModuleSource %v for ModuleRelease/%s/%s got an error: %w", moduleSource, moduleName, moduleVersion, err)
		}

		moduleSymLink := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, item.Spec.ModuleName))
		_, err = os.Stat(moduleSymLink)
		if err != nil {
			// module symlink not found
			r.log.Infof("Module %q symlink is absent on file system. Restoring it", moduleName)
			if os.IsNotExist(err) {
				err := r.createModuleSymlink(moduleName, moduleVersion, ms, moduleWeight)
				if err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
				// some other error
			} else {
				return fmt.Errorf("module %s check error: %s", moduleName, err)
			}
			// check if module symlink leads to current version
		} else {
			dstDir, err := filepath.EvalSymlinks(moduleSymLink)
			if err != nil {
				return fmt.Errorf("couldn't evaluate module %s symlink %s: %s", moduleName, moduleSymLink, err)
			}

			// module symlink leads to some other version.
			// also, if dstDir doesn't exist, its Base evaluates to .
			if filepath.Base(dstDir) != moduleVersion {
				r.log.Infof("Module %q symlink is incorrect. Restoring it", moduleName)
				if err := r.createModuleSymlink(moduleName, moduleVersion, ms, moduleWeight); err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
			}
		}

		// sync registry spec
		if err := syncModuleRegistrySpec(r.downloadedModulesDir, moduleName, moduleVersion, ms); err != nil {
			return fmt.Errorf("couldn't sync the %s module's registry settings with the %s module source: %w", moduleName, ms.Name, err)
		}
		r.log.Infof("Resynced the %s module's registry settings with the %s module source", moduleName, ms.Name)
	}
	return nil
}

type moduleOpenAPISpec struct {
	Properties struct {
		Registry struct {
			Properties struct {
				Base struct {
					Default string `yaml:"default"`
				} `yaml:"base"`
				DockerCFG struct {
					Default string `yaml:"default"`
				} `yaml:"dockercfg"`
				Scheme struct {
					Default string `yaml:"default"`
				} `yaml:"scheme"`
				CA struct {
					Default string `yaml:"default"`
				} `yaml:"ca"`
			} `yaml:"properties"`
		} `yaml:"registry,omitempty"`
	} `yaml:"properties,omitempty"`
}

// syncModulesRegistrySpec compares and updates current registry settings of a deployed module (in the ./openapi/values.yaml file)
// and the registry settings set in the related module source
func syncModuleRegistrySpec(downloadedModulesDir, moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource) error {
	var openAPISpec moduleOpenAPISpec

	openAPIFile, err := os.Open(filepath.Join(downloadedModulesDir, moduleName, moduleVersion, "openapi/values.yaml"))
	if err != nil {
		return fmt.Errorf("couldn't open the %s module's openapi values: %w", moduleName, err)
	}
	defer openAPIFile.Close()

	b, err := io.ReadAll(openAPIFile)
	if err != nil {
		return fmt.Errorf("couldn't read from the %s module's openapi values: %w", moduleName, err)
	}

	err = yaml.Unmarshal(b, &openAPISpec)
	if err != nil {
		return fmt.Errorf("couldn't unmarshal the %s module's registry spec: %w", moduleName, err)
	}

	registrySpec := openAPISpec.Properties.Registry.Properties

	dockercfg := downloader.DockerCFGForModules(moduleSource.Spec.Registry.Repo, moduleSource.Spec.Registry.DockerCFG)

	if moduleSource.Spec.Registry.CA != registrySpec.CA.Default || dockercfg != registrySpec.DockerCFG.Default || moduleSource.Spec.Registry.Repo != registrySpec.Base.Default || moduleSource.Spec.Registry.Scheme != registrySpec.Scheme.Default {
		err = downloader.InjectRegistryToModuleValues(filepath.Join(downloadedModulesDir, moduleName, moduleVersion), moduleSource)
	}

	return err
}

// wipeModuleSymlinks checks if there are symlinks for the module with different weight in the symlink folder
func wipeModuleSymlinks(symlinksDir, moduleName string) error {
	// delete all module's symlinks in a loop
	for {
		anotherModuleSymlink, err := findExistingModuleSymlink(symlinksDir, moduleName)
		if err != nil {
			return fmt.Errorf("couldn't check if there are any other symlinks for module %v: %w", moduleName, err)
		}

		if len(anotherModuleSymlink) > 0 {
			if err := os.Remove(anotherModuleSymlink); err != nil {
				return fmt.Errorf("couldn't delete stale symlink %v for module %v: %w", anotherModuleSymlink, moduleName, err)
			}
			// go for another spin
			continue
		}

		// no more symlinks found
		break
	}
	return nil
}

// createModuleSymlink checks if there are any other symlinks for a module in the symlink dir and deletes them before
// attempting to download current version of the module and creating correct symlink
func (r *reconciler) createModuleSymlink(moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource, moduleWeight uint32) error {
	r.log.Infof("Module %q is absent on file system. Restoring it from source %q", moduleName, moduleSource.Name)

	// removing possible symlink doubles
	err := wipeModuleSymlinks(r.symlinksDir, moduleName)
	if err != nil {
		return err
	}

	// check if module's directory exists on fs
	info, err := os.Stat(path.Join(r.downloadedModulesDir, moduleName, moduleVersion))
	if err != nil || !info.IsDir() {
		r.log.Infof("Downloading module %q from registry", moduleName)
		// download the module to fs
		options := utils.GenerateRegistryOptionsFromModuleSource(moduleSource, r.clusterUUID, r.logger)
		md := downloader.NewModuleDownloader(r.dc, r.downloadedModulesDir, moduleSource, options)
		_, err = md.DownloadByModuleVersion(moduleName, moduleVersion)
		if err != nil {
			return fmt.Errorf("download module %v with version %v failed: %w. Skipping", moduleName, moduleVersion, err)
		}
	}

	// restore symlink
	moduleRelativePath := filepath.Join("../", moduleName, moduleVersion)
	symlinkPath := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
	err = restoreModuleSymlink(r.downloadedModulesDir, symlinkPath, moduleRelativePath)
	if err != nil {
		return fmt.Errorf("creating symlink for module %v failed: %w", moduleName, err)
	}
	r.log.Infof("Module %s:%s restored to %s", moduleName, moduleVersion, moduleRelativePath)

	return nil
}

func (r *reconciler) parseNotificationConfig(ctx context.Context) (updater.NotificationConfig, error) {
	var secret corev1.Secret
	err := r.client.Get(ctx, types.NamespacedName{Name: "deckhouse-discovery", Namespace: "d8-system"}, &secret)
	if err != nil {
		return updater.NotificationConfig{}, fmt.Errorf("get secret: %w", err)
	}

	// TODO: remove this dependency
	jsonSettings, ok := secret.Data["updateSettings.json"]
	if !ok {
		return updater.NotificationConfig{}, nil
	}

	var settings struct {
		NotificationConfig updater.NotificationConfig `json:"notification"`
	}

	err = json.Unmarshal(jsonSettings, &settings)
	if err != nil {
		return updater.NotificationConfig{}, fmt.Errorf("unmarshal json: %w", err)
	}

	return settings.NotificationConfig, nil
}

func validateModule(def models.DeckhouseModuleDefinition, values addonutils.Values, logger *log.Logger) error {
	if def.Weight < 900 || def.Weight > 999 {
		return fmt.Errorf("external module weight must be between 900 and 999")
	}
	if def.Path == "" {
		return fmt.Errorf("cannot validate module without path. Path is required to load openapi specs")
	}

	cb, vb, err := addonutils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
	if err != nil {
		return fmt.Errorf("read open API files: %w", err)
	}
	dm, err := addonmodules.NewBasicModule(def.Name, def.Path, def.Weight, nil, cb, vb, logger.Named("basic-module"))
	if err != nil {
		return fmt.Errorf("new deckhouse module: %w", err)
	}

	if values != nil {
		dm.SaveConfigValues(values)
	}

	err = dm.Validate()
	// Next we will need to record all validation errors except required (602).
	var result, mErr *multierror.Error
	if errors.As(err, &mErr) {
		for _, me := range mErr.Errors {
			var e *openapierrors.Validation
			if errors.As(me, &e) {
				if e.Code() == 602 {
					continue
				}
			}
			result = multierror.Append(result, me)
		}
	}
	// Now result will contain all validation errors, if any, except required.

	if result != nil {
		return fmt.Errorf("validate module: %w", result)
	}

	return nil
}

func restoreModuleSymlink(downloadedModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(downloadedModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}

func (r *reconciler) updateModuleReleaseDownloadStatistic(ctx context.Context, release *v1alpha1.ModuleRelease,
	ds *downloader.DownloadStatistic,
) (*v1alpha1.ModuleRelease, error) {
	release.Status.Size = ds.Size
	release.Status.PullDuration = metav1.Duration{Duration: ds.PullDuration}

	return release, r.client.Status().Update(ctx, release)
}

func (r *reconciler) registerMetrics(ctx context.Context) error {
	var releasesList v1alpha1.ModuleReleaseList
	err := r.client.List(ctx, &releasesList)
	if err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for _, release := range releasesList.Items {
		l := map[string]string{
			"version": release.Spec.Version.String(),
			"module":  release.Spec.ModuleName,
		}

		r.metricStorage.GaugeSet("{PREFIX}module_pull_seconds_total", release.Status.PullDuration.Seconds(), l)
		r.metricStorage.GaugeSet("{PREFIX}module_size_bytes_total", float64(release.Status.Size), l)
	}

	return nil
}

func createOrUpdateModuleDocumentationCR(
	ctx context.Context,
	client client.Client,
	moduleName, moduleVersion, moduleChecksum, modulePath, moduleSource string,
	ownerRef metav1.OwnerReference,
) error {
	var md v1alpha1.ModuleDocumentation
	err := client.Get(ctx, types.NamespacedName{Name: moduleName}, &md)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// just create
			md = v1alpha1.ModuleDocumentation{
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ModuleDocumentationGVK.Kind,
					APIVersion: v1alpha1.ModuleDocumentationGVK.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
					Labels: map[string]string{
						"module": moduleName,
						"source": moduleSource,
					},
					OwnerReferences: []metav1.OwnerReference{ownerRef},
				},
				Spec: v1alpha1.ModuleDocumentationSpec{
					Version:  moduleVersion,
					Path:     modulePath,
					Checksum: moduleChecksum,
				},
			}

			err = client.Create(ctx, &md)
			if err != nil {
				return err
			}
		}

		return err
	}

	if md.Spec.Version != moduleVersion || md.Spec.Checksum != moduleChecksum {
		// update CR
		md.Spec.Path = modulePath
		md.Spec.Version = moduleVersion
		md.Spec.Checksum = moduleChecksum
		md.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

		err = client.Update(ctx, &md)
		if err != nil {
			return err
		}
	}

	return nil
}

// cleanUpModuleReleases finds and deletes all outdated releases of the module in Suspend, Skipped or Superseded phases, except for <outdatedReleasesKeepCount> most recent ones
func (r *reconciler) cleanUpModuleReleases(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var result ctrl.Result
	// get related releases
	var moduleReleasesFromSource v1alpha1.ModuleReleaseList
	err := r.client.List(ctx, &moduleReleasesFromSource, client.MatchingLabels{"source": mr.GetModuleSource(), "module": mr.GetModuleName()})
	if err != nil {
		return result, fmt.Errorf("couldn't list module releases to clean up: %w", err)
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
		// r.log.Infof("%s: retry after %s", err.Error(), requeueAfter)
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

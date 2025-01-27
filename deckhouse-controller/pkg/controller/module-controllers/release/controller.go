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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/flant/shell-operator/pkg/metric_storage"
	openapierrors "github.com/go-openapi/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	deckhouseconfig "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	d8env "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

// moduleReleaseReconciler is the controller implementation for ModuleRelease resources
type moduleReleaseReconciler struct {
	client client.Client

	dc            dependency.Container
	metricStorage *metric_storage.MetricStorage
	logger        logger.Logger

	moduleManager        moduleManager
	downloadedModulesDir string
	symlinksDir          string

	deckhouseEmbeddedPolicy *helpers.ModuleUpdatePolicySpecContainer

	preflightCountDown *sync.WaitGroup

	m             sync.Mutex
	delayTimer    *time.Timer
	restartReason string
	clusterUUID   string
}

const (
	RegistrySpecChangedAnnotation = "modules.deckhouse.io/registry-spec-changed"
	UpdatePolicyLabel             = "modules.deckhouse.io/update-policy"
	deckhouseNodeNameAnnotation   = "modules.deckhouse.io/deployed-on"

	defaultCheckInterval   = 15 * time.Second
	fsReleaseFinalizer     = "modules.deckhouse.io/exist-on-fs"
	sourceReleaseFinalizer = "modules.deckhouse.io/release-exists"
	disabledByIgnorePolicy = `Update disabled by 'Ignore' update policy`

	outdatedReleasesKeepCount = 3
)

func NewModuleReleaseController(
	mgr manager.Manager,
	dc dependency.Container,
	embeddedPolicyContainer *helpers.ModuleUpdatePolicySpecContainer,
	mm moduleManager,
	metricStorage *metric_storage.MetricStorage,
	preflightCountDown *sync.WaitGroup,
) error {
	lg := log.WithField("component", "ModuleReleaseController")

	c := &moduleReleaseReconciler{
		client:               mgr.GetClient(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dc:                   dc,
		logger:               lg,

		metricStorage:           metricStorage,
		moduleManager:           mm,
		symlinksDir:             filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		deckhouseEmbeddedPolicy: embeddedPolicyContainer,

		delayTimer: time.NewTimer(3 * time.Second),

		preflightCountDown: preflightCountDown,
	}

	// Add Preflight Check
	err := mgr.Add(manager.RunnableFunc(c.PreflightCheck))
	if err != nil {
		return err
	}
	c.preflightCountDown.Add(1)

	ctr, err := controller.New("module-release", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		CacheSyncTimeout:        3 * time.Minute,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              c,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModuleRelease{}).
		// for reconcile documentation if accidentally removed
		Owns(&v1alpha1.ModuleDocumentation{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(ctr)
}

func (r *moduleReleaseReconciler) emitRestart(msg string) {
	r.m.Lock()
	r.delayTimer.Reset(3 * time.Second)
	r.restartReason = msg
	r.m.Unlock()
}

func (r *moduleReleaseReconciler) restartLoop(ctx context.Context) {
	for {
		r.m.Lock()
		select {
		case <-r.delayTimer.C:
			if r.restartReason != "" {
				r.logger.Infof("Restarting Deckhouse because %s", r.restartReason)

				err := syscall.Kill(1, syscall.SIGUSR2)
				if err != nil {
					r.logger.Fatalf("Send SIGUSR2 signal failed: %s", err)
				}
			}
			r.delayTimer.Reset(3 * time.Second)

		case <-ctx.Done():
			return
		}

		r.m.Unlock()
	}
}

// only ModuleRelease with active finalizer can get here, we have to remove the module on filesystem and remove the finalizer
func (r *moduleReleaseReconciler) deleteReconcile(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var result ctrl.Result

	// deleted release
	// also cleanup the filesystem
	modulePath := path.Join(r.downloadedModulesDir, mr.Spec.ModuleName, "v"+mr.Spec.Version.String())

	err := os.RemoveAll(modulePath)
	if err != nil {
		return result, fmt.Errorf("remove all in %s: %w", modulePath, err)
	}

	if mr.Status.Phase == v1alpha1.PhaseDeployed {
		extenders.DeleteConstraints(mr.GetModuleName())
		symlinkPath := filepath.Join(r.downloadedModulesDir, "modules", fmt.Sprintf("%d-%s", mr.Spec.Weight, mr.Spec.ModuleName))
		err := os.RemoveAll(symlinkPath)
		if err != nil {
			return result, err
		}
		// TODO(yalosev): we have to disable module here somehow.
		// otherwise, hooks from file system will fail

		// restart controller for completely remove module
		// TODO: we need another solution for remove module from modulemanager
		r.emitRestart("a module release was removed")
	}

	if !controllerutil.ContainsFinalizer(mr, fsReleaseFinalizer) {
		return result, nil
	}

	controllerutil.RemoveFinalizer(mr, fsReleaseFinalizer)
	err = r.client.Update(ctx, mr)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *moduleReleaseReconciler) createOrUpdateReconcile(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var result ctrl.Result

	switch mr.Status.Phase {
	case "":
		mr.Status.Phase = v1alpha1.PhasePending
		mr.Status.TransitionTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		if err := r.client.Status().Update(ctx, mr); err != nil {
			return result, fmt.Errorf("update status: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil // process to the next phase

	case v1alpha1.PhaseSuperseded, v1alpha1.PhaseSuspended, v1alpha1.PhaseSkipped:
		if mr.Labels["status"] != strings.ToLower(mr.Status.Phase) {
			// update labels
			addLabels(mr, map[string]string{"status": strings.ToLower(mr.Status.Phase)})
			if err := r.client.Update(ctx, mr); err != nil {
				return result, fmt.Errorf("update status: %w", err)
			}
		}

		return result, nil

	case v1alpha1.PhaseDeployed:
		return r.reconcileDeployedRelease(ctx, mr)
	}

	// if ModulePullOverride is set, don't process pending release, to avoid fs override
	exists, err := r.isModulePullOverrideExists(ctx, mr.GetModuleSource(), mr.Spec.ModuleName)
	if err != nil {
		return result, err
	}

	if exists {
		r.logger.Infof("ModulePullOverride for module %q exists. Skipping release processing", mr.Spec.ModuleName)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// process only pending releases
	return r.reconcilePendingRelease(ctx, mr)
}

func (r *moduleReleaseReconciler) isModulePullOverrideExists(ctx context.Context, sourceName, moduleName string) (bool, error) {
	var res v1alpha1.ModulePullOverrideList
	err := r.client.List(ctx, &res, client.MatchingLabels{"source": sourceName, "module": moduleName}, client.Limit(1))
	if err != nil {
		return false, err
	}

	return len(res.Items) > 0, nil
}

func (r *moduleReleaseReconciler) reconcileDeployedRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var result ctrl.Result
	var metaUpdateRequired bool

	// check if RegistrySpecChangedAnnotation annotation is set and processes it
	if _, set := mr.GetAnnotations()[RegistrySpecChangedAnnotation]; set {
		// if module is enabled - push runModule task in the main queue
		r.logger.Infof("Applying new registry settings to the %s module", mr.Spec.ModuleName)
		err := r.moduleManager.RunModuleWithNewOpenAPISchema(mr.Spec.ModuleName, mr.ObjectMeta.Labels["source"], filepath.Join(r.downloadedModulesDir, mr.Spec.ModuleName, fmt.Sprintf("v%s", mr.Spec.Version)))
		if err != nil {
			return result, fmt.Errorf("run module with new OpenAPI schema: %w", err)
		}
		// delete annotation and requeue
		delete(mr.ObjectMeta.Annotations, RegistrySpecChangedAnnotation)
		metaUpdateRequired = true
	}

	// add finalizer and status label
	if !controllerutil.ContainsFinalizer(mr, fsReleaseFinalizer) {
		controllerutil.AddFinalizer(mr, fsReleaseFinalizer)
		metaUpdateRequired = true
	}

	if mr.Labels["status"] != strings.ToLower(v1alpha1.PhaseDeployed) {
		addLabels(mr, map[string]string{"status": strings.ToLower(v1alpha1.PhaseDeployed)})
		metaUpdateRequired = true
	}

	if metaUpdateRequired {
		err := r.client.Update(ctx, mr)
		if err != nil {
			return result, fmt.Errorf("update release: %w", err)
		}
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

func (r *moduleReleaseReconciler) reconcilePendingRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var result ctrl.Result
	moduleName := mr.Spec.ModuleName

	r.logger.Debugf("checking requirements of '%s' for module '%s' by extenders", mr.GetName(), mr.GetModuleName())
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

	k8 := newKubeAPI(ctx, r.logger, r.client, r.downloadedModulesDir, r.symlinksDir, r.moduleManager, r.dc, r.clusterUUID)
	settings := &updater.Settings{
		NotificationConfig: nConfig,
		Mode:               updater.ParseUpdateMode(policy.Spec.Update.Mode),
		Windows:            policy.Spec.Update.Windows,
	}
	releaseUpdater := newModuleUpdater(r.dc, r.logger, settings, k8, r.moduleManager.GetEnabledModuleNames(), r.metricStorage)
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
			r.logger.Warnf("Module %q doesn't exist on the filesystem. Restoring", moduleName)
			err = enableModule(r.downloadedModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
			if err != nil {
				r.logger.Errorf("Module restore for module %q and release %q failed: %v", moduleName, deployedRelease.Spec.Version.String(), err)

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
		return r.wrapApplyReleaseError(err)
	}

	modulesChangedReason = "a new module release found"
	return r.cleanUpModuleReleases(ctx, mr)
}

func (r *moduleReleaseReconciler) wrapApplyReleaseError(err error) (ctrl.Result, error) {
	var result ctrl.Result
	var notReadyErr *updater.NotReadyForDeployError

	if errors.As(err, &notReadyErr) {
		r.logger.Infoln(err.Error())
		// TODO: requeue all releases if deckhouse update settings is changed
		// requeueAfter := notReadyErr.RetryDelay()
		// if requeueAfter == 0 {
		// requeueAfter = defaultCheckInterval
		// }
		// r.logger.Infof("%s: retry after %s", err.Error(), requeueAfter)
		// return ctrl.Result{RequeueAfter: requeueAfter}, nil
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	return result, fmt.Errorf("apply predicted release: %w", err)
}

// getReleasePolicy checks if any update policy matches the module release and if it's so - returns the policy and its release channel.
// if several policies match the module release labels, conflict=true is returned
func (r *moduleReleaseReconciler) getReleasePolicy(sourceName, moduleName string, policies []v1alpha1.ModuleUpdatePolicy) (*v1alpha1.ModuleUpdatePolicy, error) {
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
		r.logger.Infof("ModuleUpdatePolicy for ModuleSource: %q, Module: %q not found, using Embedded policy: %+v", sourceName, moduleName, *r.deckhouseEmbeddedPolicy.Get())
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

func (r *moduleReleaseReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
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
func (r *moduleReleaseReconciler) updateModuleReleaseStatusMessage(ctx context.Context, mr *v1alpha1.ModuleRelease, message string) error {
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
func (r *moduleReleaseReconciler) PreflightCheck(ctx context.Context) (err error) {
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

func (r *moduleReleaseReconciler) getClusterUUID(ctx context.Context) string {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	err := r.client.Get(ctx, key, &secret)
	if err != nil {
		r.logger.Warnf("Read clusterUUID from secret %s failed: %v. Generating random uuid", key, err)
		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	return uuid.Must(uuid.NewV4()).String()
}

func (r *moduleReleaseReconciler) deleteModulesWithAbsentRelease(ctx context.Context) error {
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

	r.logger.Debugf("%d ModuleReleases found", len(releases))

	for _, release := range releases {
		delete(fsModulesLinks, release.Spec.ModuleName)
	}

	for module, moduleLinkPath := range fsModulesLinks {
		var mpo v1alpha1.ModulePullOverride
		err = r.client.Get(ctx, types.NamespacedName{Name: module}, &mpo)
		if err != nil && apierrors.IsNotFound(err) {
			r.logger.Warnf("Module %q has neither ModuleRelease nor ModuleOverride. Purging from FS", module)
			_ = os.RemoveAll(moduleLinkPath)
		}
	}

	return nil
}

func (r *moduleReleaseReconciler) readModulesFromFS(dir string) (map[string]string, error) {
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
func (r *moduleReleaseReconciler) restoreAbsentModulesFromReleases(ctx context.Context) error {
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
			r.logger.Errorf("Couldn't check module pull override for module %s: %s", moduleName, err)
		}

		if exists {
			r.logger.Infof("ModulePullOverride for module %q exists. Skipping release restore", moduleName)
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
			r.logger.Infof("Module %q symlink is absent on file system. Restoring it", moduleName)
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
				r.logger.Infof("Module %q symlink is incorrect. Restoring it", moduleName)
				if err := r.createModuleSymlink(moduleName, moduleVersion, ms, moduleWeight); err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
			}
		}

		// sync registry spec
		if err := syncModuleRegistrySpec(r.downloadedModulesDir, moduleName, moduleVersion, ms); err != nil {
			return fmt.Errorf("couldn't sync the %s module's registry settings with the %s module source: %w", moduleName, ms.Name, err)
		}
		r.logger.Infof("Resynced the %s module's registry settings with the %s module source", moduleName, ms.Name)
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
func (r *moduleReleaseReconciler) createModuleSymlink(moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource, moduleWeight uint32) error {
	r.logger.Infof("Module %q is absent on file system. Restoring it from source %q", moduleName, moduleSource.Name)

	// removing possible symlink doubles
	err := wipeModuleSymlinks(r.symlinksDir, moduleName)
	if err != nil {
		return err
	}

	// check if module's directory exists on fs
	info, err := os.Stat(path.Join(r.downloadedModulesDir, moduleName, moduleVersion))
	if err != nil || !info.IsDir() {
		r.logger.Infof("Downloading module %q from registry", moduleName)
		// download the module to fs
		options := utils.GenerateRegistryOptionsFromModuleSource(moduleSource, r.clusterUUID)
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
	r.logger.Infof("Module %s:%s restored to %s", moduleName, moduleVersion, moduleRelativePath)

	return nil
}

func (r *moduleReleaseReconciler) parseNotificationConfig(ctx context.Context) (updater.NotificationConfig, error) {
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

func validateModule(def models.DeckhouseModuleDefinition, values addonutils.Values) error {
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
	dm, err := addonmodules.NewBasicModule(def.Name, def.Path, def.Weight, nil, cb, vb)
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

type moduleManager interface {
	DisableModuleHooks(moduleName string)
	GetModule(moduleName string) *addonmodules.BasicModule
	RunModuleWithNewOpenAPISchema(moduleName, moduleSource, modulePath string) error
	GetEnabledModuleNames() []string
	IsModuleEnabled(moduleName string) bool
}

func (r *moduleReleaseReconciler) updateModuleReleaseDownloadStatistic(ctx context.Context, release *v1alpha1.ModuleRelease,
	ds *downloader.DownloadStatistic,
) (*v1alpha1.ModuleRelease, error) {
	release.Status.Size = ds.Size
	release.Status.PullDuration = metav1.Duration{Duration: ds.PullDuration}

	return release, r.client.Status().Update(ctx, release)
}

func (r *moduleReleaseReconciler) registerMetrics(ctx context.Context) error {
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
func (r *moduleReleaseReconciler) cleanUpModuleReleases(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
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

	outdatedReleases := make(map[string][]outdatedRelease, 0)

	// get all outdated releases by module names
	for _, rl := range moduleReleasesFromSource.Items {
		if rl.Status.Phase == v1alpha1.PhaseSuperseded || rl.Status.Phase == v1alpha1.PhaseSuspended || rl.Status.Phase == v1alpha1.PhaseSkipped {
			outdatedReleases[rl.Spec.ModuleName] = append(outdatedReleases[rl.Spec.ModuleName], outdatedRelease{
				name:    rl.Name,
				version: rl.Spec.Version,
			})
		}
	}

	// sort and delete all outdated releases except for <outdatedReleasesKeepCount> last releases per a module
	for moduleName, releases := range outdatedReleases {
		sort.Slice(releases, func(i, j int) bool { return releases[j].version.LessThan(releases[i].version) })
		r.logger.Debugf("Found the following outdated releases for %s module: %v", moduleName, releases)
		if len(releases) > outdatedReleasesKeepCount {
			for i := outdatedReleasesKeepCount; i < len(releases); i++ {
				releaseObj := &v1alpha1.ModuleRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name: releases[i].name,
					},
				}
				err = r.client.Delete(ctx, releaseObj)
				if err != nil && !apierrors.IsNotFound(err) {
					return result, fmt.Errorf("couldn't clean up outdated release %q of %s module: %w", releases[i].name, moduleName, err)
				}
				r.logger.Infof("cleaned up outdated release %q of %q module", releases[i].name, moduleName)
			}
		}
	}

	return result, nil
}

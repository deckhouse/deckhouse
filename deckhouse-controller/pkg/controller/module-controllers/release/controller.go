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
	"strings"
	"sync"
	"syscall"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
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
	deckhouseconfig "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

// moduleReleaseReconciler is the controller implementation for ModuleRelease resources
type moduleReleaseReconciler struct {
	client client.Client

	dc            dependency.Container
	metricStorage *metric_storage.MetricStorage
	logger        logger.Logger

	moduleManager      moduleManager
	externalModulesDir string
	symlinksDir        string

	deckhouseEmbeddedPolicy *v1alpha1.ModuleUpdatePolicySpec

	preflightCountDown *sync.WaitGroup

	m             sync.Mutex
	delayTimer    *time.Timer
	restartReason string
}

const (
	RegistrySpecChangedAnnotation = "modules.deckhouse.io/registry-spec-changed"
	UpdatePolicyLabel             = "modules.deckhouse.io/update-policy"
	deckhouseNodeNameAnnotation   = "modules.deckhouse.io/deployed-on"

	defaultCheckInterval   = 15 * time.Second
	fsReleaseFinalizer     = "modules.deckhouse.io/exist-on-fs"
	sourceReleaseFinalizer = "modules.deckhouse.io/release-exists"
	disabledByIgnorePolicy = `Update disabled by 'Ignore' update policy`
)

func NewModuleReleaseController(
	mgr manager.Manager,
	dc dependency.Container,
	embeddedPolicy *v1alpha1.ModuleUpdatePolicySpec,
	mm moduleManager,
	metricStorage *metric_storage.MetricStorage,
	preflightCountDown *sync.WaitGroup,
) error {
	lg := log.WithField("component", "ModuleReleaseController")

	c := &moduleReleaseReconciler{
		client:             mgr.GetClient(),
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		dc:                 dc,
		logger:             lg,

		metricStorage:           metricStorage,
		moduleManager:           mm,
		symlinksDir:             filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
		deckhouseEmbeddedPolicy: embeddedPolicy,

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
		CacheSyncTimeout:        15 * time.Minute,
		NeedLeaderElection:      pointer.Bool(false),
		Reconciler:              c,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModuleRelease{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(ctr)
}

func (c *moduleReleaseReconciler) emitRestart(msg string) {
	c.m.Lock()
	c.delayTimer.Reset(3 * time.Second)
	c.restartReason = msg
	c.m.Unlock()
}

func (c *moduleReleaseReconciler) restartLoop(ctx context.Context) {
	for {
		c.m.Lock()
		select {
		case <-c.delayTimer.C:
			if c.restartReason != "" {
				c.logger.Infof("Restarting Deckhouse because %s", c.restartReason)

				err := syscall.Kill(1, syscall.SIGUSR2)
				if err != nil {
					c.logger.Fatalf("Send SIGUSR2 signal failed: %s", err)
				}
			}
			c.delayTimer.Reset(3 * time.Second)

		case <-ctx.Done():
			return
		}

		c.m.Unlock()
	}
}

// only ModuleRelease with active finalizer can get here, we have to remove the module on filesystem and remove the finalizer
func (c *moduleReleaseReconciler) deleteReconcile(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	// deleted release
	// also cleanup the filesystem
	modulePath := path.Join(c.externalModulesDir, mr.Spec.ModuleName, "v"+mr.Spec.Version.String())

	err := os.RemoveAll(modulePath)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if mr.Status.Phase == v1alpha1.PhaseDeployed {
		symlinkPath := filepath.Join(c.externalModulesDir, "modules", fmt.Sprintf("%d-%s", mr.Spec.Weight, mr.Spec.ModuleName))
		err := os.RemoveAll(symlinkPath)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		// TODO(yalosev): we have to disable module here somehow.
		// otherwise, hooks from file system will fail
	}

	if !controllerutil.ContainsFinalizer(mr, fsReleaseFinalizer) {
		return ctrl.Result{}, nil
	}

	controllerutil.RemoveFinalizer(mr, fsReleaseFinalizer)
	err = c.client.Update(ctx, mr)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (c *moduleReleaseReconciler) createOrUpdateReconcile(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	switch mr.Status.Phase {
	case "":
		mr.Status.Phase = v1alpha1.PhasePending
		mr.Status.TransitionTime = metav1.NewTime(c.dc.GetClock().Now().UTC())
		if e := c.client.Status().Update(ctx, mr); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{Requeue: true}, nil // process to the next phase

	case v1alpha1.PhaseSuperseded, v1alpha1.PhaseSuspended:
		if mr.Labels["status"] != strings.ToLower(mr.Status.Phase) {
			// update labels
			addLabels(mr, map[string]string{"status": strings.ToLower(mr.Status.Phase)})
			if err := c.client.Update(ctx, mr); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}

		return ctrl.Result{}, nil

	case v1alpha1.PhaseDeployed:
		return c.reconcileDeployedRelease(ctx, mr)
	}

	// if ModulePullOverride is set, don't process pending release, to avoid fs override
	exists, err := c.isModulePullOverrideExists(ctx, mr.GetModuleSource(), mr.Spec.ModuleName)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if exists {
		c.logger.Infof("ModulePullOverride for module %q exists. Skipping release processing", mr.Spec.ModuleName)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// process only pending releases
	return c.reconcilePendingRelease(ctx, mr)
}

func (c *moduleReleaseReconciler) isModulePullOverrideExists(ctx context.Context, sourceName, moduleName string) (bool, error) {
	var res v1alpha1.ModulePullOverrideList
	err := c.client.List(ctx, &res, client.MatchingLabels{"source": sourceName, "module": moduleName}, client.Limit(1))
	if err != nil {
		return false, err
	}

	return len(res.Items) > 0, nil
}

func (c *moduleReleaseReconciler) reconcileDeployedRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	var metaUpdateRequired bool
	// check if RegistrySpecChangedAnnotation annotation is set and processes it
	if _, set := mr.GetAnnotations()[RegistrySpecChangedAnnotation]; set {
		// if module is enabled - push runModule task in the main queue
		c.logger.Infof("Applying new registry settings to the %s module", mr.Spec.ModuleName)
		err := c.moduleManager.RunModuleWithNewStaticValues(mr.Spec.ModuleName, mr.ObjectMeta.Labels["source"], filepath.Join(c.externalModulesDir, mr.Spec.ModuleName, fmt.Sprintf("v%s", mr.Spec.Version)))
		if err != nil {
			return ctrl.Result{Requeue: true}, err
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
		return ctrl.Result{Requeue: true}, c.client.Update(ctx, mr)
	}

	// at least one release for module source is deployed, add finalizer to prevent module source deletion
	ms := new(v1alpha1.ModuleSource)
	err := c.client.Get(ctx, types.NamespacedName{Name: mr.GetModuleSource()}, ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if !controllerutil.ContainsFinalizer(ms, sourceReleaseFinalizer) {
		controllerutil.AddFinalizer(ms, sourceReleaseFinalizer)
		err = c.client.Update(ctx, ms)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	// checks if the modulerelease is overridden by modulepulloverride
	mpo := new(v1alpha1.ModulePullOverride)
	err = c.client.Get(ctx, types.NamespacedName{Name: mr.GetModuleName()}, mpo)
	// mpo has been found and mpo version must be used as the source of the documentation
	if err == nil {
		return ctrl.Result{}, nil
	}

	// some other error apart from IsNotFound
	if err != nil && !apierrors.IsNotFound(err) {
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
		Controller: pointer.Bool(true),
	}

	err = createOrUpdateModuleDocumentationCR(ctx, c.client, mr.GetModuleName(), moduleVersion, checksum, modulePath, mr.GetModuleSource(), ownerRef)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (c *moduleReleaseReconciler) reconcilePendingRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	moduleName := mr.Spec.ModuleName

	otherReleases := new(v1alpha1.ModuleReleaseList)
	err := c.client.List(ctx, otherReleases, client.MatchingLabels{"module": moduleName})
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// search symlink for module by regexp
	// module weight for a new version of the module may be different from the old one,
	// we need to find a symlink that contains the module name without looking at the weight prefix.
	currentModuleSymlink, err := findExistingModuleSymlink(c.symlinksDir, moduleName)
	if err != nil {
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			c.emitRestart(modulesChangedReason)
		}
	}()

	nConfig, err := c.parseNotificationConfig(ctx)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("parse notification config: %w", err)
	}

	policy := new(v1alpha1.ModuleUpdatePolicy)
	// if release has associated update policy
	if policyName, found := mr.ObjectMeta.Labels[UpdatePolicyLabel]; found {
		if policyName == "" {
			policy = &v1alpha1.ModuleUpdatePolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ModuleUpdatePolicyGVK.Kind,
					APIVersion: v1alpha1.ModuleUpdatePolicyGVK.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
				Spec: *c.deckhouseEmbeddedPolicy,
			}
		} else {
			// get policy spec
			err = c.client.Get(ctx, types.NamespacedName{Name: policyName}, policy)
			if err != nil {
				if e := c.updateModuleReleaseStatusMessage(ctx, mr, fmt.Sprintf("Update policy %s not found", policyName)); e != nil {
					return ctrl.Result{Requeue: true}, e
				}
				return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			}
		}

		if policy.Spec.Update.Mode == "Ignore" {
			if e := c.updateModuleReleaseStatusMessage(ctx, mr, disabledByIgnorePolicy); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
	} else {
		if e := c.updateModuleReleaseStatusMessage(ctx, mr, fmt.Sprintf("Update policy not set. Create a ModuleUpdatePolicy object and label the release '%s=<policy_name>'", UpdatePolicyLabel)); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	kubeAPI := newKubeAPI(ctx, c.logger, c.client, c.externalModulesDir, c.symlinksDir, c.moduleManager, c.dc)
	releaseUpdater := newModuleUpdater(c.logger, nConfig, policy.Spec.Update.Mode, kubeAPI, c.moduleManager.GetEnabledModuleNames())

	pointerReleases := make([]*v1alpha1.ModuleRelease, 0, len(otherReleases.Items))
	for _, r := range otherReleases.Items {
		pointerReleases = append(pointerReleases, &r)
	}
	releaseUpdater.PrepareReleases(pointerReleases)
	if releaseUpdater.ReleasesCount() == 0 {
		return ctrl.Result{}, nil
	}

	releaseUpdater.PredictNextRelease()

	if releaseUpdater.LastReleaseDeployed() {
		// latest release deployed
		deployedRelease := otherReleases.Items[releaseUpdater.GetCurrentDeployedReleaseIndex()]
		deckhouseconfig.Service().AddModuleNameToSource(deployedRelease.Spec.ModuleName, deployedRelease.GetModuleSource())

		// check symlink exists on FS, relative symlink
		modulePath := generateModulePath(moduleName, deployedRelease.Spec.Version.String())
		if !isModuleExistsOnFS(c.symlinksDir, currentModuleSymlink, modulePath) {
			newModuleSymlink := path.Join(c.symlinksDir, fmt.Sprintf("%d-%s", deployedRelease.Spec.Weight, moduleName))
			c.logger.Debugf("Module %q is not exists on the filesystem. Restoring", moduleName)
			err = enableModule(c.externalModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
			if err != nil {
				c.logger.Errorf("Module restore failed: %v", err)
				if e := c.suspendModuleVersionForRelease(ctx, &deployedRelease, err); e != nil {
					return ctrl.Result{Requeue: true}, e
				}

				return ctrl.Result{Requeue: true}, err
			}
			// defer restart
			modulesChangedReason = "one of modules is not enabled"
		}

		for _, index := range releaseUpdater.GetSkippedPatchesIndexes() {
			release := otherReleases.Items[index]

			release.Status.Phase = v1alpha1.PhaseSuperseded
			release.Status.Message = ""
			release.Status.TransitionTime = metav1.NewTime(c.dc.GetClock().Now().UTC())
			if e := c.client.Status().Update(ctx, &release); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
		}

		return ctrl.Result{}, nil
	}

	if releaseUpdater.GetPredictedReleaseIndex() == -1 {
		return ctrl.Result{}, nil
	}

	if releaseUpdater.PredictedReleaseIsPatch() {
		// patch release does not respect update windows or ManualMode
		if !releaseUpdater.ApplyPredictedRelease(nil) {
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		modulesChangedReason = "a new module release found"
		return ctrl.Result{}, nil
	}

	var windows update.Windows
	if !releaseUpdater.InManualMode() {
		windows = policy.Spec.Update.Windows
	}

	if !releaseUpdater.ApplyPredictedRelease(windows) {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	modulesChangedReason = "a new module release found"
	return ctrl.Result{}, nil
}

func (c *moduleReleaseReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// Get the ModuleRelease resource with this name
	mr := new(v1alpha1.ModuleRelease)
	err := c.client.Get(ctx, types.NamespacedName{Name: request.Name}, mr)
	if err != nil {
		// The ModuleRelease resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if !mr.DeletionTimestamp.IsZero() {
		return c.deleteReconcile(ctx, mr)
	}

	return c.createOrUpdateReconcile(ctx, mr)
}

func (c *moduleReleaseReconciler) suspendModuleVersionForRelease(ctx context.Context, release *v1alpha1.ModuleRelease, err error) error {
	if os.IsNotExist(err) {
		err = errors.New("not found")
	}

	release.Status.Phase = v1alpha1.PhaseSuspended
	release.Status.Message = fmt.Sprintf("Desired version of the module met problems: %s", err)
	release.Status.TransitionTime = metav1.NewTime(c.dc.GetClock().Now().UTC())

	return c.client.Status().Update(ctx, release)
}

func enableModule(externalModulesDir, oldSymlinkPath, newSymlinkPath, modulePath string) error {
	if oldSymlinkPath != "" {
		if _, err := os.Lstat(oldSymlinkPath); err == nil {
			err = os.Remove(oldSymlinkPath)
			if err != nil {
				return err
			}
		}
	}

	if _, err := os.Lstat(newSymlinkPath); err == nil {
		err = os.Remove(newSymlinkPath)
		if err != nil {
			return err
		}
	}

	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(modulePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
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
func (c *moduleReleaseReconciler) updateModuleReleaseStatusMessage(ctx context.Context, mr *v1alpha1.ModuleRelease, message string) error {
	if mr.Status.Message == message {
		return nil
	}

	mr.Status.Message = message

	err := c.client.Status().Update(ctx, mr)
	if err != nil {
		return err
	}

	return nil
}

// PreflightCheck start a few checks and synchronize deckhouse filesystem with ModuleReleases
//   - Download modules, which have status=deployed on ModuleRelease but have no files on Filesystem
//   - Delete modules, that don't have ModuleRelease presented in the cluster
func (c *moduleReleaseReconciler) PreflightCheck(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			c.preflightCountDown.Done()
		}
	}()
	if c.externalModulesDir == "" {
		return nil
	}

	// Check if controller's dependencies have been initialized
	_ = wait.PollUntilContextCancel(ctx, utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			// TODO: add modulemanager initialization check c.moduleManager.AreModulesInited() (required for reloading modules without restarting deckhouse)
			return deckhouseconfig.IsServiceInited(), nil
		})

	go c.restartLoop(ctx)
	err = c.restoreAbsentModulesFromReleases(ctx)
	if err != nil {
		return fmt.Errorf("modules restoration from releases failed: %w", err)
	}

	err = c.deleteModulesWithAbsentRelease(ctx)
	if err != nil {
		return fmt.Errorf("absent modules cleanup failed: %w", err)
	}

	return c.registerMetrics(ctx)
}

func (c *moduleReleaseReconciler) deleteModulesWithAbsentRelease(ctx context.Context) error {
	symlinksDir := filepath.Join(c.externalModulesDir, "modules")

	fsModulesLinks, err := c.readModulesFromFS(symlinksDir)
	if err != nil {
		return fmt.Errorf("read source modules from the filesystem failed: %w", err)
	}

	var releasesList v1alpha1.ModuleReleaseList
	err = c.client.List(ctx, &releasesList)
	if err != nil {
		return fmt.Errorf("fetch ModuleReleases failed: %w", err)
	}
	releases := releasesList.Items

	c.logger.Debugf("%d ModuleReleases found", len(releases))

	for _, release := range releases {
		delete(fsModulesLinks, release.Spec.ModuleName)
	}

	for module, moduleLinkPath := range fsModulesLinks {
		var mpo v1alpha1.ModulePullOverride
		err = c.client.Get(ctx, types.NamespacedName{Name: module}, &mpo)
		if err != nil && apierrors.IsNotFound(err) {
			c.logger.Warnf("Module %q has neither ModuleRelease nor ModuleOverride. Purging from FS", module)
			_ = os.RemoveAll(moduleLinkPath)
		}
	}

	return nil
}

func (c *moduleReleaseReconciler) readModulesFromFS(dir string) (map[string]string, error) {
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
func (c *moduleReleaseReconciler) restoreAbsentModulesFromReleases(ctx context.Context) error {
	var releaseList v1alpha1.ModuleReleaseList
	err := c.client.List(ctx, &releaseList)
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
		exists, err := c.isModulePullOverrideExists(ctx, moduleSource, moduleName)
		if err != nil {
			c.logger.Errorf("Couldn't check module pull override for module %s: %s", moduleName, err)
		}

		if exists {
			c.logger.Infof("ModulePullOverride for module %q exists. Skipping release restore", moduleName)
			continue
		}

		// get relevant module source
		ms := new(v1alpha1.ModuleSource)
		err = c.client.Get(ctx, types.NamespacedName{Name: moduleSource}, ms)
		if err != nil {
			return fmt.Errorf("ModuleSource %v for ModuleRelease/%s/%s got an error: %w", moduleSource, moduleName, moduleVersion, err)
		}

		moduleSymLink := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, item.Spec.ModuleName))
		_, err = os.Stat(moduleSymLink)
		if err != nil {
			// module symlink not found
			c.logger.Infof("Module %q symlink is absent on file system. Restoring it", moduleName)
			if os.IsNotExist(err) {
				err := c.createModuleSymlink(moduleName, moduleVersion, ms, moduleWeight)
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
				c.logger.Infof("Module %q symlink is incorrect. Restoring it", moduleName)
				if err := c.createModuleSymlink(moduleName, moduleVersion, ms, moduleWeight); err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
			}
		}

		// sync registry spec
		if err := syncModuleRegistrySpec(c.externalModulesDir, moduleName, moduleVersion, ms); err != nil {
			return fmt.Errorf("couldn't sync the %s module's registry settings with the %s module source: %w", moduleName, ms.Name, err)
		}
		c.logger.Infof("Resynced the %s module's registry settings with the %s module source", moduleName, ms.Name)
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
func syncModuleRegistrySpec(externalModulesDir, moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource) error {
	var openAPISpec moduleOpenAPISpec

	openAPIFile, err := os.Open(filepath.Join(externalModulesDir, moduleName, moduleVersion, "openapi/values.yaml"))
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

	if moduleSource.Spec.Registry.CA != registrySpec.CA.Default || moduleSource.Spec.Registry.DockerCFG != registrySpec.DockerCFG.Default || moduleSource.Spec.Registry.Repo != registrySpec.Base.Default || moduleSource.Spec.Registry.Scheme != registrySpec.Scheme.Default {
		err = downloader.InjectRegistryToModuleValues(filepath.Join(externalModulesDir, moduleName, moduleVersion), moduleSource)
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
func (c *moduleReleaseReconciler) createModuleSymlink(moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource, moduleWeight uint32) error {
	c.logger.Infof("Module %q is absent on file system. Restoring it from source %q", moduleName, moduleSource.Name)

	// removing possible symlink doubles
	err := wipeModuleSymlinks(c.symlinksDir, moduleName)
	if err != nil {
		return err
	}

	// check if module's directory exists on fs
	info, err := os.Stat(path.Join(c.externalModulesDir, moduleName, moduleVersion))
	if err != nil || !info.IsDir() {
		c.logger.Infof("Downloading module %q from registry", moduleName)
		// download the module to fs
		md := downloader.NewModuleDownloader(c.dc, c.externalModulesDir, moduleSource, utils.GenerateRegistryOptions(moduleSource))
		_, err = md.DownloadByModuleVersion(moduleName, moduleVersion)
		if err != nil {
			return fmt.Errorf("download module %v with version %v failed: %w. Skipping", moduleName, moduleVersion, err)
		}
	}

	// restore symlink
	moduleRelativePath := filepath.Join("../", moduleName, moduleVersion)
	symlinkPath := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
	err = restoreModuleSymlink(c.externalModulesDir, symlinkPath, moduleRelativePath)
	if err != nil {
		return fmt.Errorf("creating symlink for module %v failed: %w", moduleName, err)
	}
	c.logger.Infof("Module %s:%s restored to %s", moduleName, moduleVersion, moduleRelativePath)

	return nil
}

func (c *moduleReleaseReconciler) parseNotificationConfig(ctx context.Context) (*updater.NotificationConfig, error) {
	var secret corev1.Secret
	err := c.client.Get(ctx, types.NamespacedName{Name: "deckhouse-discovery", Namespace: "d8-system"}, &secret)
	if err != nil {
		return nil, fmt.Errorf("get secret: %w", err)
	}

	jsonSettings, ok := secret.Data["updateSettings.json"]
	if !ok {
		return new(updater.NotificationConfig), nil
	}

	var settings struct {
		NotificationConfig *updater.NotificationConfig `json:"notification"`
	}

	err = json.Unmarshal(jsonSettings, &settings)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	return settings.NotificationConfig, nil
}

func validateModule(def models.DeckhouseModuleDefinition) error {
	if def.Weight < 900 || def.Weight > 999 {
		return fmt.Errorf("external module weight must be between 900 and 999")
	}

	if def.Path == "" {
		return fmt.Errorf("cannot validate module without path. Path is required to load openapi specs")
	}

	dm, err := models.NewDeckhouseModule(def, addonutils.Values{}, nil, nil)
	if err != nil {
		return fmt.Errorf("new deckhouse module: %w", err)
	}

	err = dm.GetBasicModule().Validate()
	if err != nil {
		return fmt.Errorf("validate module: %w", err)
	}

	return nil
}

func restoreModuleSymlink(externalModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}

type moduleManager interface {
	DisableModuleHooks(moduleName string)
	GetModule(moduleName string) *addonmodules.BasicModule
	RunModuleWithNewStaticValues(moduleName, moduleSource, modulePath string) error
	GetEnabledModuleNames() []string
}

func (c *moduleReleaseReconciler) updateModuleReleaseDownloadStatistic(ctx context.Context, release *v1alpha1.ModuleRelease,
	ds *downloader.DownloadStatistic) (*v1alpha1.ModuleRelease, error) {
	release.Status.Size = ds.Size
	release.Status.PullDuration = metav1.Duration{Duration: ds.PullDuration}

	return release, c.client.Status().Update(ctx, release)
}

func (c *moduleReleaseReconciler) registerMetrics(ctx context.Context) error {
	var releasesList v1alpha1.ModuleReleaseList
	err := c.client.List(ctx, &releasesList)
	if err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for _, release := range releasesList.Items {
		l := map[string]string{
			"version": release.Spec.Version.String(),
			"module":  release.Spec.ModuleName,
		}

		c.metricStorage.GaugeSet("{PREFIX}module_pull_seconds_total", release.Status.PullDuration.Seconds(), l)
		c.metricStorage.GaugeSet("{PREFIX}module_size_bytes_total", float64(release.Status.Size), l)
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

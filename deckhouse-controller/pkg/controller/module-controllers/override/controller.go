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

package override

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	cp "github.com/otiai10/copy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-override-controller"

	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	defaultRequeueAfter     = time.Minute
)

func RegisterController(runtimeManager manager.Manager, mm moduleManager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               runtimeManager.GetClient(),
		log:                  logger,
		moduleManager:        mm,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		dependencyContainer:  dc,
	}

	r.init.Add(1)

	// add preflight
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	pullOverrideController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha2.ModulePullOverride{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(pullOverrideController)
}

type reconciler struct {
	init                 *sync.WaitGroup
	client               client.Client
	log                  *log.Logger
	dependencyContainer  dependency.Container
	moduleManager        moduleManager
	downloadedModulesDir string
	symlinksDir          string
	clusterUUID          string
}

type moduleManager interface {
	DisableModuleHooks(moduleName string)
	GetModule(moduleName string) *addonmodules.BasicModule
	RunModuleWithNewOpenAPISchema(moduleName, moduleSource, modulePath string) error
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

	r.log.Debug("controller is ready")

	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.log.Debugf("reconciling the '%s' module pull override", req.Name)
	mpo := new(v1alpha2.ModulePullOverride)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, mpo); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warnf("the '%s' module pull override not found", req.Name)
			return ctrl.Result{}, nil
		}
		r.log.Errorf("failed to get the '%s' module pull override: %v", req.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !mpo.DeletionTimestamp.IsZero() {
		r.log.Infof("deleting the '%s' module pull override", req.Name)
		return r.deleteModuleOverride(ctx, mpo)
	}

	// handle create/update events
	return r.handleModuleOverride(ctx, mpo)
}

func (r *reconciler) handleModuleOverride(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	defer r.log.Debugf("the '%s' module pull override reconciled", mpo.Name)

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Errorf("failed to get the '%s' module: %v", mpo.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
		r.log.Warnf("the '%s' module not found", mpo.Name)
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleNotFound {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleNotFound
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Errorf("failed to update the '%s' module pull override: %v", mpo.Name, uerr)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.log.Debugf("the '%s' module is embedded, skip it", mpo.Name)
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleEmbedded {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleEmbedded
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Errorf("failed to update the '%s' module pull override: %v", mpo.Name, uerr)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// module must be enabled
	if !module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig) {
		r.log.Debugf("the '%s' module is disabled, skip it", mpo.Name)
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleDisabled {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleDisabled
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Errorf("failed to update the '%s' module pull override: %v", mpo.Name, uerr)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// source must be
	if module.Properties.Source == "" {
		r.log.Debugf("the '%s' module does not have an active source, skip it", mpo.Name)
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageNoSource {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageNoSource
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Errorf("failed to update the '%s' module pull override: %v", mpo.Name, uerr)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// set condition overridden for the module
	err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		module.SetConditionTrue(v1alpha1.ModuleConditionIsOverridden)
		return true
	})
	if err != nil {
		r.log.Errorf("failed to update the '%s' module: %v", mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	var needUpdate bool

	// set finalizer if it is not set
	if !controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		controllerutil.AddFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		needUpdate = true
	}

	// check if RegistrySpecChanged annotation is set and process it
	if _, set := mpo.GetAnnotations()[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged]; set {
		// if module is enabled - push runModule task in the main queue
		r.log.Infof("apply new registry settings to the '%s' module", mpo.Name)
		modulePath := filepath.Join(r.downloadedModulesDir, mpo.Name, downloader.DefaultDevVersion)
		if err = r.moduleManager.RunModuleWithNewOpenAPISchema(mpo.Name, module.Properties.Source, modulePath); err != nil {
			r.log.Errorf("failed to run the '%s' module with new OpenAPI schema': %v", mpo.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
		// delete annotation and requeue
		delete(mpo.ObjectMeta.Annotations, v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged)
		needUpdate = true
	}

	if needUpdate {
		if err = r.client.Update(ctx, mpo); err != nil {
			r.log.Errorf("failed to update the '%s' module pull override: %v", mpo.Name, err)
		}
		return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, nil
	}

	source := new(v1alpha1.ModuleSource)
	if err = r.client.Get(ctx, client.ObjectKey{Name: module.Properties.Source}, source); err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Errorf("failed to get the '%s' module source for the '%s' module pull override: %v", module.Properties.Source, mpo.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageSourceNotFound {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageSourceNotFound
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Errorf("failed to update the '%s' module pull override status: %v", mpo.Name, uerr)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	tmpDir, err := os.MkdirTemp("", "module*")
	if err != nil {
		r.log.Errorf("failed to create temporary directory for the '%s' module pull override: %v", mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	// clear temp dir
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			r.log.Errorf("failed to remove the '%s' old module dir for the '%s' module pull override: %v", tmpDir, mpo.Name, err)
		}
	}()

	options := utils.GenerateRegistryOptionsFromModuleSource(source, r.clusterUUID, r.log)
	md := downloader.NewModuleDownloader(r.dependencyContainer, tmpDir, source, options)

	r.log.Debugf("downloading the '%s' tag of the '%s' module", mpo.Spec.ImageTag, mpo.Name)
	newChecksum, moduleDef, err := md.DownloadDevImageTag(mpo.Name, mpo.Spec.ImageTag, mpo.Status.ImageDigest)
	if err != nil {
		mpo.Status.Message = fmt.Sprintf("Download error: %v", err)
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Errorf("failed to update the '%s' module pull override status: %v", mpo.Name, uerr)
			return ctrl.Result{Requeue: true}, nil
		}
		r.log.Errorf("failed to download dev image tag for the '%s' module pull override: %v", mpo.Name, err)
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	// check if module is up-to-date
	if newChecksum == "" {
		r.log.Debugf("the '%s' module is up to date", mpo.Name)
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageReady
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Errorf("failed to update the '%s' module pull override status: %v", mpo.Name, uerr)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	if moduleDef == nil {
		mpo.Status.Message = v1alpha1.ModulePullOverrideMessageNoDef
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Errorf("failed to update the '%s' module pull override: %v", mpo.Name, uerr)
			return ctrl.Result{Requeue: true}, nil
		}
		r.log.Errorf("got an empty module definition for the '%s' module pull override", mpo.Name)
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	var values = make(addonutils.Values)
	if basicModule := r.moduleManager.GetModule(moduleDef.Name); basicModule != nil {
		values = basicModule.GetConfigValues(false)
	}

	if err = moduleDef.Validate(values, r.log); err != nil {
		mpo.Status.Message = fmt.Sprintf("Validation error: %v", err)
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Errorf("failed to update the '%s' module pull override status: %v", mpo.Name, uerr)
			return ctrl.Result{Requeue: true}, nil
		}
		r.log.Errorf("failed to validate the '%s' module pull override: %v", mpo.Name, err)
		return ctrl.Result{}, nil
	}

	moduleStorePath := path.Join(r.downloadedModulesDir, moduleDef.Name, downloader.DefaultDevVersion)
	if err = os.RemoveAll(moduleStorePath); err != nil {
		r.log.Errorf("failed to remove the '%s' old module dir for the '%s' module pull override: %v", moduleStorePath, mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	if err = cp.Copy(tmpDir, r.downloadedModulesDir); err != nil {
		r.log.Errorf("failed to copy the module from the downloaded module dir for the '%s' module pull override: %v", mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	symlinkPath := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", moduleDef.Weight, mpo.Name))
	if err = r.enableModule(mpo.Name, symlinkPath); err != nil {
		mpo.Status.Message = fmt.Sprintf("Enable error: %v", err)
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Errorf("failed to update the '%s' module pull override status: %v", mpo.Name, uerr)
			return ctrl.Result{Requeue: true}, nil
		}
		r.log.Errorf("failed to enable the '%s' module: %v", mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	// disable target module hooks so as not to invoke them before restart
	if r.moduleManager.GetModule(mpo.Name) != nil {
		r.moduleManager.DisableModuleHooks(mpo.Name)
	}

	defer func() {
		r.log.Infof("restart Deckhouse because %q ModulePullOverride image was updated", mpo.Name)
		if err = syscall.Kill(1, syscall.SIGUSR2); err != nil {
			r.log.Fatalf("failed to send SIGUSR2 signal: %v", err)
		}
	}()

	mpo.Status.Message = v1alpha1.ModulePullOverrideMessageReady
	mpo.Status.ImageDigest = newChecksum
	mpo.Status.Weight = moduleDef.Weight

	if err = r.updateModulePullOverrideStatus(ctx, mpo); err != nil {
		r.log.Errorf("failed to update the '%s' module pull override status: %v", mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	// TODO: What is it ?
	if _, ok := mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationRenew]; ok {
		delete(mpo.Annotations, v1alpha1.ModulePullOverrideAnnotationRenew)
		_ = r.client.Update(ctx, mpo)
	}

	modulePath := fmt.Sprintf("/%s/dev", mpo.GetModuleName())
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha2.ModulePullOverrideGVK.GroupVersion().String(),
		Kind:       v1alpha2.ModulePullOverrideGVK.Kind,
		Name:       mpo.GetName(),
		UID:        mpo.GetUID(),
		Controller: ptr.To(true),
	}

	if err = utils.EnsureModuleDocumentation(ctx, r.client, mpo.Name, module.Properties.Source, mpo.Status.ImageDigest, mpo.Spec.ImageTag, modulePath, ownerRef); err != nil {
		r.log.Errorf("failed to ensure module documentation for the '%s' module pull override: %v", mpo.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
}

func (r *reconciler) deleteModuleOverride(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		module := new(v1alpha1.Module)
		if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.GetName()}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				r.log.Errorf("failed to get the '%s' module: %v", mpo.GetName(), err)
				return ctrl.Result{Requeue: true}, nil
			}
			controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
			if err = r.client.Update(ctx, mpo); err != nil {
				r.log.Errorf("failed to remove finalizer for the '%s' module pull override: %v", mpo.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, nil
		}

		err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(mpo *v1alpha1.Module) bool {
			mpo.SetConditionFalse(v1alpha1.ModuleConditionIsOverridden, "", "")
			return true
		})
		if err != nil {
			r.log.Errorf("failed to update the '%s' module status: %v", mpo.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}

		controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		if err = r.client.Update(ctx, mpo); err != nil {
			r.log.Errorf("failed to remove finalizer for the '%s' module pull override: %v", mpo.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) enableModule(moduleName, symlinkPath string) error {
	currentModuleSymlink, err := utils.GetModuleSymlink(r.symlinksDir, moduleName)
	if err != nil {
		r.log.Warnf("failed to find the current module symlink for the '%s' module: %v", moduleName, err)
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	modulePath := path.Join("../", moduleName, downloader.DefaultDevVersion)
	return utils.EnableModule(r.downloadedModulesDir, currentModuleSymlink, symlinkPath, modulePath)
}

func (r *reconciler) updateModulePullOverrideStatus(ctx context.Context, mpo *v1alpha2.ModulePullOverride) error {
	mpo.Status.UpdatedAt = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
	return r.client.Status().Update(ctx, mpo)
}

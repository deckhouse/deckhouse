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
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	cp "github.com/otiai10/copy"
	corev1 "k8s.io/api/core/v1"
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

	r.log.Debug("reconciling module pull override", slog.String("name", req.Name))
	mpo := new(v1alpha2.ModulePullOverride)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, mpo); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warn("module pull override not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}
		r.log.Error("failed to get module pull override", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !mpo.DeletionTimestamp.IsZero() {
		r.log.Info("deleting the module pull override", slog.String("name", req.Name))
		return r.deleteModuleOverride(ctx, mpo)
	}

	// handle create/update events
	return r.handleModuleOverride(ctx, mpo)
}

func (r *reconciler) handleModuleOverride(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	defer r.log.Debug("module pull override reconciled", slog.String("name", mpo.Name))

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error("failed to get module", slog.String("name", mpo.Name), log.Err(err))

			return ctrl.Result{}, err
		}

		r.log.Warn("module not found", slog.String("name", mpo.Name))
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleNotFound {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleNotFound
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update module pull override", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// skip embedded modules
	if module.IsEmbedded() {
		r.log.Debug("module is embedded, skip it", slog.String("name", mpo.Name))
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleEmbedded {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleEmbedded
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update module pull override", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
			}
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// module must be enabled
	if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
		r.log.Debug("module is disabled, skip it", slog.String("name", mpo.Name))
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleDisabled {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleDisabled
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update module pull override", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// source must be
	if module.Properties.Source == "" {
		r.log.Debug("module does not have an active source, skip it", slog.String("name", mpo.Name))
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageNoSource {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageNoSource
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update module pull override", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
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
		r.log.Error("failed to update module", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	var needUpdate bool

	// set finalizer if it is not set
	if !controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		controllerutil.AddFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		needUpdate = true
	}

	if needUpdate {
		if err = r.client.Update(ctx, mpo); err != nil {
			r.log.Error("failed to update the module pull override", slog.String("name", mpo.Name), log.Err(err))
		}
		return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, nil
	}

	source := new(v1alpha1.ModuleSource)
	if err = r.client.Get(ctx, client.ObjectKey{Name: module.Properties.Source}, source); err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error("failed to get the module source for the module pull override", slog.String("source", module.Properties.Source), slog.String("target", mpo.Name), log.Err(err))
			return ctrl.Result{}, err
		}
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageSourceNotFound {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageSourceNotFound
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
			}
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	tmpDir, err := os.MkdirTemp("", "module*")
	if err != nil {
		r.log.Error("failed to create temporary directory for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// clear temp dir
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			r.log.Error("failed to remove the old module dir for the module pull override", slog.String("old_dir", tmpDir), slog.String("name", mpo.Name), log.Err(err))
		}
	}()

	options := utils.GenerateRegistryOptionsFromModuleSource(source, r.clusterUUID, r.log)
	md := downloader.NewModuleDownloader(r.dependencyContainer, tmpDir, source, r.log.Named("downloader"), options)

	r.log.Debug("downloading tag of module", slog.String("tag", mpo.Spec.ImageTag), slog.String("name", mpo.Name))
	newChecksum, moduleDef, err := md.DownloadDevImageTag(mpo.Name, mpo.Spec.ImageTag, mpo.Status.ImageDigest)
	if err != nil {
		mpo.Status.Message = fmt.Sprintf("Download error: %v", err)
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}
		r.log.Error("failed to download dev image tag for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	// check if module is up-to-date
	if newChecksum == "" {
		r.log.Debug("module is up to date", slog.String("name", mpo.Name))
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageReady
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
			}
		}
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	if moduleDef == nil {
		mpo.Status.Message = v1alpha1.ModulePullOverrideMessageNoDef
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Error("failed to update the module pull override", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}
		r.log.Error("got an empty module definition for the module pull override", slog.String("name", mpo.Name))
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	values := make(addonutils.Values)
	if module := r.moduleManager.GetModule(mpo.GetModuleName()); module != nil {
		values = module.GetConfigValues(false)
	} else {
		config := new(v1alpha1.ModuleConfig)
		if err = r.client.Get(ctx, client.ObjectKey{Name: mpo.GetModuleName()}, config); err != nil {
			if !apierrors.IsNotFound(err) {
				r.log.Error("failed to get the module config", slog.String("name", mpo.GetModuleName()), log.Err(err))
				return ctrl.Result{}, err
			}
		} else {
			values = addonutils.Values(config.Spec.Settings)
		}
	}

	if err = moduleDef.Validate(values, r.log); err != nil {
		mpo.Status.Message = fmt.Sprintf("Validation error: %v", err)
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}
		r.log.Error("failed to validate the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
	}

	moduleStorePath := path.Join(r.downloadedModulesDir, moduleDef.Name, downloader.DefaultDevVersion)
	if err = os.RemoveAll(moduleStorePath); err != nil {
		r.log.Error("failed to remove the old module dir for the module pull override", slog.String("old_dir", moduleStorePath), slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if err = cp.Copy(tmpDir, r.downloadedModulesDir); err != nil {
		r.log.Error("failed to copy the module from the downloaded module dir for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	symlinkPath := filepath.Join(r.symlinksDir, fmt.Sprintf("%d-%s", moduleDef.Weight, mpo.Name))
	if err = r.enableModule(mpo.Name, symlinkPath); err != nil {
		mpo.Status.Message = fmt.Sprintf("Enable error: %v", err)
		if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
			r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}
		r.log.Error("failed to enable the module", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// disable target module hooks so as not to invoke them before restart
	if r.moduleManager.GetModule(mpo.Name) != nil {
		r.moduleManager.DisableModuleHooks(mpo.Name)
	}

	defer func() {
		r.log.Info("restart Deckhouse because ModulePullOverride image was updated", slog.String("name", mpo.Name))
		if err = syscall.Kill(1, syscall.SIGUSR2); err != nil {
			r.log.Fatal("failed to send SIGUSR2 signal", log.Err(err))
		}
	}()

	mpo.Status.Message = v1alpha1.ModulePullOverrideMessageReady
	mpo.Status.ImageDigest = newChecksum
	mpo.Status.Weight = moduleDef.Weight

	if err = r.updateModulePullOverrideStatus(ctx, mpo); err != nil {
		r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
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
		r.log.Error("failed to ensure module documentation for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
}

func (r *reconciler) deleteModuleOverride(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		if mpo.Spec.Rollback {
			// clear symlink dir
			if err := os.RemoveAll(path.Join(r.symlinksDir, mpo.Name)); err != nil {
				r.log.Error("failed to remove the module pull override symlink", slog.String("name", mpo.Name), log.Err(err))
				return ctrl.Result{}, err
			}
			// clear downloaded dir
			if err := os.RemoveAll(path.Join(r.downloadedModulesDir, mpo.GetModuleName(), downloader.DefaultDevVersion)); err != nil {
				r.log.Error("failed to remove the module pull override downloaded dir", slog.String("name", mpo.Name), log.Err(err))
				return ctrl.Result{}, err
			}
			// restart deckhouse
			defer func() {
				r.log.Info("restart deckhouse because module rollback", slog.String("name", mpo.Name))
				if err := syscall.Kill(1, syscall.SIGUSR2); err != nil {
					r.log.Fatal("failed to send SIGUSR2 signal", log.Err(err))
				}
			}()
		}

		module := new(v1alpha1.Module)
		if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.GetName()}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				r.log.Error("failed to get the module", slog.String("name", mpo.GetName()), log.Err(err))
				return ctrl.Result{Requeue: true}, nil
			}
			controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
			if err = r.client.Update(ctx, mpo); err != nil {
				r.log.Error("failed to remove finalizer for the module pull override", slog.String("name", mpo.Name), log.Err(err))
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, nil
		}

		err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(mpo *v1alpha1.Module) bool {
			mpo.SetConditionFalse(v1alpha1.ModuleConditionIsOverridden, "", "")
			return true
		})
		if err != nil {
			r.log.Error("failed to update the module status", slog.String("name", mpo.Name), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}

		controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		if err = r.client.Update(ctx, mpo); err != nil {
			r.log.Error("failed to remove finalizer for the module pull override", slog.String("name", mpo.Name), log.Err(err))
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) enableModule(moduleName, symlinkPath string) error {
	currentModuleSymlink, err := utils.GetModuleSymlink(r.symlinksDir, moduleName)
	if err != nil {
		r.log.Warn("failed to find the current module symlink", slog.String("name", moduleName), log.Err(err))
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	modulePath := path.Join("../", moduleName, downloader.DefaultDevVersion)
	return utils.EnableModule(r.downloadedModulesDir, currentModuleSymlink, symlinkPath, modulePath)
}

func (r *reconciler) updateModulePullOverrideStatus(ctx context.Context, mpo *v1alpha2.ModulePullOverride) error {
	mpo.Status.UpdatedAt = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
	return r.client.Status().Update(ctx, mpo)
}

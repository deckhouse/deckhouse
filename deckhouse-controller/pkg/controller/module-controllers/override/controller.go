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
	"sync"
	"syscall"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-override-controller"

	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	defaultRequeueAfter     = time.Minute
)

func RegisterController(runtimeManager manager.Manager,
	mm moduleManager,
	loader *moduleloader.Loader,
	dc dependency.Container,
	logger *log.Logger) error {
	r := &reconciler{
		init:                new(sync.WaitGroup),
		client:              runtimeManager.GetClient(),
		log:                 logger,
		loader:              loader,
		moduleManager:       mm,
		dependencyContainer: dc,
	}

	r.init.Add(1)

	// add preflight
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(runtimeManager).
		Named(controllerName).
		For(&v1alpha2.ModulePullOverride{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	return nil
}

type reconciler struct {
	init                *sync.WaitGroup
	client              client.Client
	loader              *moduleloader.Loader
	log                 *log.Logger
	dependencyContainer dependency.Container
	moduleManager       moduleManager
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
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
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

	// a module the override installs for the first time has no object yet
	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error("failed to get module", slog.String("name", mpo.Name), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("get: %w", err)
		}

		module = nil
	}

	// skip embedded modules
	if module != nil && module.IsEmbedded() {
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

	enabled, err := r.moduleEnabled(ctx, mpo.Name, module)
	if err != nil {
		r.log.Error("failed to check whether the module is enabled", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	if !enabled {
		r.log.Debug("module is disabled, skip it", slog.String("name", mpo.Name))
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageModuleDisabled {
			mpo.Status.Message = v1alpha1.ModulePullOverrideMessageModuleDisabled
			// unset image digest to trigger latter downloading
			mpo.Status.ImageDigest = ""
			if uerr := r.updateModulePullOverrideStatus(ctx, mpo); uerr != nil {
				r.log.Error("failed to update module pull override", slog.String("name", mpo.Name), log.Err(uerr))
				return ctrl.Result{}, uerr
			}
		}

		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	}

	// the override carries no source: the repository is resolved from the resources naming one
	repository, err := pkgsync.ModuleRepository(ctx, r.client, mpo.Name)
	if err != nil {
		r.log.Error("failed to resolve the module repository", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	sourceName := pkgsync.SourceNameForRepository(repository)
	if sourceName == "" {
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

	// the module follows the override from here: the tag is its version, the dev annotation
	// routes it, and the condition reports the override to the old stack
	if err := r.ensureDevModule(ctx, mpo, repository); err != nil {
		r.log.Error("failed to mark the module overridden", slog.String("name", mpo.Name), log.Err(err))
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
	if err = r.client.Get(ctx, client.ObjectKey{Name: sourceName}, source); err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error("failed to get the module source for the module pull override", slog.String("module_source", sourceName), slog.String("target", mpo.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("get: %w", err)
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

	digest, err := r.loader.Installer().GetImageDigest(ctx, source, mpo.Name, mpo.Spec.ImageTag)
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
	if digest == mpo.Status.ImageDigest {
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

	if err = r.deployModule(ctx, source, mpo); err != nil {
		r.log.Error("failed to deploy module", slog.String("module", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	defer func() {
		r.log.Info("restart Deckhouse because ModulePullOverride image was updated", slog.String("name", mpo.Name))
		if err = syscall.Kill(1, syscall.SIGUSR2); err != nil {
			r.log.Fatal("failed to send SIGUSR2 signal", log.Err(err))
		}
	}()

	mpo.Status.Message = v1alpha1.ModulePullOverrideMessageReady
	mpo.Status.ImageDigest = digest

	if err = r.updateModulePullOverrideStatus(ctx, mpo); err != nil {
		r.log.Error("failed to update the module pull override status", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, err
	}

	// TODO: What is it ?
	if _, ok := mpo.Annotations[v1alpha1.ModulePullOverrideAnnotationRenew]; ok {
		delete(mpo.Annotations, v1alpha1.ModulePullOverrideAnnotationRenew)
		_ = r.client.Update(ctx, mpo)
	}

	// Use mount point path: /modules/<module> (modules are mounted at /deckhouse/downloaded/modules/<module>)
	modulePath := fmt.Sprintf("/modules/%s", mpo.GetModuleName())
	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha2.ModulePullOverrideGVK.GroupVersion().String(),
		Kind:       v1alpha2.ModulePullOverrideGVK.Kind,
		Name:       mpo.GetName(),
		UID:        mpo.GetUID(),
		Controller: ptr.To(true),
	}

	if err = utils.EnsureModuleDocumentation(ctx, r.client, mpo.Name, sourceName, mpo.Status.ImageDigest, mpo.Spec.ImageTag, modulePath, ownerRef); err != nil {
		r.log.Error("failed to ensure module documentation for the module pull override", slog.String("name", mpo.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("ensure module documentation: %w", err)
	}

	return ctrl.Result{RequeueAfter: mpo.Spec.ScanInterval.Duration}, nil
}

// moduleEnabled reports whether the module may be pulled: a module config decides explicitly,
// and without one the module manager's effective state does. A module never installed and not
// configured is off, so a first override needs a ModuleConfig with enabled: true.
func (r *reconciler) moduleEnabled(ctx context.Context, name string, module *v1alpha2.Module) (bool, error) {
	config := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, config); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("get the module config: %w", err)
		}
	} else if config.Spec.Enabled != nil {
		return *config.Spec.Enabled, nil
	}

	return module != nil && module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleManager, metav1.ConditionTrue), nil
}

// ensureDevModule places the module by the override: created when it has no object, moved
// onto the image tag otherwise, marked dev and overridden.
func (r *reconciler) ensureDevModule(ctx context.Context, mpo *v1alpha2.ModulePullOverride, repository string) error {
	module := new(v1alpha2.Module)
	err := r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module)
	if apierrors.IsNotFound(err) {
		module = &v1alpha2.Module{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha2.ModuleGVK.GroupVersion().String(),
				Kind:       v1alpha2.ModuleKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        mpo.Name,
				Annotations: map[string]string{v1alpha2.ModuleAnnotationDev: "true"},
			},
			Spec: v1alpha2.ModuleSpec{
				PackageRepositoryName: repository,
				PackageVersion:        mpo.Spec.ImageTag,
			},
		}

		err = r.client.Create(ctx, module)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create the module: %w", err)
		}

		// the source controller placed the offered module meanwhile: move that object
		if err != nil {
			module = new(v1alpha2.Module)
			err = r.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module)
		}
	}

	if err != nil {
		return fmt.Errorf("get the module: %w", err)
	}

	patch := client.MergeFrom(module.DeepCopy())

	if module.Annotations == nil {
		module.Annotations = make(map[string]string)
	}
	module.Annotations[v1alpha2.ModuleAnnotationDev] = "true"
	module.Spec.PackageRepositoryName = repository
	module.Spec.PackageVersion = mpo.Spec.ImageTag

	data, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build patch: %w", err)
	}

	// a module just created by this override carries the placement already
	if string(data) != "{}" {
		if err := r.client.Patch(ctx, module, client.RawPatch(patch.Type(), data)); err != nil {
			return fmt.Errorf("patch the module: %w", err)
		}
	}

	return utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionIsOverridden, metav1.ConditionTrue) {
			return false
		}

		module.SetConditionTrue(v1alpha1.ModuleConditionIsOverridden, v1alpha1.ModuleReasonOverridden)

		return true
	})
}

// deployModule downloads module on tmp, validates and installs
func (r *reconciler) deployModule(ctx context.Context, source *v1alpha1.ModuleSource, mpo *v1alpha2.ModulePullOverride) error {
	modulePath, err := r.loader.Installer().Download(ctx, source, mpo.Name, mpo.Spec.ImageTag)
	if err != nil {
		return fmt.Errorf("download the module '%s': %w", mpo.Name, err)
	}

	// clear tmp module dir
	defer func() {
		if err = os.RemoveAll(modulePath); err != nil {
			r.log.Error("failed to remove module path", slog.String("path", modulePath), log.Err(err))
		}
	}()

	def := &moduletypes.Definition{
		Name: mpo.Name,
		Path: modulePath,
	}

	values := make(addonutils.Values)
	if module := r.moduleManager.GetModule(mpo.GetModuleName()); module != nil {
		values = module.GetConfigValues(false)
	} else {
		config := new(v1alpha1.ModuleConfig)
		if err = r.client.Get(ctx, client.ObjectKey{Name: mpo.GetModuleName()}, config); err != nil {
			if !apierrors.IsNotFound(err) {
				r.log.Error("failed to get the module config", slog.String("name", mpo.GetModuleName()), log.Err(err))
				return err
			}
		} else {
			settings := config.Spec.Settings.GetMap()

			values = addonutils.Values(settings)
		}
	}
	if err := def.Validate(values, r.log); err != nil {
		mpo.Status.Message = fmt.Sprintf("Validation error: %v", err)
		if err := r.updateModulePullOverrideStatus(ctx, mpo); err != nil {
			return fmt.Errorf("update mpo status: %w", err)
		}

		return fmt.Errorf("validation error: %w", err)
	}

	if err = r.loader.Installer().Install(ctx, mpo.Name, mpo.Spec.ImageTag, modulePath); err != nil {
		return fmt.Errorf("install the module '%s': %w", mpo.Name, err)
	}

	// disable target module hooks so as not to invoke them before restart
	if r.moduleManager.GetModule(mpo.Name) != nil {
		r.moduleManager.DisableModuleHooks(mpo.Name)
	}

	return nil
}

func (r *reconciler) deleteModuleOverride(ctx context.Context, mpo *v1alpha2.ModulePullOverride) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer) {
		if mpo.Spec.Rollback {
			if err := r.loader.Installer().Uninstall(ctx, mpo.Name); err != nil {
				return ctrl.Result{}, fmt.Errorf("uninstall the module '%s': %w", mpo.Name, err)
			}

			// restart deckhouse
			defer func() {
				r.log.Info("restart deckhouse because module rollback", slog.String("name", mpo.Name))
				if err := syscall.Kill(1, syscall.SIGUSR2); err != nil {
					r.log.Fatal("failed to send SIGUSR2 signal", log.Err(err))
				}
			}()
		}

		module := new(v1alpha2.Module)
		if err := r.client.Get(ctx, client.ObjectKey{Name: mpo.GetName()}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				r.log.Error("failed to get the module", slog.String("name", mpo.GetName()), log.Err(err))
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}

			controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
			if err = r.client.Update(ctx, mpo); err != nil {
				r.log.Error("failed to remove finalizer for the module pull override", slog.String("name", mpo.Name), log.Err(err))
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}

			return ctrl.Result{}, nil
		}

		// the module no longer follows a tag; the sync at the next start places it by its release
		if err := r.releaseDevModule(ctx, module); err != nil {
			r.log.Error("failed to release the module from the override", slog.String("name", mpo.Name), log.Err(err))
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}

		controllerutil.RemoveFinalizer(mpo, v1alpha1.ModulePullOverrideFinalizer)
		if err := r.client.Update(ctx, mpo); err != nil {
			r.log.Error("failed to remove finalizer for the module pull override", slog.String("name", mpo.Name), log.Err(err))
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) updateModulePullOverrideStatus(ctx context.Context, mpo *v1alpha2.ModulePullOverride) error {
	mpo.Status.UpdatedAt = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
	if err := r.client.Status().Update(ctx, mpo); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// releaseDevModule drops the dev mark of a module whose override is gone and reports it.
func (r *reconciler) releaseDevModule(ctx context.Context, module *v1alpha2.Module) error {
	if _, ok := module.Annotations[v1alpha2.ModuleAnnotationDev]; ok {
		patch := client.MergeFrom(module.DeepCopy())
		delete(module.Annotations, v1alpha2.ModuleAnnotationDev)

		if err := r.client.Patch(ctx, module, patch); err != nil {
			return fmt.Errorf("patch the module: %w", err)
		}
	}

	return utils.UpdateStatus[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if module.IsCondition(v1alpha1.ModuleConditionIsOverridden, metav1.ConditionFalse) {
			return false
		}

		module.SetConditionFalse(v1alpha1.ModuleConditionIsOverridden, v1alpha1.ModuleReasonDisabled, "")

		return true
	})
}

// Copyright 2024 Flant JSC
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

package source

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	d8env "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-source-controller"

	deckhouseNamespace = "d8-system"

	deckhouseDiscoverySecret = "deckhouse-discovery"

	moduleSourceReleaseExistsFinalizer = "modules.deckhouse.io/release-exists"
	moduleSourceModuleExistsFinalizer  = "modules.deckhouse.io/module-exists"

	moduleSourceAnnotationForceDelete      = "modules.deckhouse.io/force-delete"
	moduleSourceAnnotationRegistryChecksum = "modules.deckhouse.io/registry-spec-checksum"

	defaultScanInterval = 3 * time.Minute

	maxConcurrentReconciles = 3
	cacheSyncTimeout        = 3 * time.Minute
)

func RegisterController(runtimeManager manager.Manager, dc dependency.Container, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, logger *log.Logger) error {
	r := &reconciler{
		init:                 &sync.WaitGroup{},
		client:               runtimeManager.GetClient(),
		log:                  logger,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		embeddedPolicy:       embeddedPolicy,
		dependencyContainer:  dc,
	}

	r.init.Add(1)

	// add preflight to set the cluster UUID
	if err := runtimeManager.Add(manager.RunnableFunc(r.setClusterUUID)); err != nil {
		return err
	}

	sourceController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModuleSource{}).
		Watches(&v1alpha1.Module{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.(*v1alpha1.Module).Properties.Source}}}
		}), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldMod := updateEvent.ObjectOld.(*v1alpha1.Module)
				newMod := updateEvent.ObjectNew.(*v1alpha1.Module)
				// handle change source
				if oldMod.Properties.Source != newMod.Properties.Source {
					return true
				}
				// handle change policy
				if oldMod.Properties.UpdatePolicy != newMod.Properties.UpdatePolicy {
					return true
				}
				// handle enable
				if !oldMod.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig) && newMod.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig) {
					return true
				}
				return false
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(sourceController)
}

type reconciler struct {
	init                 *sync.WaitGroup
	client               client.Client
	log                  *log.Logger
	dependencyContainer  dependency.Container
	embeddedPolicy       *helpers.ModuleUpdatePolicySpecContainer
	downloadedModulesDir string
	clusterUUID          string
}

func (r *reconciler) setClusterUUID(ctx context.Context) error {
	defer r.init.Done()

	// attempt to read the cluster UUID from a secret
	secret := new(corev1.Secret)
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: deckhouseNamespace, Name: deckhouseDiscoverySecret}, secret); err != nil {
		r.log.Warnf("failed to read clusterUUID from the 'deckhouse-discovery' secret: %v. Generating random uuid", err)
		r.clusterUUID = uuid.Must(uuid.NewV4()).String()
		return nil
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		r.clusterUUID = string(clusterUUID)
		return nil
	}

	// generate a random UUID if the key is missing
	r.clusterUUID = uuid.Must(uuid.NewV4()).String()
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.log.Debugf("reconciling the '%s' module source", req.Name)
	moduleSource := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, req.NamespacedName, moduleSource); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Warnf("the '%s' module source not found", req.Name)
			return ctrl.Result{}, nil
		}
		r.log.Errorf("failed to get the '%s' module source: %v", req.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !moduleSource.DeletionTimestamp.IsZero() {
		r.log.Debugf("deleting the '%s' module source", req.Name)
		return r.deleteModuleSource(ctx, moduleSource)
	}

	// handle create/update events
	return r.handleModuleSource(ctx, moduleSource)
}

func (r *reconciler) handleModuleSource(ctx context.Context, source *v1alpha1.ModuleSource) (ctrl.Result, error) {
	// generate options for connecting to the registry
	opts := utils.GenerateRegistryOptionsFromModuleSource(source, r.clusterUUID, r.log)

	// create a registry client
	registryClient, err := r.dependencyContainer.GetRegistryClient(source.Spec.Registry.Repo, opts...)
	if err != nil {
		r.log.Errorf("failed to get registry client for the '%s' module source: %v", source.Name, err)
		if uerr := r.updateModuleSourceStatusMessage(ctx, source, err.Error()); uerr != nil {
			return ctrl.Result{Requeue: true}, nil
		}
		// error can occur on wrong auth only, we don't want to requeue the source until auth is fixed
		return ctrl.Result{Requeue: false}, nil
	}

	// sync registry settings
	shouldRequeue, err := r.syncRegistrySettings(ctx, source)
	if err != nil {
		r.log.Errorf("failed to sync registry settings for the '%s' module source: %v", source.Name, err)
		if uerr := r.updateModuleSourceStatusMessage(ctx, source, err.Error()); uerr != nil {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if shouldRequeue {
		// new registry settings checksum should be applied to module source
		if err = r.client.Update(ctx, source); err != nil {
			r.log.Errorf("failed to update the '%s' module source status: %v", source.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
		// requeue moduleSource after modifying annotation
		r.log.Debugf("the '%s' module source will be requeued", source.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// list available modules(tags) from the registry
	r.log.Debugf("fetch modules from the '%s' module source", source.Name)
	pulledModules, err := registryClient.ListTags(ctx)
	if err != nil {
		r.log.Errorf("failed to list tags for the '%s' module source: %v", source.Name, err)
		if uerr := r.updateModuleSourceStatusMessage(ctx, source, err.Error()); uerr != nil {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	sort.Strings(pulledModules)

	// remove the source from available sources in deleted modules
	nameSet := make(map[string]bool)
	for _, pulledModuleName := range pulledModules {
		nameSet[pulledModuleName] = true
	}
	for _, availableModule := range source.Status.AvailableModules {
		if !nameSet[availableModule.Name] {
			if err = r.cleanSourceInModule(ctx, source.Name, availableModule.Name); err != nil {
				r.log.Errorf("failed to clean the module from the '%s' module source: %v", availableModule.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	}

	if err = r.processModules(ctx, source, opts, pulledModules); err != nil {
		r.log.Errorf("failed to process modules for the '%s' module source: %v", source.Name, err)
		return ctrl.Result{Requeue: true}, nil
	}
	r.log.Debugf("the '%s' module source reconciled", source.Name)

	// everything is ok, check source on the other iteration
	return ctrl.Result{RequeueAfter: defaultScanInterval}, nil
}

func (r *reconciler) processModules(ctx context.Context, source *v1alpha1.ModuleSource, opts []cr.Option, pulledModules []string) error {
	md := downloader.NewModuleDownloader(r.dependencyContainer, r.downloadedModulesDir, source, opts)

	var availableModules []v1alpha1.AvailableModule
	var pullErrorsExist bool
	for _, moduleName := range pulledModules {
		if moduleName == "modules" {
			r.log.Warn("the 'modules' is a forbidden name, skip the module.")
			continue
		}

		availableModule := v1alpha1.AvailableModule{Name: moduleName}
		for _, available := range source.Status.AvailableModules {
			if available.Name == moduleName {
				availableModule = available
			}
		}

		// clear error
		availableModule.PullError = ""

		// get update policy
		policy, err := utils.UpdatePolicy(ctx, r.client, r.embeddedPolicy, moduleName)
		if err != nil {
			return fmt.Errorf("failed to get update policy for the '%s' module: %v", moduleName, err)
		}

		// TODO(ipaqsa): can be removed
		availableModule.Policy = policy.Name

		// create or update module
		module, err := r.ensureModule(ctx, source.Name, moduleName)
		if err != nil {
			return fmt.Errorf("failed to ensure module for the '%s' module source: %v", moduleName, err)
		}

		// add module name as a possible name for validation module config webhook
		d8config.Service().AddPossibleName(moduleName)

		if module == nil {
			availableModules = append(availableModules, availableModule)
			// skip module
			continue
		}

		exist, err := utils.ModulePullOverrideExists(ctx, r.client, source.Name, moduleName)
		if err != nil {
			return fmt.Errorf("failed to get pull override for the '%s' module: %v", moduleName, err)
		}

		if exist {
			availableModules = append(availableModules, availableModule)
			// skip overridden module
			availableModule.Overridden = true
			continue
		}

		var cachedChecksum = availableModule.Checksum

		// check if release exists
		exist, err = r.releaseExists(ctx, moduleName, cachedChecksum)
		if err != nil {
			return fmt.Errorf("failed to check if the module '%s' has a release: %v", moduleName, err)
		}
		if !exist {
			// if release does not exist, clear checksum to trigger meta downloading
			cachedChecksum = ""
		}

		// download module metadata from the specified release channel
		r.log.Debugf("download meta from the '%s' release channel for the '%s' module for the '%s' module source", policy.Spec.ReleaseChannel, moduleName, source.Name)
		meta, err := md.DownloadMetadataFromReleaseChannel(moduleName, policy.Spec.ReleaseChannel, cachedChecksum)
		if err != nil {
			availableModule.PullError = err.Error()
			availableModules = append(availableModules, availableModule)
			pullErrorsExist = true
			continue
		}

		if availableModule.Checksum != meta.Checksum || (meta.ModuleVersion != "" && !exist) {
			r.log.Debugf("ensure module release for the '%s' module for the '%s' module source", moduleName, source.Name)
			if err = r.ensureModuleRelease(ctx, source.GetUID(), source.Name, moduleName, policy.Name, meta); err != nil {
				return fmt.Errorf("failed to ensure module release for the '%s'", moduleName)
			}
			availableModule.Checksum = meta.Checksum
		} else {
			r.log.Debugf("the '%s' module in the '%s' module source not changed", moduleName, source.Name)
			err = utils.Update[*v1alpha1.Module](ctx, r.client, module, true, func(module *v1alpha1.Module) bool {
				module.Status.Phase = v1alpha1.ModulePhaseReady

				module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, true)
				module.SetConditionReason(v1alpha1.ModuleConditionIsReady, "", "")
				return true
			})
			if err != nil {
				return fmt.Errorf("failed to update the '%s' module: %v", moduleName, err)
			}
		}
		availableModules = append(availableModules, availableModule)

		// update release channel
		err = utils.Update[*v1alpha1.Module](ctx, r.client, module, false, func(module *v1alpha1.Module) bool {
			if module.Properties.ReleaseChannel != policy.Spec.ReleaseChannel {
				module.Properties.ReleaseChannel = policy.Spec.ReleaseChannel
				return true
			}
			return false
		})
		if err != nil {
			return fmt.Errorf("failed to update release channel or version for the '%s' module: %v", moduleName, err)
		}
	}
	// update status
	err := utils.Update[*v1alpha1.ModuleSource](ctx, r.client, source, true, func(obj *v1alpha1.ModuleSource) bool {
		source.Status.Message = ""
		source.Status.SyncTime = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
		source.Status.AvailableModules = availableModules
		source.Status.ModulesCount = len(availableModules)
		if pullErrorsExist {
			source.Status.Message = "Some errors occurred. Inspect status for details"
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("failed to update the '%s' module source status: %v", source.Name, err)
	}

	// set finalizer
	err = utils.Update[*v1alpha1.ModuleSource](ctx, r.client, source, false, func(obj *v1alpha1.ModuleSource) bool {
		if len(source.Status.AvailableModules) > 0 && !controllerutil.ContainsFinalizer(source, moduleSourceModuleExistsFinalizer) {
			controllerutil.AddFinalizer(source, moduleSourceModuleExistsFinalizer)
			return true
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("failed to set finalizer to the '%s' module source: %v", source.Name, err)
	}

	return nil
}

func (r *reconciler) deleteModuleSource(ctx context.Context, source *v1alpha1.ModuleSource) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(source, moduleSourceReleaseExistsFinalizer) {
		if source.GetAnnotations()[moduleSourceAnnotationForceDelete] != "true" {
			// list deployed ModuleReleases associated with the ModuleSource
			releases := new(v1alpha1.ModuleReleaseList)
			if err := r.client.List(ctx, releases, client.MatchingLabels{"source": source.Name, "status": "deployed"}); err != nil {
				return ctrl.Result{Requeue: true}, nil
			}

			// prevent deletion if there are deployed releases
			if len(releases.Items) > 0 {
				err := utils.Update[*v1alpha1.ModuleSource](ctx, r.client, source, true, func(obj *v1alpha1.ModuleSource) bool {
					source.Status.Message = "The ModuleSource contains at least 1 Deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue"
					return true
				})
				if err != nil {
					r.log.Errorf("failed to update the '%s' module source status: %v", source.Name, err)
					return ctrl.Result{Requeue: true}, nil
				}
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}

			controllerutil.RemoveFinalizer(source, moduleSourceReleaseExistsFinalizer)
			if err := r.client.Update(ctx, source); err != nil {
				r.log.Errorf("failed to update the '%s' module source: %v", source.Name, err)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	}

	if controllerutil.ContainsFinalizer(source, moduleSourceModuleExistsFinalizer) {
		for _, module := range source.Status.AvailableModules {
			if err := r.cleanSourceInModule(ctx, source.Name, module.Name); err != nil {
				r.log.Errorf("failed to clean source in the '%s' module, during deleting the '%s' module source", module.Name, source.Name)
				return ctrl.Result{Requeue: true}, nil
			}
		}

		controllerutil.RemoveFinalizer(source, moduleSourceModuleExistsFinalizer)
		if err := r.client.Update(ctx, source); err != nil {
			r.log.Errorf("failed to update the '%s' module source: %v", source.Name, err)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

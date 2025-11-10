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
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	controllerName = "d8-module-source-controller"

	defaultScanInterval = 3 * time.Minute

	maxConcurrentReconciles = 3
	cacheSyncTimeout        = 3 * time.Minute

	maxModulesLimit = 1500
	serviceName     = "module-source-controller"
)

var ErrSettingsNotChanged = errors.New("settings not changed")

func RegisterController(
	runtimeManager manager.Manager,
	mm moduleManager,
	edition *d8edition.Edition,
	dc dependency.Container,
	metricStorage metricsstorage.Storage,
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:                 new(sync.WaitGroup),
		client:               runtimeManager.GetClient(),
		dc:                   dc,
		logger:               logger,
		moduleManager:        mm,
		edition:              edition,
		metricStorage:        metricStorage,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		embeddedPolicy:       embeddedPolicy,
	}

	r.init.Add(1)

	// add preflight to set the cluster UUID
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	sourceController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModuleSource{}).
		Watches(&v1alpha1.Module{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: obj.(*v1alpha1.Module).Properties.Source}}}
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
				// not found or !true -> true
				if !oldMod.HasCondition(v1alpha1.ModuleConditionEnabledByModuleConfig) ||
					!oldMod.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) &&
						newMod.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
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
		Complete(sourceController); err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	return nil
}

type reconciler struct {
	init   *sync.WaitGroup
	client client.Client
	dc     dependency.Container
	logger *log.Logger

	metricStorage metricsstorage.Storage

	embeddedPolicy       *helpers.ModuleUpdatePolicySpecContainer
	moduleManager        moduleManager
	edition              *d8edition.Edition
	downloadedModulesDir string
	clusterUUID          string
}

type moduleManager interface {
	AreModulesInited() bool
}

func (r *reconciler) preflight(ctx context.Context) error {
	defer r.init.Done()

	// wait until module manager init
	r.logger.Debug("wait until module manager is inited")
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return r.moduleManager.AreModulesInited(), nil
	}); err != nil {
		return fmt.Errorf("init module manager: %w", err)
	}

	r.clusterUUID = utils.GetClusterUUID(ctx, r.client)

	r.logger.Debug("controller is ready")

	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Debug("reconciling module source", slog.String("name", req.Name))
	moduleSource := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, req.NamespacedName, moduleSource); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module source not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}
		r.logger.Error("failed to get module source", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !moduleSource.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting module source", slog.String("name", req.Name))
		return r.deleteModuleSource(ctx, moduleSource)
	}

	// handle create/update events
	return r.handleModuleSource(ctx, moduleSource)
}

func (r *reconciler) handleModuleSource(ctx context.Context, source *v1alpha1.ModuleSource) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handleModuleSource")
	defer span.End()

	span.SetAttributes(attribute.String("source", source.Name))

	// generate options for connecting to the registry
	opts := utils.GenerateRegistryOptionsFromModuleSource(source, r.clusterUUID, r.logger)

	// create a registry client
	registryClient, err := r.dc.GetRegistryClient(source.Spec.Registry.Repo, opts...)
	if err != nil {
		r.logger.Error("failed to get registry client for the module source", slog.String("source_name", source.Name), log.Err(err))
		if uerr := r.updateModuleSourceStatusMessage(ctx, source, err.Error()); uerr != nil {
			r.logger.Error("failed to update source status message", slog.String("source_name", source.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}
		// error can occur on wrong auth only, we don't want to requeue the source until auth is fixed
		return ctrl.Result{}, nil
	}

	// sync registry settings
	if err = r.syncRegistrySettings(ctx, source); err != nil && !errors.Is(err, ErrSettingsNotChanged) {
		r.logger.Error("failed to sync registry settings for module source", slog.String("source_name", source.Name), log.Err(err))
		if uerr := r.updateModuleSourceStatusMessage(ctx, source, err.Error()); uerr != nil {
			r.logger.Error("failed to update source status message", slog.String("source_name", source.Name), log.Err(uerr))
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{}, err
	}
	if err == nil {
		// new registry settings checksum should be applied to module source
		if err = r.client.Update(ctx, source); err != nil {
			r.logger.Error("failed to update module source status", slog.String("source_name", source.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}
		// requeue module source after modifying annotation
		r.logger.Debug("module source will be requeued", slog.String("source_name", source.Name))
		return ctrl.Result{Requeue: true}, nil
	}

	span.AddEvent("fetch tags from the registry")

	// list available modules(tags) from the registry
	r.logger.Debug("fetch modules from the module source", slog.String("source_name", source.Name))
	pulledModules, err := registryClient.ListTags(ctx)
	if err != nil {
		r.logger.Error("failed to list tags for the module source", slog.String("source_name", source.Name), log.Err(err))
		if uerr := r.updateModuleSourceStatusMessage(ctx, source, err.Error()); uerr != nil {
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{RequeueAfter: defaultScanInterval}, nil
	}

	span.AddEvent("successfully fetched the tags for the registry",
		trace.WithAttributes(attribute.Int("count", len(pulledModules))))

	// limit pulled module
	if len(pulledModules) > maxModulesLimit {
		pulledModules = pulledModules[:maxModulesLimit]
	}

	// remove the source from available sources in deleted modules
	namesSet := make(map[string]bool)
	for _, pulledModuleName := range pulledModules {
		namesSet[pulledModuleName] = true
	}
	for _, availableModule := range source.Status.AvailableModules {
		if !namesSet[availableModule.Name] {
			if err = r.cleanSourceInModule(ctx, source.Name, availableModule.Name); err != nil {
				r.logger.Error("failed to clean the module from the module source", slog.String("name", availableModule.Name), log.Err(err))
				return ctrl.Result{}, err
			}
		}
	}

	if err = r.processModules(ctx, source, opts, pulledModules); err != nil {
		r.logger.Error("failed to process modules for the module source", slog.String("source_name", source.Name), log.Err(err))

		return ctrl.Result{}, err
	}

	r.logger.Debug("module source reconciled", slog.String("source_name", source.Name))

	// everything is ok, check source on the other iterations
	return ctrl.Result{RequeueAfter: defaultScanInterval}, nil
}

func (r *reconciler) processModules(ctx context.Context, source *v1alpha1.ModuleSource, opts []cr.Option, pulledModules []string) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "processModules")
	defer span.End()

	md := downloader.NewModuleDownloader(r.dc, r.downloadedModulesDir, source, r.logger.Named("downloader"), opts)
	sort.Strings(pulledModules)

	availableModules := make([]v1alpha1.AvailableModule, 0)
	var errorsExist bool

	for _, moduleName := range pulledModules {
		logger := r.logger.With(slog.String("module_name", moduleName))

		if moduleName == "modules" || len(moduleName) > 64 {
			logger.Warn("the module has a forbidden name, skip it")
			continue
		}

		if errs := validation.IsDNS1123Subdomain(moduleName); len(errs) > 0 {
			logger.Warn("the module has invalid name: must comply with RFC 1123 subdomain format, skip it")
			continue
		}

		availableModule := v1alpha1.AvailableModule{Name: moduleName}
		for _, available := range source.Status.AvailableModules {
			if available.Name == moduleName {
				availableModule = available
				break
			}
		}

		// clear process error
		availableModule.Error = ""

		// TODO: remove this emptify after 1.75
		// nolint: staticcheck
		availableModule.PullError = ""

		// clear overridden
		availableModule.Overridden = false

		// get update policy
		policy, err := utils.UpdatePolicy(ctx, r.client, r.embeddedPolicy, moduleName)
		if err != nil {
			logger.Warn("failed to get update policy for module, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		availableModule.Policy = policy.Name

		logger = logger.With(slog.String("release channel", policy.Spec.ReleaseChannel))

		// create or update module
		module, err := r.ensureModule(ctx, source.Name, moduleName, policy.Spec.ReleaseChannel)
		if err != nil {
			// skip modules that require resync
			if errors.Is(err, ErrRequireResync) {
				availableModule.Version = "unknown"
				availableModules = append(availableModules, availableModule)
				continue
			}

			logger.Warn("failed to ensure module, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		logger = logger.With(slog.String("source_name", source.Name))

		exists, err := utils.ModulePullOverrideExists(ctx, r.client, moduleName)
		if err != nil {
			logger.Warn("failed to get module pull override, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		// skip overridden module
		if exists {
			availableModule.Overridden = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		metricModuleGroup := metrics.D8ModuleUpdatingGroup + "_" + strcase.ToSnake(moduleName) + "_" + strcase.ToSnake(source.GetName())
		r.metricStorage.Grouped().ExpireGroupMetrics(metricModuleGroup)

		logger.Debug("download module meta from release channel")

		meta, err := md.DownloadMetadataFromReleaseChannel(ctx, moduleName, policy.Spec.ReleaseChannel)
		if err != nil {
			if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) && module.Properties.Source == source.Name {
				r.logger.Warn("failed to download module", slog.String("name", moduleName), log.Err(err))
				availableModule.Error = err.Error()
				errorsExist = true

				metricLabels := map[string]string{
					"module":   moduleName,
					"version":  availableModule.Version,
					"registry": source.Spec.Registry.Repo,
				}

				r.metricStorage.Grouped().GaugeSet(metricModuleGroup, metrics.D8ModuleUpdatingModuleIsNotValid, 1, metricLabels)
			}

			availableModule.Version = "unknown"
			availableModules = append(availableModules, availableModule)

			continue
		}

		// check if release exists
		exists, err = r.releaseExists(ctx, source.Name, moduleName, availableModule.Checksum)
		if err != nil {
			logger.Error("failed to check if module has a release, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		if r.needToEnsureRelease(source, module, availableModule, meta, exists) {
			logger.Debug("ensure release")

			err = ctrlutils.UpdateStatusWithRetry(ctx, r.client, module, func() error {
				if module.Status.Phase == v1alpha1.ModulePhaseAvailable || module.Status.Phase == v1alpha1.ModulePhaseConflict {
					module.Status.Phase = v1alpha1.ModulePhaseDownloading
					module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonDownloading, v1alpha1.ModuleMessageDownloading)
				}

				return nil
			})
			if err != nil {
				logger.Error("failed to update module status before fetch, skipping", slog.String("name", moduleName), log.Err(err))
				availableModule.Error = err.Error()
				availableModule.Version = "unknown"
				errorsExist = true
				availableModules = append(availableModules, availableModule)
				continue
			}

			err = ctrlutils.UpdateWithRetry(ctx, r.client, module, func() error {
				module.Properties.Source = source.Name

				return nil
			})
			if err != nil {
				logger.Error("failed to update module before fetch, skipping", slog.String("name", moduleName), log.Err(err))
				availableModule.Error = err.Error()
				availableModule.Version = "unknown"
				errorsExist = true
				availableModules = append(availableModules, availableModule)
				continue
			}

			err = r.fetchModuleReleases(ctx, md, moduleName, meta, source, policy.Name, policy.Spec.ReleaseChannel, metricModuleGroup, opts)
			if err != nil {
				logger.Error("fetch module releases", log.Err(err))
				availableModule.Error = err.Error()
				// wipe checksum to trigger meta downloading
				meta.Checksum = ""
				errorsExist = true
			}
		}

		availableModule.Checksum = meta.Checksum
		availableModule.Version = meta.ModuleVersion

		availableModules = append(availableModules, availableModule)
	}

	// update source status
	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, source, func() error {
		source.Status.Phase = v1alpha1.ModuleSourcePhaseActive
		source.Status.SyncTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		source.Status.AvailableModules = availableModules
		source.Status.ModulesCount = len(availableModules)
		source.Status.Message = ""

		if errorsExist {
			source.Status.Message = v1alpha1.ModuleSourceMessageErrors
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module source status: %w", source.Name, err)
	}

	// set finalizer
	err = utils.Update(ctx, r.client, source, func(source *v1alpha1.ModuleSource) bool {
		if !controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists) {
			controllerutil.AddFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists)

			return true
		}

		return false
	})
	if err != nil {
		return fmt.Errorf("set finalizer to the '%s' module source: %w", source.Name, err)
	}

	return nil
}

func (r *reconciler) deleteModuleSource(ctx context.Context, source *v1alpha1.ModuleSource) (ctrl.Result, error) {
	if source.Status.Phase != v1alpha1.ModuleSourcePhaseTerminating {
		source.Status.Phase = v1alpha1.ModuleSourcePhaseTerminating
		if err := r.client.Status().Update(ctx, source); err != nil {
			r.logger.Warn("failed to set terminating to the source", slog.String("module_source", source.GetName()), log.Err(err))

			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists) {
		if source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationForceDelete] != "true" {
			// list deployed ModuleReleases associated with the ModuleSource
			releases := new(v1alpha1.ModuleReleaseList)
			if err := r.client.List(ctx, releases, client.MatchingLabels{"source": source.Name, "status": "deployed"}); err != nil {
				r.logger.Warn("failed to list releases", slog.String("module_source", source.GetName()), log.Err(err))

				return ctrl.Result{}, fmt.Errorf("list: %w", err)
			}

			// prevent deletion if there are deployed releases
			if len(releases.Items) > 0 {
				err := utils.UpdateStatus(ctx, r.client, source, func(source *v1alpha1.ModuleSource) bool {
					source.Status.Message = "The source contains at least 1 deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue"
					return true
				})
				if err != nil {
					r.logger.Error("failed to update module source status", slog.String("name", source.Name), log.Err(err))
					return ctrl.Result{}, fmt.Errorf("update status: %w", err)
				}

				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
		}

		controllerutil.RemoveFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists)
		if err := r.client.Update(ctx, source); err != nil {
			r.logger.Error("failed to update module source", slog.String("name", source.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists) {
		if source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationForceDelete] != "true" {
			for _, module := range source.Status.AvailableModules {
				if err := r.cleanSourceInModule(ctx, source.Name, module.Name); err != nil {
					r.logger.Error("failed to clean source in module during deletion of module source", slog.String("name", module.Name), slog.String("source_name", source.Name), log.Err(err))
					return ctrl.Result{}, err
				}
			}
		}

		controllerutil.RemoveFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists)
		if err := r.client.Update(ctx, source); err != nil {
			r.logger.Error("failed to update module source", slog.String("source_name", source.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

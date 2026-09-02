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
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
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
	deckhouseSettings *helpers.DeckhouseSettingsContainer,
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
		downloadedModulesDir: app.DownloadedModulesDir(),
		embeddedPolicy:       embeddedPolicy,
		deckhouseSettings:    deckhouseSettings,
	}

	r.init.Add(1)

	// add preflight to set the cluster UUID
	if err := runtimeManager.Add(manager.RunnableFunc(r.preflight)); err != nil {
		return fmt.Errorf("add preflight: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(runtimeManager).
		Named(controllerName).
		For(&v1alpha1.ModuleSource{}).
		// A module config enables a module, picks its source or its policy: the sources offering
		// the module rescan. A config created before the first scan of its source is caught by
		// the regular rescan, since the source lists the module only after that scan.
		Watches(&v1alpha1.ModuleConfig{}, handler.EnqueueRequestsFromMapFunc(r.sourcesOfConfig), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldConfig := updateEvent.ObjectOld.(*v1alpha1.ModuleConfig)
				newConfig := updateEvent.ObjectNew.(*v1alpha1.ModuleConfig)

				return oldConfig.Spec.Source != newConfig.Spec.Source ||
					oldConfig.Spec.UpdatePolicy != newConfig.Spec.UpdatePolicy ||
					oldConfig.IsEnabled() != newConfig.IsEnabled()
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
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
	init   *sync.WaitGroup
	client client.Client
	dc     dependency.Container
	logger *log.Logger

	metricStorage metricsstorage.Storage

	embeddedPolicy       *helpers.ModuleUpdatePolicySpecContainer
	deckhouseSettings    *helpers.DeckhouseSettingsContainer
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
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
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

	scanInterval := defaultScanInterval
	if interval := source.Spec.ScanInterval; interval != nil && interval.Duration > defaultScanInterval {
		scanInterval = interval.Duration
	}

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
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
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

		return ctrl.Result{RequeueAfter: scanInterval}, nil
	}

	span.AddEvent("successfully fetched the tags for the registry",
		trace.WithAttributes(attribute.Int("count", len(pulledModules))))

	// limit pulled module
	if len(pulledModules) > maxModulesLimit {
		pulledModules = pulledModules[:maxModulesLimit]
	}

	// a module the source stopped offering loses this source: the object of a module nothing
	// installed and no other source offers goes
	pulled := make(map[string]struct{}, len(pulledModules))
	for _, name := range pulledModules {
		pulled[name] = struct{}{}
	}

	for _, available := range source.Status.AvailableModules {
		if _, ok := pulled[available.Name]; ok {
			continue
		}

		if err := r.cleanCatalogModule(ctx, source, available.Name); err != nil {
			r.logger.Error("failed to clean the module the source stopped offering", slog.String("source_name", source.Name), slog.String("module_name", available.Name), log.Err(err))

			return ctrl.Result{}, err
		}
	}

	if err = r.processModules(ctx, source, opts, pulledModules); err != nil {
		r.logger.Error("failed to process modules for the module source", slog.String("source_name", source.Name), log.Err(err))

		return ctrl.Result{}, err
	}

	r.logger.Debug("module source reconciled", slog.String("source_name", source.Name), slog.String("interval", scanInterval.String()))

	// everything is ok, check source on the other iterations
	return ctrl.Result{RequeueAfter: scanInterval}, nil
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

		policy, err := utils.GetUpdatePolicyByModule(ctx, r.client, r.embeddedPolicy, moduleName)
		if err != nil {
			logger.Warn("failed to get update policy for module, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		availableModule.Policy = policy.Name

		logger = logger.With(slog.String("release_channel", policy.Spec.ReleaseChannel), slog.String("source_name", source.Name))

		module, err := r.getModule(ctx, moduleName)
		if err != nil {
			logger.Warn("failed to get module, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		config, err := r.moduleConfig(ctx, moduleName)
		if err != nil {
			logger.Warn("failed to get module config, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		offering, err := r.offeringSources(ctx, source, moduleName)
		if err != nil {
			logger.Error("failed to list module sources, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		// a module nothing installed has an object too: the repository it would come from, the
		// channel of its policy and the offered or the conflict state
		if module == nil || !module.IsInstalled() {
			module, err = r.ensureCatalogModule(ctx, moduleName, policy.Spec.ReleaseChannel, config, offering)
			if err != nil {
				logger.Warn("failed to ensure the module, skipping", slog.String("name", moduleName), log.Err(err))
				availableModule.Error = err.Error()
				availableModule.Version = "unknown"
				errorsExist = true
				availableModules = append(availableModules, availableModule)
				continue
			}
		}

		// a module installed from this source follows the channel of its policy
		if module.IsInstalled() && !module.IsEmbedded() && activeSource(module, config) == source.Name {
			if err := r.ensureReleaseChannel(ctx, module, policy.Spec.ReleaseChannel); err != nil {
				logger.Warn("failed to set the module release channel, skipping", slog.String("name", moduleName), log.Err(err))
				availableModule.Error = err.Error()
				availableModule.Version = "unknown"
				errorsExist = true
				availableModules = append(availableModules, availableModule)
				continue
			}
		}

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
			// only the source the enabled module comes from reports the failure
			if config != nil && config.IsEnabled() && activeSource(module, config) == source.Name {
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

		// Resolve which source an embedded module should be pre-staged from while its
		// embedded copy is still shipped. The module stays embedded, so the choice is
		// driven by the operator's ModuleConfig .spec.source, the only real source, or
		// the canonical "deckhouse" source - see resolveEmbeddedTargetSource. Being
		// offered by several sources is not automatically a conflict: "deckhouse" + a
		// mirror like "deckhouse-upstream-ee" resolves cleanly. A real conflict only
		// arises from a stale .spec.source or from several non-default sources with no
		// selection.
		var embeddedTargetSource string
		if module != nil && module.IsEmbedded() {
			chosenSource := pkgsync.ConfiguredSource(config)

			var conflict bool
			embeddedTargetSource, conflict = resolveEmbeddedTargetSource(chosenSource, offering)
			if conflict {
				// Skip pre-staging with a diagnostic warning; do NOT raise a user-facing
				// ModuleAtConflict alert. This branch is embedded-only, and the embedded
				// copy keeps serving the module regardless of which source a future
				// release would come from, so this is a deferred pre-staging decision, not
				// a runtime conflict. Firing d8_module_at_conflict here would register the
				// same module-labelled series under several metric groups, making the
				// whole self /metrics page fail to collect (up=0 -> D8DeckhouseSelfTargetDown).
				if chosenSource != "" {
					logger.Warn("embedded module's configured source does not offer the module, cannot pre-stage a release until the ModuleConfig .spec.source is fixed",
						slog.String("name", moduleName),
						slog.String("configured_source", chosenSource),
						slog.Any("available_sources", offering))
				} else {
					logger.Warn("embedded module is offered by several non-default sources and none is selected via ModuleConfig, cannot pre-stage a release until the conflict is resolved",
						slog.String("name", moduleName),
						slog.Any("available_sources", offering))
				}

				availableModule.Checksum = meta.Checksum
				availableModule.Version = meta.ModuleVersion
				availableModules = append(availableModules, availableModule)
				continue
			}
		}

		metadata, err := utils.ModuleMetadata(ctx, r.client, module)
		if err != nil {
			logger.Error("failed to get the module metadata, skipping", slog.String("name", moduleName), log.Err(err))
			availableModule.Error = err.Error()
			availableModule.Version = "unknown"
			errorsExist = true
			availableModules = append(availableModules, availableModule)
			continue
		}

		if !r.releaseEnsureAllowed(source, moduleName, module, config, metadata, meta, offering, embeddedTargetSource) {
			availableModule.Checksum = meta.Checksum
			availableModule.Version = meta.ModuleVersion
			availableModules = append(availableModules, availableModule)
			continue
		}

		// checksum changed or the target release is missing - ensure as usual.
		ensure := availableModule.Checksum != meta.Checksum || !exists
		if !ensure {
			// The target release already exists and its checksum has not changed, but
			// the step-by-step chain of ModuleReleases from the deployed release up to
			// the target may still be incomplete: intermediate minor versions can be
			// mirrored into the registry after the target release was first created, and
			// the fetch that created the target could not see them then. The checksum
			// guard never reopens on its own, so re-run the fetch whenever the chain has
			// a gap - ensureReleases is idempotent and creates only the missing releases.
			complete, err := r.releaseChainToTargetComplete(ctx, moduleName, meta.ModuleVersion)
			if err != nil {
				logger.Error("failed to check release chain to target, skipping", slog.String("name", moduleName), log.Err(err))
				availableModule.Error = err.Error()
				// meta was fetched successfully above, so the channel version is known -
				// keep it instead of wiping it to "unknown" on this transient check error
				availableModule.Checksum = meta.Checksum
				availableModule.Version = meta.ModuleVersion
				errorsExist = true
				availableModules = append(availableModules, availableModule)
				continue
			}
			ensure = !complete
		}
		if ensure {
			logger.Debug("ensure release")

			// the first release of an offered module moves it out of the catalog
			if module != nil {
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

	// the modules the source installed die with their releases: the release controller
	// uninstalls them, and the package sync drops their objects at the next start. The modules
	// the source merely offered lose it here: the object of a module no other source offers
	// goes. A forced deletion does not wait for a failing cleanup.
	if controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists) {
		forced := source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationForceDelete] == "true"

		for _, available := range source.Status.AvailableModules {
			err := r.cleanCatalogModule(ctx, source, available.Name)
			if err == nil {
				continue
			}

			if !forced {
				r.logger.Error("failed to clean the module of the deleted source", slog.String("source_name", source.Name), slog.String("module_name", available.Name), log.Err(err))

				return ctrl.Result{}, fmt.Errorf("clean the '%s' module: %w", available.Name, err)
			}

			r.logger.Warn("failed to clean the module of the force deleted source", slog.String("source_name", source.Name), slog.String("module_name", available.Name), log.Err(err))
		}

		controllerutil.RemoveFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists)
		if err := r.client.Update(ctx, source); err != nil {
			r.logger.Error("failed to update module source", slog.String("source_name", source.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("update: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// sourcesOfConfig maps a module config onto the sources its module may come from: the one
// the config names and every source offering the module.
func (r *reconciler) sourcesOfConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	config := obj.(*v1alpha1.ModuleConfig)

	sources := new(v1alpha1.ModuleSourceList)
	if err := r.client.List(ctx, sources); err != nil {
		r.logger.Warn("failed to list module sources", slog.String("module", config.Name), log.Err(err))

		return nil
	}

	names := sources.Offering(config.Name)
	if chosen := pkgsync.ConfiguredSource(config); chosen != "" && !slices.Contains(names, chosen) {
		names = append(names, chosen)
	}

	requests := make([]reconcile.Request, 0, len(names))
	for _, name := range names {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: name}})
	}

	return requests
}

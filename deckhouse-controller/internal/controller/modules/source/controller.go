// Copyright 2026 Flant JSC
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
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/modules/source/releases"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-source-controller"

	// maxConcurrentReconciles is safe above one: a source writes only its own status and the
	// releases of the modules it serves, and no release belongs to two sources.
	maxConcurrentReconciles = 3
	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute

	// defaultScanInterval is how long to wait before rescanning a source that carries no
	// interval of its own.
	defaultScanInterval = 3 * time.Minute
	// deployedReleaseRequeue paces the retry while a deleted source still holds deployed releases.
	deployedReleaseRequeue = 5 * time.Second

	// maxModulesLimit caps how many tags one scan turns into modules, so a mis-pointed
	// repository cannot spend the whole reconcile on thousands of names.
	maxModulesLimit = 1500
	// maxModuleNameLength is the longest module name a source may offer.
	maxModuleNameLength = 64
	// reservedModuleName is a registry path segment, never a module.
	reservedModuleName = "modules"

	// unknownVersion marks an available module whose version this pass could not establish.
	unknownVersion = "unknown"

	// messageDeployedReleasesExist tells the operator why a deleted source is not going away.
	messageDeployedReleasesExist = "The source contains at least 1 deployed release and cannot be deleted. " +
		"Please delete target ModuleReleases manually to continue"
)

// errSettingsNotChanged reports that the source's registry spec still matches the checksum
// recorded on it, so the releases it owns need no re-stamping.
var errSettingsNotChanged = errors.New("settings not changed")

// RegisterController registers the ModuleSource controller with the manager. A source drives the
// auto-update cycle and nothing else: it polls the registry, resolves the update policy and keeps
// the ModuleRelease chain complete. It creates no Module of either API version, places nothing on
// the filesystem, and owns no object in the package system.
func RegisterController(
	runtime ctrlmanager.Manager,
	manager editionResolver,
	ms metricsstorage.Storage,
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer,
	settings *helpers.DeckhouseSettingsContainer,
	dc dependency.Container,
	logger *log.Logger,
) error {
	r := &reconciler{
		client:         runtime.GetClient(),
		registry:       registry.NewService(dc, logger),
		dc:             dc,
		manager:        manager,
		metricStorage:  ms,
		embeddedPolicy: embeddedPolicy,
		settings:       settings,
		logger:         logger.Named(controllerName),
	}

	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1alpha1.ModuleSource{}).
		// The config carries the operator's intent, which is what decides whether a module is
		// worth fetching releases for. Without this watch a newly enabled module waits out the
		// scan interval, because the registry checksum has not moved.
		Watches(&v1alpha1.ModuleConfig{}, handler.EnqueueRequestsFromMapFunc(r.enqueueAllSources)).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      new(false),
		}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// enqueueAllSources maps a module config onto every source. A config names a module, never the
// source offering it, so each source has to re-evaluate the modules it serves.
func (r *reconciler) enqueueAllSources(ctx context.Context, _ client.Object) []reconcile.Request {
	sources := new(v1alpha1.ModuleSourceList)
	if err := r.client.List(ctx, sources); err != nil {
		r.logger.Error("failed to list the module sources", log.Err(err))
		return nil
	}

	requests := make([]reconcile.Request, 0, len(sources.Items))
	for _, source := range sources.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: source.Name}})
	}

	return requests
}

// reconciler keeps the ModuleReleases offered by a source in step with its registry.
type reconciler struct {
	client   client.Client
	registry releases.TagLister
	dc       dependency.Container
	manager  editionResolver

	metricStorage metricsstorage.Storage

	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	settings       *helpers.DeckhouseSettingsContainer

	logger *log.Logger
}

// editionResolver reports the running edition, used to resolve a module's accessibility.
type editionResolver interface {
	Edition() *edition.Edition
}

// Reconcile dispatches the source to the delete or the create/update handler.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Debug("reconcile module source", slog.String("name", req.Name))

	source := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, req.NamespacedName, source); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module source not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get the module source", slog.String("name", req.Name), log.Err(err))

		return ctrl.Result{}, fmt.Errorf("get module source '%s': %w", req.Name, err)
	}

	// handle delete event
	if !source.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, source)
	}

	// handle create/update events
	return r.handleCreateOrUpdate(ctx, source)
}

// handleCreateOrUpdate rescans the source's registry and brings the ModuleRelease chain of every
// module it offers up to the version the module's update policy points at.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, source *v1alpha1.ModuleSource) (ctrl.Result, error) {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "handleCreateOrUpdate")
	defer span.End()

	span.SetAttributes(attribute.String("source", source.Name))

	logger := r.logger.With(slog.String("source_name", source.Name))

	logger.Debug("handle module source")
	defer logger.Debug("handle module source complete")

	// A changed registry spec has to reach the deployed releases before anything is fetched
	// against it, so the sync runs first and requeues rather than continuing on stale settings.
	err := r.syncRegistrySettings(ctx, source)
	if err != nil && !errors.Is(err, errSettingsNotChanged) {
		logger.Error("failed to sync the registry settings", log.Err(err))

		if serr := r.setStatusMessage(ctx, source, err.Error()); serr != nil {
			return ctrl.Result{}, serr
		}

		return ctrl.Result{}, err
	}

	if err == nil {
		// the new checksum lives on the in-memory source until this write lands
		if err = r.client.Update(ctx, source); err != nil {
			logger.Error("failed to update the module source", log.Err(err))
			return ctrl.Result{}, fmt.Errorf("update module source '%s': %w", source.Name, err)
		}

		logger.Debug("registry settings changed, requeue the module source")

		return ctrl.Result{Requeue: true}, nil
	}

	logger.Debug("fetch the modules offered by the module source")

	remote := registry.BuildRemote(source)

	pulled, err := r.registry.ListTags(ctx, remote)
	if err != nil {
		logger.Error("failed to list the tags of the module source", log.Err(err))

		if serr := r.setStatusMessage(ctx, source, err.Error()); serr != nil {
			return ctrl.Result{}, serr
		}

		// bad credentials and an unreachable registry look alike here; both are retried
		return ctrl.Result{RequeueAfter: scanInterval(source)}, nil
	}

	if len(pulled) > maxModulesLimit {
		logger.Warn("the module source offers more modules than the limit, the rest is ignored",
			slog.Int("count", len(pulled)), slog.Int("limit", maxModulesLimit))

		pulled = pulled[:maxModulesLimit]
	}

	if err = r.processModules(ctx, source, remote, pulled); err != nil {
		logger.Error("failed to process the modules of the module source", log.Err(err))

		return ctrl.Result{}, err
	}

	logger.Debug("module source reconciled", slog.String("interval", scanInterval(source).String()))

	return ctrl.Result{RequeueAfter: scanInterval(source)}, nil
}

// handleDelete releases the source once nothing depends on it. A deployed release blocks the
// deletion, because removing the source under it strands the module on a version nothing can
// update; the force-delete annotation is the operator's way to override that.
func (r *reconciler) handleDelete(ctx context.Context, source *v1alpha1.ModuleSource) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("source_name", source.Name))

	logger.Debug("handle delete module source")
	defer logger.Debug("handle delete module source complete")

	if source.Status.Phase != v1alpha1.ModuleSourcePhaseTerminating {
		patch := client.MergeFrom(source.DeepCopy())
		source.Status.Phase = v1alpha1.ModuleSourcePhaseTerminating

		if err := r.client.Status().Patch(ctx, source, patch); err != nil {
			logger.Warn("failed to set the module source terminating", log.Err(err))
			return ctrl.Result{}, fmt.Errorf("patch module source status '%s': %w", source.Name, err)
		}
	}

	forced := source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationForceDelete] == "true"

	if !forced && controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerReleaseExists) {
		deployed, err := r.deployedReleasesExist(ctx, source)
		if err != nil {
			logger.Warn("failed to list the releases of the module source", log.Err(err))
			return ctrl.Result{}, err
		}

		if deployed {
			if err = r.setStatusMessage(ctx, source, messageDeployedReleasesExist); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: deployedReleaseRequeue}, nil
		}
	}

	for _, finalizer := range []string{
		v1alpha1.ModuleSourceFinalizerReleaseExists,
		v1alpha1.ModuleSourceFinalizerModuleExists,
	} {
		if err := r.releaseFinalizer(ctx, source, finalizer); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// processModules walks the modules the source offers and ensures the releases each one needs,
// recording the outcome per module in the source's status. One module's failure never aborts the
// pass: scanning is the source's whole job, so a broken module must not hide the healthy ones.
func (r *reconciler) processModules(ctx context.Context, source *v1alpha1.ModuleSource, remote registry.Remote, pulledModules []string) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "processModules")
	defer span.End()

	// Both collaborators are built per pass, because the metadata loader is configured from this
	// source's own registry credentials.
	loader := newMetadataLoader(ctx, r.dc, r.client, source, r.logger)

	releaseService := releases.New(releases.Config{
		Client:        r.client,
		Registry:      r.registry,
		Loader:        loader,
		Clock:         r.dc.GetClock(),
		MetricStorage: r.metricStorage,
		Logger:        r.logger.Named("releases"),
	})

	sort.Strings(pulledModules)

	availableModules := make([]v1alpha1.AvailableModule, 0, len(pulledModules))
	var errorsExist bool

	for _, moduleName := range pulledModules {
		logger := r.logger.With(
			slog.String("source_name", source.Name),
			slog.String("module_name", moduleName))

		if !validModuleName(moduleName) {
			logger.Warn("the module name is not usable, skip it")
			continue
		}

		available := carryAvailableModule(source, moduleName)

		if err := r.processModule(ctx, source, loader, releaseService, remote, moduleName, &available, logger); err != nil {
			logger.Error("failed to process the module", log.Err(err))

			available.Error = err.Error()
			errorsExist = true
		}

		availableModules = append(availableModules, available)
	}

	if err := r.setScanStatus(ctx, source, availableModules, errorsExist); err != nil {
		return err
	}

	return r.ensureFinalizer(ctx, source)
}

// processModule ensures the releases one module needs, filling in available as it learns the
// module's policy, checksum and version. A returned error is recorded against the module and
// marks the scan as partially failed; it never aborts the remaining modules.
func (r *reconciler) processModule(
	ctx context.Context,
	source *v1alpha1.ModuleSource,
	loader *metadataLoader,
	releaseService *releases.Service,
	remote registry.Remote,
	moduleName string,
	available *v1alpha1.AvailableModule,
	logger *log.Logger,
) error {
	policy, err := utils.GetUpdatePolicyByModule(ctx, r.client, r.embeddedPolicy, moduleName)
	if err != nil {
		available.Version = unknownVersion
		return fmt.Errorf("get update policy: %w", err)
	}

	available.Policy = policy.Name
	logger = logger.With(slog.String("release_channel", policy.Spec.ReleaseChannel))

	// An override pins the module to a tag of its own, so releases must not move it.
	overridden, err := utils.ModulePullOverrideExists(ctx, r.client, moduleName)
	if err != nil {
		available.Version = unknownVersion
		return fmt.Errorf("get module pull override: %w", err)
	}

	if overridden {
		available.Overridden = true
		return nil
	}

	config, err := r.getModuleConfig(ctx, moduleName)
	if err != nil {
		available.Version = unknownVersion
		return err
	}

	repositories, err := r.availableRepositories(ctx, moduleName)
	if err != nil {
		available.Version = unknownVersion
		return err
	}

	metricGroup := moduleMetricGroup(source.Name, moduleName)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	logger.Debug("download the module metadata from the release channel")

	meta, err := loader.channelMetadata(ctx, moduleName, policy.Spec.ReleaseChannel)
	if err != nil {
		available.Version = unknownVersion

		// A module nobody asked this source for is allowed to be broken; alerting on it would
		// fire for every module in the registry the cluster does not use.
		if !configWantsModuleFromSource(config, source.Name) {
			logger.Debug("failed to download the module metadata, the module is not requested here", log.Err(err))
			return nil
		}

		r.metricStorage.Grouped().GaugeSet(metricGroup, metrics.D8ModuleUpdatingModuleIsNotValid, 1, map[string]string{
			metrics.LabelModule:   moduleName,
			metrics.LabelVersion:  available.Version,
			metrics.LabelRegistry: source.Spec.Registry.Repo,
		})

		return err
	}

	targetExists, err := releaseService.Exists(ctx, moduleName, available.Checksum)
	if err != nil {
		available.Version = unknownVersion
		return err
	}

	if !r.releaseEnsureAllowed(source, moduleName, meta, config, repositories) {
		available.Checksum, available.Version = meta.Checksum, meta.Version
		return nil
	}

	ensure, err := needEnsuring(ctx, releaseService, moduleName, meta, available.Checksum, targetExists)
	if err != nil {
		// the channel version is known even though the chain check failed, so keep it
		available.Checksum, available.Version = meta.Checksum, meta.Version
		return err
	}

	if ensure {
		logger.Debug("ensure the releases")

		err = releaseService.Ensure(ctx, releases.Request{
			Source:         source,
			Remote:         remote,
			ModuleName:     moduleName,
			Target:         meta,
			UpdatePolicy:   policy.Name,
			ReleaseChannel: policy.Spec.ReleaseChannel,
			MetricGroup:    metricGroup,
		})
		if err != nil {
			// wipe the checksum so the next pass re-downloads the metadata
			available.Checksum, available.Version = "", meta.Version

			return fmt.Errorf("ensure module releases: %w", err)
		}
	}

	available.Checksum, available.Version = meta.Checksum, meta.Version

	return nil
}

// needEnsuring reports whether the step-by-step fetch has to run for the module.
//
// A changed checksum or a missing target release is the ordinary trigger. The subtle case is
// neither: the target exists and the checksum has not moved, yet the chain of ModuleReleases from
// the deployed release up to the target can still have a gap, because intermediate minor versions
// may be mirrored into the registry after the target release was first created, and the fetch that
// created it could not see them then. The checksum guard never reopens on its own, so the chain is
// re-checked - Ensure is idempotent and creates only what is missing.
func needEnsuring(
	ctx context.Context,
	releaseService *releases.Service,
	moduleName string,
	meta *releases.Metadata,
	knownChecksum string,
	targetExists bool,
) (bool, error) {
	if knownChecksum != meta.Checksum || !targetExists {
		return true, nil
	}

	complete, err := releaseService.ChainComplete(ctx, moduleName, meta.Version)
	if err != nil {
		return false, fmt.Errorf("check the release chain to the target: %w", err)
	}

	return !complete, nil
}

// syncRegistrySettings stamps every deployed release the source owns when the source's registry
// spec changes, so each one re-renders its openapi values against the new registry. The checksum
// annotation is what makes this happen once per change instead of once per scan; it is written on
// the in-memory source and the caller persists it.
//
// It returns errSettingsNotChanged when the spec still matches the recorded checksum - the common
// case, and the signal for the caller to carry on with the scan.
func (r *reconciler) syncRegistrySettings(ctx context.Context, source *v1alpha1.ModuleSource) error {
	marshaled, err := json.Marshal(source.Spec.Registry)
	if err != nil {
		return fmt.Errorf("marshal the registry spec of module source '%s': %w", source.Name, err)
	}

	currentChecksum := fmt.Sprintf("%x", md5.Sum(marshaled))

	// a source that has never been scanned only records the checksum
	if len(source.ObjectMeta.Annotations) == 0 {
		source.ObjectMeta.Annotations = map[string]string{
			v1alpha1.ModuleSourceAnnotationRegistryChecksum: currentChecksum,
		}

		return nil
	}

	if source.ObjectMeta.Annotations[v1alpha1.ModuleSourceAnnotationRegistryChecksum] == currentChecksum {
		return errSettingsNotChanged
	}

	releases := new(v1alpha1.ModuleReleaseList)
	if err = r.client.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: source.Name}); err != nil {
		return fmt.Errorf("list module releases to update the registry settings: %w", err)
	}

	for i := range releases.Items {
		release := &releases.Items[i]
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !ownedBySource(release, source) {
			continue
		}

		patch := client.MergeFrom(release.DeepCopy())
		if release.ObjectMeta.Annotations == nil {
			release.ObjectMeta.Annotations = make(map[string]string)
		}

		release.ObjectMeta.Annotations[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged] = r.dc.GetClock().Now().UTC().Format(time.RFC3339)

		if err = r.client.Patch(ctx, release, patch); err != nil {
			return fmt.Errorf("annotate module release '%s' as registry-changed: %w", release.Name, err)
		}
	}

	source.ObjectMeta.Annotations[v1alpha1.ModuleSourceAnnotationRegistryChecksum] = currentChecksum

	return nil
}

// ownedBySource reports whether the release belongs to this exact source. The UID is part of the
// check so a recreated source of the same name does not adopt the previous one's releases.
func ownedBySource(release *v1alpha1.ModuleRelease, source *v1alpha1.ModuleSource) bool {
	for _, ref := range release.GetOwnerReferences() {
		if ref.UID == source.UID && ref.Name == source.Name && ref.Kind == v1alpha1.ModuleSourceGVK.Kind {
			return true
		}
	}

	return false
}

// deployedReleasesExist reports whether the source still owns a deployed release.
func (r *reconciler) deployedReleasesExist(ctx context.Context, source *v1alpha1.ModuleSource) (bool, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases, client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelSource: source.Name,
		v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
	}); err != nil {
		return false, fmt.Errorf("list module releases: %w", err)
	}

	return len(releases.Items) > 0, nil
}

// ensureFinalizer claims the finalizer that keeps the source around while it still owns modules.
func (r *reconciler) ensureFinalizer(ctx context.Context, source *v1alpha1.ModuleSource) error {
	if controllerutil.ContainsFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists) {
		return nil
	}

	patch := client.MergeFrom(source.DeepCopy())
	controllerutil.AddFinalizer(source, v1alpha1.ModuleSourceFinalizerModuleExists)

	if err := r.client.Patch(ctx, source, patch); err != nil {
		return fmt.Errorf("add finalizer to module source '%s': %w", source.Name, err)
	}

	return nil
}

// releaseFinalizer drops the named finalizer, tolerating one that is already gone.
func (r *reconciler) releaseFinalizer(ctx context.Context, source *v1alpha1.ModuleSource, finalizer string) error {
	if !controllerutil.ContainsFinalizer(source, finalizer) {
		return nil
	}

	patch := client.MergeFrom(source.DeepCopy())
	controllerutil.RemoveFinalizer(source, finalizer)

	if err := r.client.Patch(ctx, source, patch); err != nil {
		r.logger.Error("failed to remove the module source finalizer",
			slog.String("source_name", source.Name), slog.String("finalizer", finalizer), log.Err(err))

		return fmt.Errorf("patch module source '%s': %w", source.Name, err)
	}

	return nil
}

// setScanStatus records the outcome of a whole scan on the source.
func (r *reconciler) setScanStatus(ctx context.Context, source *v1alpha1.ModuleSource, available []v1alpha1.AvailableModule, errorsExist bool) error {
	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, source, func() error {
		source.Status.Phase = v1alpha1.ModuleSourcePhaseActive
		source.Status.SyncTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		source.Status.AvailableModules = available
		source.Status.ModulesCount = len(available)
		source.Status.Message = ""

		if errorsExist {
			source.Status.Message = v1alpha1.ModuleSourceMessageErrors
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update module source status '%s': %w", source.Name, err)
	}

	return nil
}

// setStatusMessage records why the source could not complete, leaving the scan results in place.
func (r *reconciler) setStatusMessage(ctx context.Context, source *v1alpha1.ModuleSource, message string) error {
	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, source, func() error {
		source.Status.Phase = v1alpha1.ModuleSourcePhaseActive
		source.Status.SyncTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		source.Status.Message = message

		return nil
	})
	if err != nil {
		r.logger.Error("failed to update the module source status", slog.String("source_name", source.Name), log.Err(err))

		return fmt.Errorf("update module source status '%s': %w", source.Name, err)
	}

	return nil
}

// scanInterval is how long to wait before rescanning the source. A source may only lengthen the
// default, so a misconfigured one cannot hammer the registry.
func scanInterval(source *v1alpha1.ModuleSource) time.Duration {
	if interval := source.Spec.ScanInterval; interval != nil && interval.Duration > defaultScanInterval {
		return interval.Duration
	}

	return defaultScanInterval
}

// validModuleName reports whether a registry tag may be treated as a module name.
func validModuleName(name string) bool {
	if name == reservedModuleName || len(name) > maxModuleNameLength {
		return false
	}

	return len(validation.IsDNS1123Subdomain(name)) == 0
}

// carryAvailableModule seeds this pass from what the last one recorded, clearing the fields the
// pass is about to decide again. The checksum is kept: it is what makes the fetch skippable.
func carryAvailableModule(source *v1alpha1.ModuleSource, moduleName string) v1alpha1.AvailableModule {
	available := v1alpha1.AvailableModule{Name: moduleName}
	for _, recorded := range source.Status.AvailableModules {
		if recorded.Name == moduleName {
			available = recorded
			break
		}
	}

	available.Error = ""
	available.Overridden = false

	return available
}

// moduleMetricGroup names the metric group holding one module's update metrics for one source.
func moduleMetricGroup(sourceName, moduleName string) string {
	return metrics.D8ModuleUpdatingGroup + "_" + strcase.ToSnake(moduleName) + "_" + strcase.ToSnake(sourceName)
}

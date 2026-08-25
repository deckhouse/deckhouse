// Copyright 2025 Flant JSC
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

package modulepackageversion

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-module-package-version-controller"

	// maxConcurrentReconciles is set to 1 to serialize status and label patches,
	// preventing conflicts on the same ModulePackageVersion resource.
	maxConcurrentReconciles = 1

	defaultRequeue = 15 * time.Second

	// defaultPathSegment is the registry sub-path for v2 module images.
	defaultPathSegment = "version"

	// legacyPathSegment is the registry sub-path for legacy module images
	// produced before the registry layout was unified under "version".
	legacyPathSegment = "release"
)

// RegisterController creates and registers the ModulePackageVersion controller.
// It watches ModulePackageVersion resources and reconciles draft versions by
// fetching metadata from the package registry and promoting them to non-draft status.
func RegisterController(sync *sync.WaitGroup, runtimeManager manager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		init:     sync,
		client:   runtimeManager.GetClient(),
		registry: registry.NewService(dc, logger),
		dc:       dc,
		logger:   logger.Named(controllerName),
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		Named(controllerName).
		For(&v1alpha1.ModulePackageVersion{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}

// reconciler promotes draft ModulePackageVersion resources by loading package
// metadata from the registry image and removing the draft label.
type reconciler struct {
	init     *sync.WaitGroup
	client   client.Client
	registry *registry.Service
	dc       dependency.Container
	logger   *log.Logger
}

// Reconcile handles a single ModulePackageVersion event. Draft resources are
// promoted by loading metadata; deleted resources have their finalizers removed
// once no Module uses them any more.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconcile resource")

	mpv := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, mpv); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("resource not found")

			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get resource", log.Err(err))

		return ctrl.Result{}, err
	}

	// handle delete event
	if !mpv.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, mpv)
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, mpv); err != nil {
		logger.Warn("failed to handle module package version", log.Err(err))

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate processes draft ModulePackageVersions through a promotion pipeline:
//  1. Fetch the package image from the registry using the repository config and
//     either the default "version" sub-path or the legacy "release" sub-path
//  2. Extract metadata (package.yaml or module.yaml, changelog.yaml, version.json)
//     from the image tar
//  3. Populate status.packageMetadata with the extracted information
//  4. Set the MetadataLoaded condition to True
//  5. Check if the package image exists in the registry and label accordingly
//  6. Add a finalizer and remove the draft label, completing promotion
//
// Non-draft resources are skipped since they have already been promoted.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) error {
	logger := r.logger.With(
		slog.String("name", mpv.Name),
		slog.String("package", mpv.Spec.PackageName),
		slog.String("version", mpv.Spec.PackageVersion),
		slog.String("repository", mpv.Spec.PackageRepositoryName))

	// Non-draft MPVs have already been promoted — nothing to do.
	if !mpv.IsDraft() {
		logger.Debug("package is not draft")

		return nil
	}

	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mpv.Spec.PackageRepositoryName}, repo); err != nil {
		original := mpv.DeepCopy()
		r.setMetadataLoadedConditionFalse(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonGetPackageRepoErr,
			fmt.Sprintf("failed to get repository '%s': %s", mpv.Spec.PackageRepositoryName, err.Error()),
		)

		if err := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status '%s': %w", mpv.Name, err)
		}

		return fmt.Errorf("get repository '%s': %w", mpv.Spec.PackageRepositoryName, err)
	}

	// Pick "version" by default; legacy images live under "release".
	segment := defaultPathSegment
	if mpv.IsLegacy() {
		segment = legacyPathSegment
	}

	remote := registry.BuildRemote(repo)
	version := mpv.Spec.PackageVersion
	versionPath := filepath.Join(mpv.Spec.PackageName, segment)

	logger.Debug("registry path",
		slog.String("path", versionPath),
		slog.String("segment", segment))

	img, err := r.registry.GetImageReader(ctx, remote, versionPath, version)
	if err != nil {
		original := mpv.DeepCopy()
		r.setMetadataLoadedConditionFalse(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonGetImageErr,
			fmt.Sprintf("get image: %s", err.Error()),
		)

		if err := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status '%s': %w", mpv.Name, err)
		}

		return fmt.Errorf("get image for '%s': %w", mpv.Name, err)
	}

	defer img.Close()

	meta, err := r.parseVersionMetadataByImage(ctx, img)
	if err != nil {
		original := mpv.DeepCopy()
		r.setMetadataLoadedConditionFalse(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonFetchErr,
			fmt.Sprintf("fetch package metadata: %s", err.Error()),
		)

		if err := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status '%s': %w", mpv.Name, err)
		}

		return fmt.Errorf("fetch package metadata '%s': %w", mpv.Name, err)
	}

	original := mpv.DeepCopy()
	setPackageMetadata(mpv, meta)
	r.setMetadataLoadedConditionTrue(mpv)

	if err = r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch status '%s': %w", mpv.Name, err)
	}

	original = mpv.DeepCopy()

	if mpv.Labels == nil {
		mpv.Labels = make(map[string]string)
	}

	// Check whether the package image exists in the registry and label accordingly.
	// The image may legitimately not exist (e.g. metadata-only bundle), so both outcomes are valid.
	if _, err = r.registry.GetImageDigest(ctx, remote, mpv.Spec.PackageName, version); err != nil {
		mpv.Labels[v1alpha1.ModulePackageVersionLabelExistInRegistry] = "false"
	} else {
		mpv.Labels[v1alpha1.ModulePackageVersionLabelExistInRegistry] = "true"
	}

	// Finalizer prevents deletion while Modules reference this version.
	if !controllerutil.ContainsFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer) {
		controllerutil.AddFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer)
	}

	delete(mpv.Labels, v1alpha1.ModulePackageVersionLabelDraft)

	if err = r.client.Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch '%s': %w", mpv.Name, err)
	}

	return nil
}

// handleDelete removes the finalizer from the ModulePackageVersion once no Module uses
// it any more. While it is still used, the reconcile is requeued every 15 seconds to
// wait for the Module to release it.
func (r *reconciler) handleDelete(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) (ctrl.Result, error) {
	logger := r.logger.With(
		slog.String("name", mpv.Name),
		slog.String("package", mpv.Spec.PackageName),
		slog.String("version", mpv.Spec.PackageVersion),
		slog.String("repository", mpv.Spec.PackageRepositoryName))

	if mpv.Status.Used {
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	if controllerutil.ContainsFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer) {
		logger.Debug("removing finalizer from module package version")

		original := mpv.DeepCopy()

		controllerutil.RemoveFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer)

		if err := r.client.Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
			logger.Warn("failed to remove finalizer", log.Err(err))

			return ctrl.Result{}, fmt.Errorf("remove finalizer from '%s': %w", mpv.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

// setMetadataLoadedConditionTrue sets the MetadataLoaded condition to True, clearing reason and message.
func (r *reconciler) setMetadataLoadedConditionTrue(mpv *v1alpha1.ModulePackageVersion) {
	mpv.Status.ObservedGeneration = mpv.Generation

	metautils.SetStatusCondition(&mpv.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded,
		Status:             metav1.ConditionTrue,
		Reason:             "Succeeded",
		ObservedGeneration: mpv.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})
}

// setMetadataLoadedConditionFalse sets the MetadataLoaded condition to False with a reason and message.
func (r *reconciler) setMetadataLoadedConditionFalse(mpv *v1alpha1.ModulePackageVersion, reason, message string) {
	mpv.Status.ObservedGeneration = mpv.Generation

	metautils.SetStatusCondition(&mpv.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: mpv.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})
}

// setPackageMetadata projects parsed module metadata onto the ModulePackageVersion
// status. Dispatches to the v2 package.yaml path or the legacy module.yaml path,
// then attaches the changelog if present. A nil meta is a no-op so callers may
// invoke unconditionally after a best-effort parse.
func setPackageMetadata(mpv *v1alpha1.ModulePackageVersion, meta *moduleMetadata) {
	if meta == nil {
		return
	}

	switch {
	case meta.packageDefinition != nil:
		mpv.Status.PackageMetadata = meta.packageDefinition.ConvertToStatusMetadata()
	case meta.moduleDefinition != nil:
		setFromModuleDefinition(mpv, meta.moduleDefinition)
	}

	mpv.Status.PackageMetadata.Changelog = &v1alpha1.PackageChangelog{
		Features: meta.changelog.Features,
		Fixes:    meta.changelog.Fixes,
	}
}

// setFromModuleDefinition projects a legacy module.yaml onto the MPV status.
// The legacy format carries flat deckhouse/kubernetes strings and a single
// parentModules map. Dependencies whose constraint ends in the "!optional"
// suffix are surfaced as conditional; the rest become mandatory.
func setFromModuleDefinition(mpv *v1alpha1.ModulePackageVersion, def *moduletypes.Definition) {
	mpv.Status.PackageMetadata = &v1alpha1.ModulePackageVersionStatusMetadata{
		Stage: def.Stage,
	}

	if def.Descriptions != nil {
		mpv.Status.PackageMetadata.Description = &v1alpha1.PackageDescription{
			Ru: def.Descriptions.Ru,
			En: def.Descriptions.En,
		}
	}

	if def.Requirements != nil {
		mpv.Status.PackageMetadata.Requirements = legacyRequirementsToCR(def.Requirements)
	}

	mpv.Status.PackageMetadata.Licensing = legacyAccessibilityToCR(def.Accessibility)

	mpv.Status.PackageMetadata.Weight = int32(def.Weight)
	mpv.Status.PackageMetadata.Critical = def.Critical
	mpv.Status.PackageMetadata.ExclusiveGroup = def.ExclusiveGroup
}

// legacyAccessibilityToCR projects legacy module.yaml accessibility onto package licensing.
func legacyAccessibilityToCR(access *moduletypes.ModuleAccessibility) *v1alpha1.PackageLicensing {
	if access == nil || len(access.Editions) == 0 {
		return nil
	}

	editions := make(map[string]v1alpha1.PackageEditionLicense, len(access.Editions))
	for name, e := range access.Editions {
		editions[name] = v1alpha1.PackageEditionLicense{
			Available:        e.Available,
			EnabledInBundles: slices.Clone(e.EnabledInBundles),
		}
	}

	return &v1alpha1.PackageLicensing{Editions: editions}
}

// legacyOptionalSuffix marks a legacy module.yaml parentModules dependency as
// conditional (skippable if the parent module is absent). See
// go_lib/dependency/extenders/moduledependency for the original parser.
const legacyOptionalSuffix = "!optional"

// legacyRequirementsToCR projects legacy module.yaml requirements (flat strings
// plus a name to constraint map) onto the PackageRequirements CR shape. A constraint
// ending in "!optional" maps to a conditional dependency; the suffix is stripped from
// the surfaced constraint string.
func legacyRequirementsToCR(req *v1alpha1.ModuleRequirements) *v1alpha1.PackageRequirements {
	kubernetes := versionConstraintToCR(req.Kubernetes)
	deckhouse := versionConstraintToCR(req.Deckhouse)

	var moduleReqs *v1alpha1.PackageModulesRequirements
	if len(req.ParentModules) > 0 {
		var (
			mandatory   []v1alpha1.PackageModuleDependency
			conditional []v1alpha1.PackageModuleDependency
		)

		for name, constraint := range req.ParentModules {
			raw, optional := strings.CutSuffix(constraint, legacyOptionalSuffix)
			dep := v1alpha1.PackageModuleDependency{
				Name:       name,
				Constraint: strings.TrimSpace(raw),
			}

			if optional {
				conditional = append(conditional, dep)
			} else {
				mandatory = append(mandatory, dep)
			}
		}

		if len(mandatory) > 0 || len(conditional) > 0 {
			moduleReqs = &v1alpha1.PackageModulesRequirements{
				Mandatory:   mandatory,
				Conditional: conditional,
			}
		}
	}

	if kubernetes == nil && deckhouse == nil && moduleReqs == nil {
		return nil
	}

	return &v1alpha1.PackageRequirements{
		Kubernetes: kubernetes,
		Deckhouse:  deckhouse,
		Modules:    moduleReqs,
	}
}

// versionConstraintToCR wraps a raw semver constraint string into the v1alpha1
// VersionConstraint CR shape, returning nil when the string is empty.
func versionConstraintToCR(raw string) *v1alpha1.VersionConstraint {
	if len(raw) == 0 {
		return nil
	}

	return &v1alpha1.VersionConstraint{Constraint: raw}
}

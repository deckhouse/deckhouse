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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
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

// reconciler promotes draft ModulePackageVersion resources by loading package
// metadata from the registry image and removing the draft label.
type reconciler struct {
	client   client.Client
	logger   *log.Logger
	registry *registry.Service
	dc       dependency.Container
}

// RegisterController creates and registers the ModulePackageVersion controller.
// It watches ModulePackageVersion resources and reconciles draft versions by
// fetching metadata from the package registry and promoting them to non-draft status.
func RegisterController(runtimeManager manager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		client:   runtimeManager.GetClient(),
		logger:   logger,
		registry: registry.NewService(dc, logger),
		dc:       dc,
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		Named(controllerName).
		For(&v1alpha1.ModulePackageVersion{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}

// Reconcile handles a single ModulePackageVersion event. Draft resources are
// promoted by loading metadata; deleted resources have their finalizers removed
// once no Module references remain (usedByCount == 0).
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	if mpv.Labels[v1alpha1.ModulePackageVersionLabelLegacy] == "true" {
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

// handleDelete removes the finalizer from the ModulePackageVersion once it is
// no longer referenced by any Module (usedByCount == 0). While references exist,
// the reconcile is requeued every 15 seconds to wait for Modules to release the MPV.
func (r *reconciler) handleDelete(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) (ctrl.Result, error) {
	logger := r.logger.With(
		slog.String("name", mpv.Name),
		slog.String("package", mpv.Spec.PackageName),
		slog.String("version", mpv.Spec.PackageVersion),
		slog.String("repository", mpv.Spec.PackageRepositoryName))

	if mpv.Status.UsedByCount > 0 {
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
		setFromPackageDefinition(mpv, meta.packageDefinition)
	case meta.moduleDefinition != nil:
		setFromModuleDefinition(mpv, meta.moduleDefinition)
	}

	mpv.Status.PackageMetadata.Changelog = &v1alpha1.PackageChangelog{
		Features: meta.changelog.Features,
		Fixes:    meta.changelog.Fixes,
	}
}

// setFromPackageDefinition projects a parsed v2 package.yaml onto the MPV status.
// Mirrors the APV controller: only fields present on dto.ModuleDefinition are
// surfaced (stage, descriptions, requirements). Module-only status fields
// (category, licensing, version-compatibility) are intentionally not populated
// here — extend dto.ModuleDefinition if you need to surface them.
func setFromPackageDefinition(mpv *v1alpha1.ModulePackageVersion, pd *dto.ModuleDefinition) {
	mpv.Status.PackageMetadata = &v1alpha1.ModulePackageVersionStatusMetadata{
		Stage: pd.Stage,
		Description: &v1alpha1.PackageDescription{
			Ru: pd.Descriptions.Ru,
			En: pd.Descriptions.En,
		},
		Requirements: requirementsToCR(pd.Requirements),
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
}

// requirementsToCR projects parsed package requirements onto the v1alpha1
// PackageRequirements CR shape. Returns nil when no requirements are configured
// so the status field omits cleanly via omitempty.
func requirementsToCR(r dto.Requirements) *v1alpha1.PackageRequirements {
	kubernetes := versionConstraintToCR(r.Kubernetes.Constraint)
	deckhouse := versionConstraintToCR(r.Deckhouse.Constraint)
	modulesCR := moduleRequirementsToCR(r.Modules)

	if kubernetes == nil && deckhouse == nil && modulesCR == nil {
		return nil
	}

	return &v1alpha1.PackageRequirements{
		Kubernetes: kubernetes,
		Deckhouse:  deckhouse,
		Modules:    modulesCR,
	}
}

// legacyOptionalSuffix marks a legacy module.yaml parentModules dependency as
// conditional (skippable if the parent module is absent). See
// go_lib/dependency/extenders/moduledependency for the original parser.
const legacyOptionalSuffix = "!optional"

// legacyRequirementsToCR projects a legacy v1alpha1.ModuleRequirements (flat strings
// plus a name → constraint map) onto the new PackageRequirements CR shape. A constraint
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

// moduleRequirementsToCR projects dto.ModulesRequirements onto the v1alpha1
// PackageModulesRequirements CR shape, returning nil when mandatory, conditional,
// anyOf, and noneOf are all empty.
func moduleRequirementsToCR(mr dto.ModulesRequirements) *v1alpha1.PackageModulesRequirements {
	if len(mr.Mandatory) == 0 && len(mr.Conditional) == 0 && len(mr.AnyOf) == 0 && len(mr.NoneOf) == 0 {
		return nil
	}

	return &v1alpha1.PackageModulesRequirements{
		Mandatory:   moduleDependenciesToCR(mr.Mandatory),
		Conditional: moduleDependenciesToCR(mr.Conditional),
		AnyOf:       moduleGroupsToCR(mr.AnyOf),
		NoneOf:      moduleGroupsToCR(mr.NoneOf),
	}
}

// moduleDependenciesToCR projects a slice of dto.ModuleDependency onto the
// v1alpha1 PackageModuleDependency CR slice. Returns nil for empty input so
// the parent CR omitempty fields render cleanly.
func moduleDependenciesToCR(deps []dto.ModuleDependency) []v1alpha1.PackageModuleDependency {
	if len(deps) == 0 {
		return nil
	}

	out := make([]v1alpha1.PackageModuleDependency, 0, len(deps))
	for _, dep := range deps {
		out = append(out, v1alpha1.PackageModuleDependency{
			Name:       dep.Name,
			Constraint: dep.Constraint,
		})
	}

	return out
}

// moduleGroupsToCR projects a slice of dto.ModuleGroup onto the v1alpha1
// PackageModuleGroup CR slice. Used for both anyOf and noneOf — the shape is
// identical at the CR layer; the bucket semantics live on the field they're
// attached to. Returns nil for empty input so the parent CR omitempty field
// renders cleanly. The legacy module.yaml path does not carry anyOf or noneOf
// groups and never reaches this function — only the v2 package.yaml path
// (setFromPackageDefinition) emits group metadata.
func moduleGroupsToCR(groups []dto.ModuleGroup) []v1alpha1.PackageModuleGroup {
	if len(groups) == 0 {
		return nil
	}

	out := make([]v1alpha1.PackageModuleGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, v1alpha1.PackageModuleGroup{
			Name:        g.Name,
			Description: g.Description,
			Modules:     moduleDependenciesToCR(g.Modules),
		})
	}

	return out
}

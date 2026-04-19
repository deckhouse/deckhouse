/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package modulepackageversion

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName          = "d8-module-package-version-controller"
	maxConcurrentReconciles = 1
	requeueTime             = 30 * time.Second
	defaultPathSegment      = "version"
	legacyPathSegment       = "release"
)

type reconciler struct {
	client client.Client
	dc     dependency.Container
	logger *log.Logger
}

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		dc:     dc,
		logger: logger,
	}

	mpvController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ModulePackageVersion{}).
		Complete(mpvController)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconciling ModulePackageVersion")

	mpv := new(v1alpha1.ModulePackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, mpv); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("module package version not found")
			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get module package version", log.Err(err))
		return ctrl.Result{}, err
	}

	// handle delete event
	if !mpv.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, mpv)
	}

	// add finalizer if not set
	if !controllerutil.ContainsFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer) {
		logger.Debug("adding finalizer to module package version")

		patch := client.MergeFrom(mpv.DeepCopy())
		controllerutil.AddFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer)

		if err := r.client.Patch(ctx, mpv, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer to ModulePackageVersion %s: %w", mpv.Name, err)
		}
	}

	// skip non-draft resources
	if !mpv.IsDraft() {
		logger.Debug("package is not draft")
		return ctrl.Result{}, nil
	}

	if err := r.handleCreateOrUpdate(ctx, logger, mpv); err != nil {
		logger.Warn("failed to handle module package version", log.Err(err))
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) handleCreateOrUpdate(ctx context.Context, logger *log.Logger, mpv *v1alpha1.ModulePackageVersion) error {
	original := mpv.DeepCopy()

	logger.Debug("handling ModulePackageVersion")
	defer logger.Debug("handle ModulePackageVersion complete")

	// Get registry credentials from PackageRepository resource
	var packageRepo v1alpha1.PackageRepository
	if err := r.client.Get(ctx, types.NamespacedName{Name: mpv.Spec.PackageRepositoryName}, &packageRepo); err != nil {
		r.markEnrichmentFailed(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonGetPackageRepoErr,
			fmt.Sprintf("failed to get packageRepository %s: %s", mpv.Spec.PackageRepositoryName, err.Error()),
		)

		if patchErr := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); patchErr != nil {
			return fmt.Errorf("patch status ModulePackageVersion %s: %w", mpv.Name, patchErr)
		}

		return fmt.Errorf("get packageRepository %s: %w", mpv.Spec.PackageRepositoryName, err)
	}

	// Determine registry path segment: "version" (default) or "release" (legacy)
	pathSegment := defaultPathSegment
	if mpv.Labels[v1alpha1.ModulePackageVersionLabelLegacy] == "true" {
		pathSegment = legacyPathSegment
	}

	registryPath := path.Join(packageRepo.Spec.Registry.Repo, mpv.Spec.PackageName, pathSegment)

	logger.Debug(
		"registry path",
		slog.String("path", registryPath),
		slog.String("segment", pathSegment),
	)

	opts := utils.GenerateRegistryOptions(&utils.RegistryConfig{
		DockerConfig: packageRepo.Spec.Registry.DockerCFG,
		CA:           packageRepo.Spec.Registry.CA,
		Scheme:       packageRepo.Spec.Registry.Scheme,
	}, r.logger)

	registryClient, err := r.dc.GetRegistryClient(registryPath, opts...)
	if err != nil {
		r.markEnrichmentFailed(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonGetRegistryClientErr,
			fmt.Sprintf("failed to get registry client: %s", err.Error()),
		)

		if patchErr := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); patchErr != nil {
			return fmt.Errorf("patch status ModulePackageVersion %s: %w", mpv.Name, patchErr)
		}

		return fmt.Errorf("get registry client for %s: %w", mpv.Name, err)
	}

	img, err := registryClient.Image(ctx, mpv.Spec.PackageVersion)
	if err != nil {
		r.markEnrichmentFailed(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonGetImageErr,
			fmt.Sprintf("failed to get image: %s", err.Error()),
		)

		if patchErr := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); patchErr != nil {
			return fmt.Errorf("patch status ModulePackageVersion %s: %w", mpv.Name, patchErr)
		}

		return fmt.Errorf("get image for %s: %w", mpv.Name+":"+mpv.Spec.PackageVersion, err)
	}

	meta, err := r.fetchModuleMetadata(ctx, img)
	if err != nil {
		r.markEnrichmentFailed(
			mpv,
			v1alpha1.ModulePackageVersionConditionReasonFetchErr,
			fmt.Sprintf("failed to fetch package metadata: %s", err.Error()),
		)

		if patchErr := r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); patchErr != nil {
			return fmt.Errorf("patch status ModulePackageVersion %s: %w", mpv.Name, patchErr)
		}

		return fmt.Errorf("failed to fetch package metadata %s: %w", mpv.Name, err)
	}

	mpv = enrichWithMetadata(mpv, meta)
	r.markEnriched(mpv)

	logger.Debug("patch module package version status")
	if err = r.client.Status().Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch status ModulePackageVersion %s: %w", mpv.Name, err)
	}

	// Remove draft label
	original = mpv.DeepCopy()
	delete(mpv.Labels, v1alpha1.ModulePackageVersionLabelDraft)

	if err = r.client.Patch(ctx, mpv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch ModulePackageVersion %s: %w", mpv.Name, err)
	}

	return nil
}

func (r *reconciler) handleDelete(ctx context.Context, mpv *v1alpha1.ModulePackageVersion) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", mpv.Name))
	logger.Debug("deleting ModulePackageVersion")

	if mpv.Status.UsedByCount > 0 {
		logger.Warn(
			"module package version is used by modules, skipping deletion",
			slog.Int("used_by_count", mpv.Status.UsedByCount),
		)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if controllerutil.ContainsFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer) {
		logger.Debug("removing finalizer from module package version")

		patch := client.MergeFrom(mpv.DeepCopy())
		controllerutil.RemoveFinalizer(mpv, v1alpha1.ModulePackageVersionFinalizer)

		if err := r.client.Patch(ctx, mpv, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer from ModulePackageVersion %s: %w", mpv.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

func enrichWithMetadata(mpv *v1alpha1.ModulePackageVersion, meta *moduleMetadata) *v1alpha1.ModulePackageVersion {
	if meta == nil {
		return mpv
	}

	// v2 format: package.yaml
	if meta.PackageDefinition != nil {
		mpv = enrichWithPackageDefinition(mpv, meta.PackageDefinition)
	} else if meta.ModuleDefinition != nil {
		// legacy format: module.yaml
		mpv = enrichWithModuleDefinition(mpv, meta.ModuleDefinition)
	}

	if mpv.Status.PackageMetadata != nil {
		mpv.Status.PackageMetadata.Changelog = meta.Changelog
	}

	return mpv
}

func enrichWithPackageDefinition(mpv *v1alpha1.ModulePackageVersion, pd *PackageDefinition) *v1alpha1.ModulePackageVersion {
	mpv.Status.PackageMetadata = &v1alpha1.ModulePackageVersionStatusMetadata{
		Category: pd.Category,
		Stage:    pd.Stage,
	}

	if pd.Description != nil {
		mpv.Status.PackageMetadata.Description = &v1alpha1.PackageDescription{
			Ru: pd.Description.Ru,
			En: pd.Description.En,
		}
	}

	if pd.Licensing != nil {
		mpv.Status.PackageMetadata.Licensing = &v1alpha1.PackageLicensing{
			Editions: convertLicensingEditions(pd.Licensing.Editions),
		}
	}

	if pd.Requirements != nil {
		mpv.Status.PackageMetadata.Requirements = &v1alpha1.PackageRequirements{
			Deckhouse:  pd.Requirements.Deckhouse,
			Kubernetes: pd.Requirements.Kubernetes,
			Modules:    pd.Requirements.Modules,
		}
	}

	if pd.VersionCompatibilityRules != nil {
		if pd.VersionCompatibilityRules.Upgrade.From != "" || pd.VersionCompatibilityRules.Downgrade.To != "" {
			mpv.Status.PackageMetadata.Compatibility = &v1alpha1.PackageVersionCompatibilityRules{
				Upgrade: &v1alpha1.PackageVersionCompatibilityRule{
					From:             pd.VersionCompatibilityRules.Upgrade.From,
					AllowSkipPatches: int(pd.VersionCompatibilityRules.Upgrade.AllowSkipPatches),
					AllowSkipMinor:   int(pd.VersionCompatibilityRules.Upgrade.AllowSkipMinor),
					AllowSkipMajor:   int(pd.VersionCompatibilityRules.Upgrade.AllowSkipMajor),
				},
				Downgrade: &v1alpha1.PackageVersionCompatibilityRule{
					To:               pd.VersionCompatibilityRules.Downgrade.To,
					AllowSkipPatches: int(pd.VersionCompatibilityRules.Downgrade.AllowSkipPatches),
					AllowSkipMinor:   int(pd.VersionCompatibilityRules.Downgrade.AllowSkipMinor),
					AllowSkipMajor:   int(pd.VersionCompatibilityRules.Downgrade.AllowSkipMajor),
					MaxRollback:      int(pd.VersionCompatibilityRules.Downgrade.MaxRollback),
				},
			}
		}
	}

	return mpv
}

func enrichWithModuleDefinition(mpv *v1alpha1.ModulePackageVersion, def *moduletypes.Definition) *v1alpha1.ModulePackageVersion {
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
		mpv.Status.PackageMetadata.Requirements = &v1alpha1.PackageRequirements{
			Deckhouse:  def.Requirements.Deckhouse,
			Kubernetes: def.Requirements.Kubernetes,
			Modules:    def.Requirements.ParentModules,
		}
	}

	return mpv
}

func convertLicensingEditions(editions map[string]PackageEdition) map[string]v1alpha1.PackageEdition {
	if editions == nil {
		return nil
	}
	result := make(map[string]v1alpha1.PackageEdition)
	for k, v := range editions {
		result[k] = v1alpha1.PackageEdition{
			Available: v.Available,
		}
	}
	return result
}

func (r *reconciler) markEnriched(mpv *v1alpha1.ModulePackageVersion) {
	r.setCondition(mpv, metav1.ConditionTrue, "MetadataLoaded", "")
}

func (r *reconciler) markEnrichmentFailed(mpv *v1alpha1.ModulePackageVersion, reason, message string) {
	r.setCondition(mpv, metav1.ConditionFalse, reason, message)
}

func (r *reconciler) setCondition(mpv *v1alpha1.ModulePackageVersion, status metav1.ConditionStatus, reason, message string) {
	condType := v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded
	now := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range mpv.Status.Conditions {
		if cond.Type == condType {
			if cond.Status != status {
				mpv.Status.Conditions[idx].LastTransitionTime = now
				mpv.Status.Conditions[idx].Status = status
			}
			mpv.Status.Conditions[idx].Reason = reason
			mpv.Status.Conditions[idx].Message = message
			return
		}
	}

	mpv.Status.Conditions = append(mpv.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

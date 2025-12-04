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

package applicationpackageversion

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-package-version-controller"

	maxConcurrentReconciles = 1
	requeueTime             = 30 * time.Second
)

type reconciler struct {
	client client.Client
	logger *log.Logger
	dc     dependency.Container
}

func RegisterController(
	runtimeManager manager.Manager,
	dc dependency.Container,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		logger: logger,
		dc:     dc,
	}

	applicationPackageVersionController, err := controller.New(controllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ApplicationPackageVersion{}).
		Complete(applicationPackageVersionController)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res := ctrl.Result{}

	r.logger.Debug("reconciling ApplicationPackageVersion", slog.String("name", req.Name))

	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, apv); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debug("application package version not found", slog.String("name", req.Name))

			return res, nil
		}

		r.logger.Warn("failed to get application package version", slog.String("name", req.Name), log.Err(err))

		return res, err
	}

	// handle delete event
	if !apv.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, apv)
	}

	// add finalizer if it is not set
	if !controllerutil.ContainsFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer) {
		r.logger.Debug("adding finalizer to application package version", slog.String("name", apv.Name))

		patch := client.MergeFrom(apv.DeepCopy())

		controllerutil.AddFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer)

		err := r.client.Patch(ctx, apv, patch)
		if err != nil {
			return res, fmt.Errorf("add finalizer to ApplicationPackageVersion %s: %w", apv.Name, err)
		}
	}

	// skip handle for non drafted resources
	if !apv.IsDraft() {
		r.logger.Debug("package is not draft", slog.String("package_name", apv.Name))

		return res, nil
	}

	// handle create/update events
	err := r.handleCreateOrUpdate(ctx, apv)
	if err != nil {
		r.logger.Warn("failed to handle application package version", slog.String("name", req.Name), log.Err(err))

		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	return res, nil
}

func (r *reconciler) handleCreateOrUpdate(ctx context.Context, apv *v1alpha1.ApplicationPackageVersion) error {
	original := apv.DeepCopy()
	logger := r.logger.With(slog.String("name", apv.Name))

	logger.Debug("handling ApplicationPackageVersion")
	defer logger.Debug("handle ApplicationPackageVersion complete")

	// Get registry credentials from PackageRepository resource
	var packageRepo v1alpha1.PackageRepository
	err := r.client.Get(ctx, types.NamespacedName{Name: apv.Spec.PackageRepository}, &packageRepo)
	if err != nil {
		r.SetConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded,
			v1alpha1.ApplicationPackageVersionConditionReasonGetPackageRepoErr,
			fmt.Sprintf("failed to get packageRepository %s: %s", apv.Spec.PackageRepository, err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, apv, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status ApplicationPackageVersion %s: %w", apv.Name, patchErr)
		}

		return fmt.Errorf("get packageRepository %s: %w", apv.Spec.PackageRepository, err)
	}

	logger.Debug("got package repository", slog.String("repo", packageRepo.Spec.Registry.Repo))

	// Create go registry client from credentials from PackageRepository
	// example path: registry.deckhouse.io/sys/deckhouse-oss/packages/$package/version:$version
	registryPath := path.Join(packageRepo.Spec.Registry.Repo, apv.Spec.PackageName, "version")

	logger.Debug("registry path", slog.String("path", registryPath))

	opts := utils.GenerateRegistryOptions(&utils.RegistryConfig{
		DockerConfig: packageRepo.Spec.Registry.DockerCFG,
		CA:           packageRepo.Spec.Registry.CA,
		Scheme:       packageRepo.Spec.Registry.Scheme,
		// UserAgent: ,
	}, r.logger)

	registryClient, err := r.dc.GetRegistryClient(registryPath, opts...)
	if err != nil {
		r.SetConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded,
			v1alpha1.ApplicationPackageVersionConditionReasonGetRegistryClientErr,
			fmt.Sprintf("failed to get registry client: %s", err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, apv, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status ApplicationPackageVersion %s: %w", apv.Name, patchErr)
		}

		return fmt.Errorf("get registry client for %s: %w", apv.Name, err)
	}

	// Get package.yaml from image
	img, err := registryClient.Image(ctx, apv.Spec.Version)
	if err != nil {
		r.SetConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded,
			v1alpha1.ApplicationPackageVersionConditionReasonGetImageErr,
			fmt.Sprintf("failed to get image: %s", err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, apv, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status ApplicationPackageVersion %s: %w", apv.Name, patchErr)
		}

		return fmt.Errorf("get image for %s: %w", apv.Name+":"+apv.Spec.Version, err)
	}

	packageMeta, err := r.fetchPackageMetadata(ctx, img)
	if err != nil {
		r.SetConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded,
			v1alpha1.ApplicationPackageVersionConditionReasonFetchErr,
			fmt.Sprintf("failed to fetch package metadata: %s", err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, apv, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status ApplicationPackageVersion %s: %w", apv.Name, patchErr)
		}

		return fmt.Errorf("failed to fetch package metadata %s: %w", apv.Name, err)
	}

	if packageMeta.PackageDefinition != nil {
		logger.Debug("got metadata from package.yaml", slog.String("meta_name", packageMeta.PackageDefinition.Name))
	}

	// Patch the status
	apv = enrichWithPackageDefinition(apv, packageMeta.PackageDefinition)

	apv = r.SetConditionTrue(apv, v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded)

	logger.Debug("patch package version status")
	err = r.client.Status().Patch(ctx, apv, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch status ApplicationPackageVersion %s: %w", apv.Name, err)
	}

	// Delete label "draft" and patch the main object
	original = apv.DeepCopy()

	delete(apv.Labels, v1alpha1.ApplicationPackageVersionLabelDraft)

	err = r.client.Patch(ctx, apv, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch ApplicationPackageVersion %s: %w", apv.Name, err)
	}

	return nil
}

func (r *reconciler) handleDelete(ctx context.Context, apv *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", apv.Name))
	logger.Debug("deleting ApplicationPackageVersion")
	defer logger.Debug("delete ApplicationPackageVersion complete")

	res := ctrl.Result{}

	if apv.Status.UsedByCount > 0 {
		logger.Warn("application package version is used by applications, skipping deletion")

		return res, fmt.Errorf("application package version is used by applications")
	}

	if controllerutil.ContainsFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer) {
		logger.Debug("removing finalizer from application package version")

		patch := client.MergeFrom(apv.DeepCopy())

		controllerutil.RemoveFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer)

		err := r.client.Patch(ctx, apv, patch)
		if err != nil {
			return res, fmt.Errorf("remove finalizer from ApplicationPackageVersion %s: %w", apv.Name, err)
		}
	}

	return res, nil
}

func (r *reconciler) SetConditionTrue(apv *v1alpha1.ApplicationPackageVersion, condType string) *v1alpha1.ApplicationPackageVersion {
	time := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range apv.Status.Conditions {
		if cond.Type == condType {
			apv.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionTrue {
				apv.Status.Conditions[idx].LastTransitionTime = time
				apv.Status.Conditions[idx].Status = corev1.ConditionTrue
			}

			apv.Status.Conditions[idx].Reason = ""
			apv.Status.Conditions[idx].Message = ""

			return apv
		}
	}

	apv.Status.Conditions = append(apv.Status.Conditions, v1alpha1.ApplicationPackageVersionCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})

	return apv
}

func (r *reconciler) SetConditionFalse(apv *v1alpha1.ApplicationPackageVersion, condType string, reason string, message string) *v1alpha1.ApplicationPackageVersion {
	time := metav1.NewTime(r.dc.GetClock().Now())

	for idx, cond := range apv.Status.Conditions {
		if cond.Type == condType {
			apv.Status.Conditions[idx].LastProbeTime = time
			if cond.Status != corev1.ConditionFalse {
				apv.Status.Conditions[idx].LastTransitionTime = time
				apv.Status.Conditions[idx].Status = corev1.ConditionFalse
			}

			apv.Status.Conditions[idx].Reason = reason
			apv.Status.Conditions[idx].Message = message

			return apv
		}
	}

	apv.Status.Conditions = append(apv.Status.Conditions, v1alpha1.ApplicationPackageVersionCondition{
		Type:               condType,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      time,
		LastTransitionTime: time,
	})

	return apv
}

func enrichWithPackageDefinition(apv *v1alpha1.ApplicationPackageVersion, pd *PackageDefinition) *v1alpha1.ApplicationPackageVersion {
	if pd == nil {
		return apv
	}

	apv.Status.PackageName = pd.Name
	apv.Status.Version = pd.Version

	apv.Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Category: pd.Category,
		Stage:    pd.Stage,
	}
	if pd.Description != nil {
		apv.Status.PackageMetadata.Description = &v1alpha1.PackageDescription{
			Ru: pd.Description.Ru,
			En: pd.Description.En,
		}
	}
	if pd.Licensing != nil {
		apv.Status.PackageMetadata.Licensing = &v1alpha1.PackageLicensing{
			Editions: convertLicensingEditions(pd.Licensing.Editions),
		}
	}

	if pd.Requirements != nil {
		apv.Status.PackageMetadata.Requirements = &v1alpha1.PackageRequirements{
			Deckhouse:  pd.Requirements.Deckhouse,
			Kubernetes: pd.Requirements.Kubernetes,
			Modules:    pd.Requirements.Modules,
		}
	}

	if pd.VersionCompatibilityRules != nil {
		if pd.VersionCompatibilityRules.Upgrade.From != "" || pd.VersionCompatibilityRules.Downgrade.To != "" {
			apv.Status.PackageMetadata.Compatibility = &v1alpha1.PackageVersionCompatibilityRules{
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

	return apv
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

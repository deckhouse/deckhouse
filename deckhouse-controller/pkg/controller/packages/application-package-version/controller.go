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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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

	packageVersion := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, packageVersion); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("application package version not found", slog.String("name", req.Name))

			return res, nil
		}

		r.logger.Warn("failed to get application package version", slog.String("name", req.Name), log.Err(err))

		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !packageVersion.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting application package version", slog.String("name", req.Name))

		return r.delete(ctx, packageVersion)
	}

	// skip handle for non drafted resources
	if !packageVersion.IsDraft() {
		r.logger.Debug("package is not draft", slog.String("package_name", packageVersion.Name))

		return res, nil
	}

	// handle create/update events
	err := r.handle(ctx, packageVersion)
	if err != nil {
		r.logger.Warn("failed to handle application package version", slog.String("name", req.Name), log.Err(err))

		res.RequeueAfter = requeueTime
		return res, nil
	}

	return res, nil
}

func (r *reconciler) handle(ctx context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) error {
	r.logger.Info("handling ApplicationPackageVersion", slog.String("name", packageVersion.Name))

	// Get registry credentials from PackageRepository resource
	var packageRepo v1alpha1.PackageRepository
	err := r.client.Get(ctx, types.NamespacedName{Name: packageVersion.Spec.Repository}, &packageRepo)
	if err != nil {
		original := packageVersion.DeepCopy()

		packageVersion.SetConditionFalse(
			v1alpha1.ApplicationPackageVersionConditionTypeEnriched,
			v1.NewTime(r.dc.GetClock().Now()),
			v1alpha1.ApplicationPackageVersionConditionReasonGetPackageRepoErr,
			fmt.Sprintf("failed to get packageRepository %s: %s", packageVersion.Spec.Repository, err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, packageVersion, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status packageVersion %s: %w", packageVersion.Name, patchErr)
		}

		return fmt.Errorf("get packageRepository %s: %w", packageVersion.Spec.Repository, err)
	}

	r.logger.Debug("got package repository",
		slog.String("package_version", packageVersion.Name),
		slog.String("repo", packageRepo.Spec.Registry.Repo))

	// Create go registry client from credentials from PackageRepository
	// example path: registry.deckhouse.io/sys/deckhouse-oss/packages/$package/version:$version
	registryPath := path.Join(packageRepo.Spec.Registry.Repo, packageVersion.Spec.PackageName, "version")
	r.logger.Debug("registry path", slog.String("name", packageVersion.Name), slog.String("path", registryPath))
	opts := utils.GenerateRegistryOptions(&utils.RegistryConfig{
		DockerConfig: packageRepo.Spec.Registry.DockerCFG,
		CA:           packageRepo.Spec.Registry.CA,
		Scheme:       packageRepo.Spec.Registry.Scheme,
		// UserAgent: ,
	}, r.logger)

	registryClient, err := r.dc.GetRegistryClient(registryPath, opts...)
	if err != nil {
		original := packageVersion.DeepCopy()
		packageVersion.SetConditionFalse(
			v1alpha1.ApplicationPackageVersionConditionTypeEnriched,
			v1.NewTime(r.dc.GetClock().Now()),
			v1alpha1.ApplicationPackageVersionConditionReasonGetRegistryClientErr,
			fmt.Sprintf("failed to get registry client: %s", err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, packageVersion, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status packageVersion %s: %w", packageVersion.Name, patchErr)
		}

		return fmt.Errorf("get registry client for %s: %w", packageVersion.Name, err)
	}

	// Get package.yaml from image
	img, err := registryClient.Image(ctx, packageVersion.Spec.Version)
	if err != nil {
		original := packageVersion.DeepCopy()
		packageVersion.SetConditionFalse(
			v1alpha1.ApplicationPackageVersionConditionTypeEnriched,
			v1.NewTime(r.dc.GetClock().Now()),
			v1alpha1.ApplicationPackageVersionConditionReasonGetImageErr,
			fmt.Sprintf("failed to get image: %s", err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, packageVersion, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status packageVersion %s: %w", packageVersion.Name, patchErr)
		}

		return fmt.Errorf("get image for %s: %w", packageVersion.Name+":"+packageVersion.Spec.Version, err)
	}

	packageMeta, err := r.fetchPackageMetadata(ctx, img)
	if err != nil {
		original := packageVersion.DeepCopy()
		packageVersion.SetConditionFalse(
			v1alpha1.ApplicationPackageVersionConditionTypeEnriched,
			v1.NewTime(r.dc.GetClock().Now()),
			v1alpha1.ApplicationPackageVersionConditionReasonFetchErr,
			fmt.Sprintf("failed to fetch package metadata: %s", err.Error()),
		)

		patchErr := r.client.Status().Patch(ctx, packageVersion, client.MergeFrom(original))
		if patchErr != nil {
			return fmt.Errorf("patch status packageVersion %s: %w", packageVersion.Name, patchErr)
		}

		return fmt.Errorf("failed to fetch package metadata %s: %w", packageVersion.Name, err)
	}
	if packageMeta.PackageDefinition != nil {
		r.logger.Debug("got metadata from package.yaml", slog.String("name", packageVersion.Name), slog.String("meta_name", packageMeta.PackageDefinition.Name))
	}

	// Start changing the packageVersion object
	original := packageVersion.DeepCopy()

	packageVersion = enrichWithPackageDefinition(packageVersion, packageMeta.PackageDefinition)

	// Patch the status
	packageVersion = packageVersion.SetConditionTrue(v1alpha1.ApplicationPackageVersionConditionTypeEnriched, v1.NewTime(r.dc.GetClock().Now()))

	r.logger.Debug("patch package version status", slog.String("name", packageVersion.Name))
	err = r.client.Status().Patch(ctx, packageVersion, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch status packageVersion %s: %w", packageVersion.Name, err)
	}

	// Delete label "draft" and patch the main object
	delete(packageVersion.Labels, v1alpha1.ApplicationPackageVersionLabelDraft)
	err = r.client.Patch(ctx, packageVersion, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch packageVersion %s: %w", packageVersion.Name, err)
	}

	r.logger.Info("handle ApplicationPackageVersion complete", slog.String("name", packageVersion.Name))
	return nil
}

func (r *reconciler) delete(_ context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	res := ctrl.Result{}

	r.logger.Info("deleting ApplicationPackageVersion", slog.String("name", packageVersion.Name))

	return res, nil
}

func enrichWithPackageDefinition(apv *v1alpha1.ApplicationPackageVersion, pd *PackageDefinition) *v1alpha1.ApplicationPackageVersion {
	if pd == nil {
		return apv
	}

	apv.Status.PackageName = pd.Name
	apv.Status.Version = pd.Version

	apv.Status.Metadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Description: &v1alpha1.PackageDescription{
			Ru: pd.Description.Ru,
			En: pd.Description.En,
		},
		Category: pd.Category,
		Stage:    pd.Stage,
	}
	if pd.Licensing != nil {
		apv.Status.Metadata.Licensing = &v1alpha1.PackageLicensing{
			Editions: convertLicensingEditions(pd.Licensing.Editions),
		}
	}

	if pd.Requirements != nil {
		apv.Status.Metadata.Requirements = &v1alpha1.PackageRequirements{
			Deckhouse:  pd.Requirements.Deckhouse,
			Kubernetes: pd.Requirements.Kubernetes,
			Modules:    pd.Requirements.Modules,
		}
	}

	if pd.VersionCompatibilityRules != nil {
		if pd.VersionCompatibilityRules.Upgrade.From != "" || pd.VersionCompatibilityRules.Downgrade.To != "" {
			apv.Status.Metadata.Compatibility = &v1alpha1.PackageVersionCompatibilityRules{
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

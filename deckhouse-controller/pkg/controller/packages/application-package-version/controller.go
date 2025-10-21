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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
)

type reconciler struct {
	client client.Client
	logger *log.Logger
	dc     dependency.Container
}

func RegisterController(
	runtimeManager manager.Manager,
	logger *log.Logger,
) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
		logger: logger,
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

		return res, err
	}

	return res, nil
}

func (r *reconciler) handle(ctx context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) error {
	// TODO: implement application package version reconciliation logic
	r.logger.Info("handling ApplicationPackageVersion", slog.String("name", packageVersion.Name))

	packageName := packageVersion.Labels["package"]
	packageRepoName := packageVersion.Labels["repository"]

	// - get registry creds from PackageRepository resource
	var pr v1alpha1.PackageRepository
	err := r.client.Get(ctx, types.NamespacedName{Name: packageRepoName}, &pr)
	if err != nil {
		return fmt.Errorf("get packageRepository %s: %w", packageRepoName, err)
	}

	// - create go registry client from creds from PackageRepository
	// example path: registry.deckhouse.io/sys/deckhouse-oss/packages/$package/release-channel:stable
	// registryPath := path.Join(pr.Spec.Registry.Repo, packageVersion.Spec.PackageName, "release-channel")
	registryPath := path.Join(pr.Spec.Registry.Repo, packageName, "release") // test
	opts := utils.GenerateRegistryOptions(&utils.RegistryConfig{
		DockerConfig: pr.Spec.Registry.DockerCFG,
		CA:           pr.Spec.Registry.CA,
		Scheme:       pr.Spec.Registry.Scheme,
		// UserAgent: ,
	}, r.logger)
	registryClient, err := r.dc.GetRegistryClient(registryPath, opts...)
	if err != nil {
		return fmt.Errorf("get registry client for %s: %w", packageVersion.Name, err)
	}

	// - get package.yaml from release image
	img, err := registryClient.Image(ctx, "stable")
	if err != nil {
		return fmt.Errorf("get release image for %s: %w", packageVersion.Name, err)
	}

	packageMeta, err := r.fetchPackageMetadata(ctx, img)
	if err != nil {
		return fmt.Errorf("fetch package release image metadata for %s: %w", packageVersion.Name, err)
	}

	// here we start changing the packageVersion object
	original := packageVersion.DeepCopy()

	packageVersion = enrichWithPackageDefinition(packageVersion, packageMeta.PackageDefinition)

	// - delete label "draft" and patch the main object
	delete(packageVersion.Labels, "draft")

	err = r.client.Patch(ctx, packageVersion, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch packageVersion %s: %w", packageVersion.Name, err)
	}

	// - patch the status
	err = r.client.Status().Patch(ctx, packageVersion, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("patch status packageVersion %s: %w", packageVersion.Name, err)
	}

	return nil
}

func (r *reconciler) delete(_ context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	// TODO: implement application package version deletion logic
	r.logger.Info("deleting ApplicationPackageVersion", slog.String("name", packageVersion.Name))
	return ctrl.Result{}, nil
}

func enrichWithPackageDefinition(apv *v1alpha1.ApplicationPackageVersion, pd *PackageDefinition) *v1alpha1.ApplicationPackageVersion {
	apv.Status.PackageName = pd.Name
	apv.Status.Version = pd.Version

	apv.Status.Metadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Description: &v1alpha1.PackageDescription{
			Ru: pd.Description.Ru,
			En: pd.Description.En,
		},
		Category: pd.Category,
		Stage:    pd.Stage,
		Licensing: &v1alpha1.PackageLicensing{
			Editions: convertLicensingEditions(pd.Licensing.Editions),
		},
	}

	if pd.Requirements != nil {
		apv.Status.Metadata.Requirements = &v1alpha1.PackageRequirements{
			Deckhouse:  pd.Requirements.Deckhouse,
			Kubernetes: pd.Requirements.Kubernetes,
			Modules:    pd.Requirements.Modules,
		}
	}

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

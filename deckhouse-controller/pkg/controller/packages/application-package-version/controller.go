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

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
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

	// Only watch PackageVersions with draft label
	draftPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		labels := obj.GetLabels()
		return labels != nil && labels[v1alpha1.ApplicationPackageVersionLabelDraft] == "true"
	})

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ApplicationPackageVersion{}).
		WithEventFilter(draftPredicate).
		Complete(applicationPackageVersionController)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Debug("reconciling ApplicationPackageVersion", slog.String("name", req.Name))

	packageVersion := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, packageVersion); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("application package version not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}
		r.logger.Error("failed to get application package version", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{Requeue: true}, nil
	}

	// handle delete event
	if !packageVersion.DeletionTimestamp.IsZero() {
		r.logger.Debug("deleting application package version", slog.String("name", req.Name))
		return r.delete(ctx, packageVersion)
	}

	// handle create/update events
	return r.handle(ctx, packageVersion)
}

func (r *reconciler) handle(ctx context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	r.logger.Info("handling ApplicationPackageVersion", slog.String("name", packageVersion.Name))

	// Check if this is a draft version
	labels := packageVersion.GetLabels()
	if labels == nil || labels[v1alpha1.ApplicationPackageVersionLabelDraft] != "true" {
		r.logger.Debug("not a draft version, skipping", slog.String("name", packageVersion.Name))
		return ctrl.Result{}, nil
	}

	// Extract repository and package name from labels
	repositoryName := labels[v1alpha1.ApplicationPackageVersionLabelRepository]
	packageName := labels[v1alpha1.ApplicationPackageVersionLabelPackage]

	if repositoryName == "" || packageName == "" {
		r.logger.Error("missing repository or package label",
			slog.String("name", packageVersion.Name),
			slog.String("repository", repositoryName),
			slog.String("package", packageName))
		return ctrl.Result{}, fmt.Errorf("missing repository or package label")
	}

	// Get PackageRepository to obtain registry credentials
	repo := &v1alpha1.PackageRepository{}
	err := r.client.Get(ctx, types.NamespacedName{Name: repositoryName}, repo)
	if err != nil {
		r.logger.Error("failed to get package repository",
			slog.String("repository", repositoryName),
			log.Err(err))
		return ctrl.Result{}, err
	}

	// Extract version from resource name (format: <repo>-<package>-<version>)
	// Parse version from name
	version, err := r.extractVersionFromName(packageVersion.Name, repositoryName, packageName)
	if err != nil {
		r.logger.Error("failed to extract version from name",
			slog.String("name", packageVersion.Name),
			log.Err(err))
		return ctrl.Result{}, err
	}

	// Generate registry options
	registryConfig := &utils.RegistryConfig{
		DockerConfig: repo.Spec.Registry.DockerCFG,
		Scheme:       repo.Spec.Registry.Scheme,
		CA:           repo.Spec.Registry.CA,
		UserAgent:    "deckhouse-package-version-controller",
	}
	opts := utils.GenerateRegistryOptions(registryConfig, r.logger)

	// Download metadata from registry
	// Path: <repo>/<packageName>/version:<version>
	metadataPath := path.Join(repo.Spec.Registry.Repo, packageName, "version")
	registryClient, err := r.dc.GetRegistryClient(metadataPath, opts...)
	if err != nil {
		r.logger.Error("failed to create registry client", log.Err(err))
		return ctrl.Result{}, err
	}

	// Get the image for the version tag to extract metadata
	image, err := registryClient.Image(ctx, version)
	if err != nil {
		r.logger.Error("failed to get image",
			slog.String("package", packageName),
			slog.String("version", version),
			log.Err(err))
		return ctrl.Result{}, err
	}

	// Extract metadata from image config and layers
	// In a real implementation, you would download and parse package.yaml, changelog.yaml, version.json
	// For now, we'll populate basic information
	metadata := r.extractMetadata(image, version)

	// Update status with metadata
	err = ctrlutils.UpdateStatusWithRetry(ctx, r.client, packageVersion, func() error {
		packageVersion.Status.PackageName = packageName
		packageVersion.Status.Version = version
		packageVersion.Status.Metadata = metadata
		return nil
	})
	if err != nil {
		r.logger.Error("failed to update package version status", log.Err(err))
		return ctrl.Result{}, err
	}

	// Remove draft label
	err = ctrlutils.UpdateWithRetry(ctx, r.client, packageVersion, func() error {
		labels := packageVersion.GetLabels()
		if labels != nil {
			delete(labels, v1alpha1.ApplicationPackageVersionLabelDraft)
			packageVersion.SetLabels(labels)
		}
		return nil
	})
	if err != nil {
		r.logger.Error("failed to remove draft label", log.Err(err))
		return ctrl.Result{}, err
	}

	r.logger.Info("successfully enriched package version metadata",
		slog.String("package", packageName),
		slog.String("version", version))

	return ctrl.Result{}, nil
}

func (r *reconciler) extractVersionFromName(resourceName, repositoryName, packageName string) (string, error) {
	// Resource name format: <repo>-<package>-<version>
	prefix := repositoryName + "-" + packageName + "-"
	if len(resourceName) <= len(prefix) {
		return "", fmt.Errorf("invalid resource name format: %s", resourceName)
	}
	version := resourceName[len(prefix):]
	// Add 'v' prefix if not present
	if version[0] != 'v' {
		version = "v" + version
	}
	return version, nil
}

func (r *reconciler) extractMetadata(image crv1.Image, version string) *v1alpha1.ApplicationPackageVersionStatusMetadata {
	// In a real implementation, this would parse package.yaml, changelog.yaml, etc.
	// For now, return basic metadata
	metadata := &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Requirements: &v1alpha1.PackageRequirements{},
		Changelog:    &v1alpha1.PackageChangelog{},
	}

	// TODO: Parse actual metadata from image layers
	// This would involve:
	// 1. Extracting package.yaml, changelog.yaml, version.json from image layers
	// 2. Parsing and populating the metadata structure

	// For now, we can extract some basic info from image config
	configFile, err := image.ConfigFile()
	if err == nil && configFile != nil {
		// Extract labels if available
		if configFile.Config.Labels != nil {
			// Can extract metadata from labels here if needed
			_ = configFile.Config.Labels
		}
	}

	return metadata
}

func (r *reconciler) delete(_ context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	// TODO: implement application package version deletion logic
	r.logger.Info("deleting ApplicationPackageVersion", slog.String("name", packageVersion.Name))
	return ctrl.Result{}, nil
}

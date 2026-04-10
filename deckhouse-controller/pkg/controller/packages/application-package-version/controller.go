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
	"path/filepath"
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
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-package-version-controller"

	// maxConcurrentReconciles is set to 1 to serialize status and label patches,
	// preventing conflicts on the same ApplicationPackageVersion resource.
	maxConcurrentReconciles = 1

	defaultRequeue = 15 * time.Second
)

// reconciler promotes draft ApplicationPackageVersion resources by loading
// package metadata from the registry image and removing the draft label.
type reconciler struct {
	client   client.Client
	logger   *log.Logger
	registry *registry.Service
	dc       dependency.Container
}

// RegisterController creates and registers the ApplicationPackageVersion controller.
// It watches ApplicationPackageVersion resources and reconciles draft versions by
// fetching metadata from the package registry and promoting them to non-draft status.
func RegisterController(runtimeManager manager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		client:   runtimeManager.GetClient(),
		logger:   logger,
		registry: registry.NewService(dc, logger),
		dc:       dc,
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

// Reconcile handles a single ApplicationPackageVersion event. Draft resources
// are promoted by loading metadata; deleted resources have their finalizers removed
// once no Application references remain (usedByCount == 0).
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.logger.With(slog.String("name", req.Name))

	logger.Debug("reconcile resource")

	apv := new(v1alpha1.ApplicationPackageVersion)
	if err := r.client.Get(ctx, req.NamespacedName, apv); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("resource not found")

			return ctrl.Result{}, nil
		}

		logger.Warn("failed to get resource", log.Err(err))

		return ctrl.Result{}, err
	}

	// handle delete event
	if !apv.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, apv)
	}

	// handle create/update events
	if err := r.handleCreateOrUpdate(ctx, apv); err != nil {
		logger.Warn("failed to handle application package version", log.Err(err))

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleCreateOrUpdate processes draft ApplicationPackageVersions through a promotion pipeline:
//  1. Fetch the package image from the registry using the repository config
//  2. Extract metadata (package.yaml, changelog.yaml, version.json) from the image tar
//  3. Populate status.packageMetadata with the extracted information
//  4. Set the MetadataLoaded condition to True
//  5. Check if the package image exists in the registry and label accordingly
//  6. Add a finalizer and remove the draft label, completing promotion
//
// Non-draft resources are skipped since they have already been promoted.
func (r *reconciler) handleCreateOrUpdate(ctx context.Context, apv *v1alpha1.ApplicationPackageVersion) error {
	logger := r.logger.With(
		slog.String("name", apv.Name),
		slog.String("package", apv.Spec.PackageName),
		slog.String("version", apv.Spec.PackageVersion),
		slog.String("repository", apv.Spec.PackageRepositoryName))

	// Non-draft APVs have already been promoted — nothing to do.
	if !apv.IsDraft() {
		logger.Debug("package is not draft")

		return nil
	}

	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: apv.Spec.PackageRepositoryName}, repo); err != nil {
		original := apv.DeepCopy()
		r.setConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionReasonGetPackageRepoErr,
			fmt.Sprintf("failed to get repository '%s': %s", apv.Spec.PackageRepositoryName, err.Error()),
		)

		if err := r.client.Status().Patch(ctx, apv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status '%s': %w", apv.Name, err)
		}

		return fmt.Errorf("get repository '%s': %w", apv.Spec.PackageRepositoryName, err)
	}

	remote := registry.BuildRemote(repo)
	version := apv.Spec.PackageVersion
	versionPath := filepath.Join(apv.Spec.PackageName, "version")

	img, err := r.registry.GetImageReader(ctx, remote, versionPath, version)
	if err != nil {
		original := apv.DeepCopy()
		r.setConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionReasonGetImageErr,
			fmt.Sprintf("get image: %s", err.Error()),
		)

		if err := r.client.Status().Patch(ctx, apv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status '%s': %w", apv.Name, err)
		}

		return fmt.Errorf("get image for '%s': %w", apv.Name, err)
	}

	meta, err := r.parseVersionMetadataByImage(ctx, img)
	if err != nil {
		original := apv.DeepCopy()
		r.setConditionFalse(
			apv,
			v1alpha1.ApplicationPackageVersionConditionReasonFetchErr,
			fmt.Sprintf("fetch package metadata: %s", err.Error()),
		)

		if err := r.client.Status().Patch(ctx, apv, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("patch status '%s': %w", apv.Name, err)
		}

		return fmt.Errorf("fetch package metadata '%s': %w", apv.Name, err)
	}

	original := apv.DeepCopy()
	apv.Status.PackageMetadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{
		Stage: meta.definition.Stage,
		Description: &v1alpha1.PackageDescription{
			Ru: meta.definition.Descriptions.Ru,
			En: meta.definition.Descriptions.En,
		},
		Requirements: &v1alpha1.PackageRequirements{
			Deckhouse:  meta.definition.Requirements.Deckhouse,
			Kubernetes: meta.definition.Requirements.Kubernetes,
			Modules:    meta.definition.Requirements.Modules,
		},
		Changelog: &v1alpha1.PackageChangelog{
			Features: meta.changelog.Features,
			Fixes:    meta.changelog.Fixes,
		},
	}

	r.setConditionTrue(apv)

	if err = r.client.Status().Patch(ctx, apv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch status '%s': %w", apv.Name, err)
	}

	original = apv.DeepCopy()

	if apv.Labels == nil {
		apv.Labels = make(map[string]string)
	}

	// Check whether the package image exists in the registry and label accordingly.
	// The image may legitimately not exist (e.g. metadata-only bundle), so both outcomes are valid.
	if _, err = r.registry.GetImageDigest(ctx, remote, apv.Spec.PackageName, version); err != nil {
		apv.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry] = "false"
	} else {
		apv.Labels[v1alpha1.ApplicationPackageVersionLabelExistInRegistry] = "true"
	}

	// Finalizer prevents deletion while Applications reference this version.
	if !controllerutil.ContainsFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer) {
		controllerutil.AddFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer)
	}

	delete(apv.Labels, v1alpha1.ApplicationPackageVersionLabelDraft)

	if err = r.client.Patch(ctx, apv, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch '%s': %w", apv.Name, err)
	}

	return nil
}

// handleDelete removes the finalizer from the ApplicationPackageVersion once it is
// no longer referenced by any Application (usedByCount == 0). While references exist,
// the reconcile is requeued every 15 seconds to wait for Applications to release the APV.
func (r *reconciler) handleDelete(ctx context.Context, apv *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	logger := r.logger.With(
		slog.String("name", apv.Name),
		slog.String("package", apv.Spec.PackageName),
		slog.String("version", apv.Spec.PackageVersion),
		slog.String("repository", apv.Spec.PackageRepositoryName))

	if apv.Status.UsedByCount > 0 {
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	if controllerutil.ContainsFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer) {
		logger.Debug("removing finalizer from application package version")

		original := apv.DeepCopy()

		controllerutil.RemoveFinalizer(apv, v1alpha1.ApplicationPackageVersionFinalizer)

		if err := r.client.Patch(ctx, apv, client.MergeFrom(original)); err != nil {
			logger.Warn("failed to remove finalizer", log.Err(err))

			return ctrl.Result{}, fmt.Errorf("remove finalizer from '%s': %w", apv.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

// setConditionTrue sets the given condition to True, clearing reason and message.
func (r *reconciler) setConditionTrue(apv *v1alpha1.ApplicationPackageVersion) {
	metautils.SetStatusCondition(&apv.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded,
		Status:             metav1.ConditionTrue,
		Reason:             "Succeeded",
		ObservedGeneration: apv.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})
}

// setConditionFalse sets the given condition to False with a reason and message.
func (r *reconciler) setConditionFalse(apv *v1alpha1.ApplicationPackageVersion, reason, message string) {
	metautils.SetStatusCondition(&apv.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ApplicationPackageVersionConditionTypeMetadataLoaded,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: apv.Generation,
		LastTransitionTime: metav1.NewTime(r.dc.GetClock().Now()),
	})
}

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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
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

	// skip handle for non drafted resources
	if packageVersion.Labels["draft"] != "true" {
		r.logger.Debug("package is not draft", slog.String("package_name", packageVersion.Name))
		return ctrl.Result{}, nil
	}

	// handle create/update events
	return r.handle(ctx, packageVersion)
}

func (r *reconciler) handle(ctx context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	// TODO: implement application package version reconciliation logic
	r.logger.Info("handling ApplicationPackageVersion", slog.String("name", packageVersion.Name))

	// - get registry creds from PackageRepository resource
	var pr v1alpha1.PackageRepository
	packageName := packageVersion.Labels["registry"]
	err := r.client.Get(ctx, types.NamespacedName{Name: packageName}, &pr)
	if err != nil {
		r.logger.Error("get packageVersion", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get packageVersion: %w", err)
	}

	// - create go registry client from creds from PackageRepository
	// example path: registry.deckhouse.io/sys/deckhouse-oss/packages/redis/release-channel
	registryPath := path.Join(pr.Spec.Registry.Repo, packageVersion.Labels["package"], "release-channel")
	opts := []cr.Option{
		cr.WithAuth(pr.Spec.Registry.DockerCFG),
		// cr.WithUserAgent(ri.UserAgent),
		cr.WithCA(pr.Spec.Registry.CA),
		cr.WithInsecureSchema(strings.ToLower(pr.Spec.Registry.Scheme) == "http"),
	}
	registryClient, err := r.dc.GetRegistryClient(registryPath, opts...)
	if err != nil {
		r.logger.Error("get registry client", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get registry client: %w", err)
	}

	// - get package.yaml from release image
	_, err = registryClient.Image(ctx, "stable")
	if err != nil {
		r.logger.Error("get release image", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get release image: %w", err)
	}

	// - fill subresource status with new data
	packageVersion.Status.PackageName = packageName
	packageVersion.Status.Version = "" // TODO

	packageVersion.Status.Metadata = &v1alpha1.ApplicationPackageVersionStatusMetadata{} // from package.yaml

	// - delete label "draft"
	delete(packageVersion.Labels, "draft")
	err = r.client.Update(ctx, packageVersion)
	if err != nil {
		r.logger.Error("update packageVersion", log.Err(err))
		return ctrl.Result{}, fmt.Errorf("update packageVersion: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) delete(_ context.Context, packageVersion *v1alpha1.ApplicationPackageVersion) (ctrl.Result, error) {
	// TODO: implement application package version deletion logic
	r.logger.Info("deleting ApplicationPackageVersion", slog.String("name", packageVersion.Name))
	return ctrl.Result{}, nil
}

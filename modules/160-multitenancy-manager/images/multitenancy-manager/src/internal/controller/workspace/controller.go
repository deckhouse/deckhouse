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

package template

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha1"
	workspacemanager "controller/internal/manager/workspace"
)

const controllerName = "d8-workspace-controller"

func Register(runtimeManager manager.Manager, logger logr.Logger) error {
	r := &reconciler{
		logger:           logger.WithName(controllerName),
		client:           runtimeManager.GetClient(),
		workspaceManager: workspacemanager.New(runtimeManager.GetClient(), logger),
	}

	workspaceController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		return fmt.Errorf("create workspace controller: %w", err)
	}

	r.logger.Info("initialize workspace controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.Workspace{}).
		WithEventFilter(predicate.Or[client.Object](
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{})).
		Complete(workspaceController)
}

var _ reconcile.Reconciler = &reconciler{}

type reconciler struct {
	workspaceManager *workspacemanager.Manager
	client           client.Client
	logger           logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Info("reconcile the workspace", "workspace", req.Name)
	workspace := new(v1alpha1.Workspace)
	if err := r.client.Get(ctx, req.NamespacedName, workspace); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("the workspace not found", "workspace", req.Name)
			return reconcile.Result{}, nil
		}
		r.logger.Error(err, "failed to get the workspace", "workspace", req.Name)
		return reconcile.Result{}, err
	}

	// handle the project template deletion
	if !workspace.DeletionTimestamp.IsZero() {
		r.logger.Info("the workspace deleted", "workspace", workspace.Name)
		return r.workspaceManager.Delete(ctx, workspace)
	}

	// ensure template
	r.logger.Info("ensure the workspace", "workspace", workspace.Name)
	return r.workspaceManager.Handle(ctx, workspace)
}

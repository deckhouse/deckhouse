/*
Copyright 2026 Flant JSC

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

package capi

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	obj := newUnstructured("infrastructure.cluster.x-k8s.io", "v1alpha1", "DeckhouseControlPlane")
	register.RegisterController("capi-control-plane", obj, &ControlPlaneReconciler{})
}

type ControlPlaneReconciler struct {
	register.Base
}

func (r *ControlPlaneReconciler) SetupWatches(_ register.Watcher) {}

func (r *ControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	obj := newUnstructured("infrastructure.cluster.x-k8s.io", "v1alpha1", "DeckhouseControlPlane")
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get DeckhouseControlPlane: %w", err)
	}

	original := obj.DeepCopy()
	obj.Object["status"] = map[string]interface{}{
		"initialized":                 true,
		"ready":                       true,
		"externalManagedControlPlane": true,
		"initialization": map[string]interface{}{
			"controlPlaneInitialized": true,
		},
	}

	if err := r.Client.Status().Patch(ctx, obj, client.MergeFrom(original)); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("patch DeckhouseControlPlane status: %w", err)
	}

	logger.Info("patched DeckhouseControlPlane status", "name", obj.GetName())
	return ctrl.Result{}, nil
}

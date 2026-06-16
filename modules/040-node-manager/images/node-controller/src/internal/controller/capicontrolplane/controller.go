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

package capicontrolplane

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1alpha1",
		Kind:    "DeckhouseControlPlane",
	})
	register.RegisterController("capi-control-plane", obj, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1alpha1",
		Kind:    "DeckhouseControlPlane",
	})

	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get DeckhouseControlPlane: %w", err)
	}

	// Patch status: ready, initialized, externalManagedControlPlane.
	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(obj.GroupVersionKind())
	patch.SetName(obj.GetName())
	patch.SetNamespace(obj.GetNamespace())
	patch.Object["status"] = map[string]interface{}{
		"initialized":                 true,
		"ready":                       true,
		"externalManagedControlPlane": true,
		"initialization": map[string]interface{}{
			"controlPlaneInitialized": true,
		},
	}

	if err := r.Client.Status().Patch(ctx, patch, client.MergeFrom(obj)); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("patch DeckhouseControlPlane status: %w", err)
	}

	logger.V(1).Info("patched DeckhouseControlPlane status", "name", obj.GetName())
	return ctrl.Result{}, nil
}

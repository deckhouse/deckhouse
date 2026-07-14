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
	"slices"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/node-controller/internal/register"
)

const capiControllerManagerFinalizer = "deckhouse.io/capi-controller-manager"

func init() {
	obj := newUnstructured("cluster.x-k8s.io", "v1beta2", "Cluster")
	register.RegisterController("capi-finalizer-cleanup", obj, &FinalizerReconciler{})
}

type FinalizerReconciler struct {
	register.Base
}

func (r *FinalizerReconciler) SetupWatches(_ register.Watcher) {}

func (r *FinalizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	obj := newUnstructured("cluster.x-k8s.io", "v1beta2", "Cluster")
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Cluster: %w", err)
	}

	finalizers := obj.GetFinalizers()
	cleaned := slices.DeleteFunc(append([]string(nil), finalizers...), func(v string) bool {
		return v == capiControllerManagerFinalizer
	})
	if len(cleaned) == len(finalizers) {
		return ctrl.Result{}, nil
	}

	original := obj.DeepCopy()
	obj.SetFinalizers(cleaned)
	if err := r.Client.Patch(ctx, obj, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer from Cluster %s: %w", req.Name, err)
	}

	logger.Info("removed capi-controller-manager finalizer", "cluster", req.Name)
	return ctrl.Result{}, nil
}

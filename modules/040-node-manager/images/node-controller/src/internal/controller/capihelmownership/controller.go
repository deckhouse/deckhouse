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

package capihelmownership

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/node-controller/internal/register"
)

var (
	clusterGVK = schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster",
	}
	mhcGVK = schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineHealthCheck",
	}
)

func init() {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(clusterGVK)
	register.RegisterController("capi-helm-ownership", obj, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	mhc := &unstructured.Unstructured{}
	mhc.SetGroupVersionKind(mhcGVK)
	w.Watches(mhc, handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(obj)}}
		},
	))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	for _, gvk := range []schema.GroupVersionKind{clusterGVK, mhcGVK} {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return ctrl.Result{}, fmt.Errorf("get %s %s: %w", gvk.Kind, req.NamespacedName, err)
		}

		if err := r.ensureKeepAnnotation(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}

		logger.V(1).Info("checked helm ownership", "kind", gvk.Kind, "name", obj.GetName())
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureKeepAnnotation(ctx context.Context, obj *unstructured.Unstructured) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	// Only patch if resource is helm-owned and doesn't have keep annotation yet.
	if _, hasHelm := annotations["meta.helm.sh/release-name"]; !hasHelm {
		return nil
	}
	if _, hasKeep := annotations["helm.sh/resource-policy"]; hasKeep {
		return nil
	}

	patch := client.MergeFrom(obj.DeepCopy())
	annotations["helm.sh/resource-policy"] = "keep"
	obj.SetAnnotations(annotations)

	if err := r.Client.Patch(ctx, obj, patch); err != nil {
		return fmt.Errorf("patch helm keep annotation on %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}

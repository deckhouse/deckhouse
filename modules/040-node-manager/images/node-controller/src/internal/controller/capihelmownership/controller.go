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

// Package capihelmownership detaches the CAPI Cluster and MachineHealthCheck
// objects from helm ownership by stamping `helm.sh/resource-policy: keep`.
//
// These objects used to be rendered by the node-manager helm chart and are now
// owned by node-controller. Without the keep annotation, helm would delete them
// on upgrade (they vanished from the rendered manifest), cascading into
// capi-controller-manager tearing down Machines / Nodes. The reconcile is
// idempotent: it only patches objects still claimed by helm that miss the
// annotation.
package capihelmownership

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	helmReleaseNameAnnotation     = "meta.helm.sh/release-name"
	helmResourcePolicyAnnotation  = "helm.sh/resource-policy"
	helmResourcePolicyKeep        = "keep"
	keepAnnotationMergePatchBytes = `{"metadata":{"annotations":{"` + helmResourcePolicyAnnotation + `":"` + helmResourcePolicyKeep + `"}}}`
)

var capiResourceGVKs = []schema.GroupVersionKind{
	{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"},
	{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineHealthCheck"},
}

type reconciler struct {
	register.Base
	gvk schema.GroupVersionKind
}

var _ register.Reconciler = (*reconciler)(nil)

func (r *reconciler) SetupWatches(_ register.Watcher) {}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.gvk)
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	annotations := obj.GetAnnotations()
	if _, claimedByHelm := annotations[helmReleaseNameAnnotation]; !claimedByHelm {
		return ctrl.Result{}, nil
	}
	if _, hasKeep := annotations[helmResourcePolicyAnnotation]; hasKeep {
		return ctrl.Result{}, nil
	}

	patchTarget := &unstructured.Unstructured{}
	patchTarget.SetGroupVersionKind(r.gvk)
	patchTarget.SetNamespace(obj.GetNamespace())
	patchTarget.SetName(obj.GetName())
	if err := r.Client.Patch(ctx, patchTarget, client.RawPatch(types.MergePatchType, []byte(keepAnnotationMergePatchBytes))); err != nil {
		return ctrl.Result{}, fmt.Errorf("stamp keep annotation on %s %s: %w", r.gvk.Kind, req.Name, err)
	}

	return ctrl.Result{}, nil
}

func init() {
	for _, gvk := range capiResourceGVKs {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		register.RegisterController("capi-helm-ownership-"+strings.ToLower(gvk.Kind), obj, &reconciler{gvk: gvk})
	}
}

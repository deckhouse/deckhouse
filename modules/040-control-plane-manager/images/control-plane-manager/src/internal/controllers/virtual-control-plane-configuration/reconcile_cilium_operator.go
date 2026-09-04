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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const ciliumOperatorManifestKey = "cilium-operator.yaml.tpl"

func (r *reconciler) reconcileCiliumOperator(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	if err := r.applyParentManifests(ctx, vcp, configSecret, ciliumOperatorManifestKey); err != nil {
		return reconcile.Result{}, fmt.Errorf("apply parent %s: %w", ciliumOperatorManifestKey, err)
	}

	return reconcile.Result{}, nil
}

// applyParentManifests applies a multi-doc template into the parent cluster, owned by the VCP.
func (r *reconciler) applyParentManifests(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
	key string,
) error {
	raw, ok := configSecret.Data[key]
	if !ok {
		return fmt.Errorf("config Secret missing %q", key)
	}

	objects, err := parseManifestDocs(raw, "")
	if err != nil {
		return err
	}

	for _, target := range objects {
		if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
			return err
		}
		if err := applyObject(ctx, r.client, target, patchWholeObject); err != nil {
			return err
		}
	}

	return nil
}

// patchWholeObject patches the full target, carrying over the identity fields required by MergeFrom.
func patchWholeObject(current, target *unstructured.Unstructured) (client.Object, bool) {
	target.SetResourceVersion(current.GetResourceVersion())
	target.SetUID(current.GetUID())
	return target, true
}

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
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
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

	for _, doc := range strings.Split(string(raw), "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		target := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), target); err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}
		if len(target.Object) == 0 {
			continue
		}
		if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
			return err
		}

		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(target.GroupVersionKind())
		err := r.client.Get(ctx, client.ObjectKeyFromObject(target), current)
		if apierrors.IsNotFound(err) {
			if err := r.client.Create(ctx, target); err != nil {
				return fmt.Errorf("create %s %s: %w", target.GetKind(), target.GetName(), err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("get %s %s: %w", target.GetKind(), target.GetName(), err)
		}

		base := current.DeepCopy()
		target.SetResourceVersion(current.GetResourceVersion())
		target.SetUID(current.GetUID())
		if err := r.client.Patch(ctx, target, client.MergeFrom(base)); err != nil {
			return fmt.Errorf("patch %s %s: %w", target.GetKind(), target.GetName(), err)
		}
	}

	return nil
}

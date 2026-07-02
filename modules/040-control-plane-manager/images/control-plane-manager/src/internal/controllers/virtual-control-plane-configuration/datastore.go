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
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const (
	datastoreReadyRequeue = 5 * time.Second
	datastoreManifestKey  = "datastore.yaml.tpl"
)

func (r *reconciler) reconcilePostgres(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, configSecret *corev1.Secret) (reconcile.Result, error) {
	target, err := loadDatastoreManifest(configSecret, vcp)
	if err != nil {
		return reconcile.Result{}, err
	}

	current := newPostgres()
	err = r.client.Get(ctx, client.ObjectKeyFromObject(target), current)
	if apierrors.IsNotFound(err) {
		if err := r.client.Create(ctx, target); err != nil {
			return reconcile.Result{}, fmt.Errorf("create Postgres: %w", err)
		}
		return reconcile.Result{RequeueAfter: datastoreReadyRequeue}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get Postgres: %w", err)
	}

	if !isPostgresAvailable(current) {
		return reconcile.Result{RequeueAfter: datastoreReadyRequeue}, nil
	}

	return reconcile.Result{}, nil
}

func newPostgres() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("managed-services.deckhouse.io/v1alpha1")
	obj.SetKind("Postgres")
	return obj
}

func loadDatastoreManifest(configSecret *corev1.Secret, vcp *controlplanev1alpha1.VirtualControlPlane) (*unstructured.Unstructured, error) {
	raw, ok := configSecret.Data[datastoreManifestKey]
	if !ok {
		return nil, fmt.Errorf("config Secret missing %q", datastoreManifestKey)
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(raw, obj); err != nil {
		return nil, fmt.Errorf("decode datastore manifest: %w", err)
	}
	obj.SetNamespace(constants.VirtualControlPlaneNamespacePrefix + vcp.Name)

	return obj, nil
}

func isPostgresAvailable(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if condition["type"] == "Available" && condition["status"] == "True" {
			return true
		}
	}
	return false
}

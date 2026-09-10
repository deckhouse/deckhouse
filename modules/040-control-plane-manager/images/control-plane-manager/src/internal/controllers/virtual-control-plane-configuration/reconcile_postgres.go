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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const (
	datastoreReadyRequeue  = 5 * time.Second
	datastoreManifestKey   = "datastore.yaml.tpl"
	datastoreHAManifestKey = "datastore-ha.yaml.tpl"
)

func (r *reconciler) reconcilePostgres(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, configSecret *corev1.Secret) (reconcile.Result, error) {
	target, err := buildTargetPostgres(configSecret, vcp)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	current := postgres()
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

	if ownerReferencesDiffer(current, target) {
		base := current.DeepCopy()
		syncOwnerReferences(current, target)
		if err := r.client.Patch(ctx, current, client.MergeFrom(base)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patch Postgres ownerRefs: %w", err)
		}
	}

	shapeBase := current.DeepCopy()
	shapeChanged, err := syncPostgresShape(current, target)
	if err != nil {
		return reconcile.Result{}, err
	}
	if shapeChanged {
		if err := r.client.Patch(ctx, current, client.MergeFrom(shapeBase)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patch Postgres shape: %w", err)
		}
		// Resizing the datastore takes a while; come back instead of declaring it Available.
		return reconcile.Result{RequeueAfter: datastoreReadyRequeue}, nil
	}

	if !isPostgresAvailable(current) {
		return reconcile.Result{RequeueAfter: datastoreReadyRequeue}, nil
	}

	return reconcile.Result{}, nil
}

func postgres() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("managed-services.deckhouse.io/v1alpha1")
	obj.SetKind("Postgres")
	return obj
}

// datastoreManifestKeyFor picks the datastore shape. Two templates rather than one parameterised
// one because spec.cluster has to appear and disappear as a whole - the operator's CEL forbids it
// on a Standalone - and the Replacer cannot do conditional blocks.
func datastoreManifestKeyFor(vcp *controlplanev1alpha1.VirtualControlPlane) string {
	if vcp.Spec.HighAvailability {
		return datastoreHAManifestKey
	}
	return datastoreManifestKey
}

func buildTargetPostgres(configSecret *corev1.Secret, vcp *controlplanev1alpha1.VirtualControlPlane) (*unstructured.Unstructured, error) {
	key := datastoreManifestKeyFor(vcp)
	raw, ok := configSecret.Data[key]
	if !ok {
		return nil, fmt.Errorf("config Secret missing %q", key)
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(raw, obj); err != nil {
		return nil, fmt.Errorf("decode datastore manifest: %w", err)
	}
	obj.SetNamespace(vcp.Namespace)

	return obj, nil
}

// syncPostgresShape brings only the fields this module owns to the desired state.
//
// A whole-spec comparison would fight the operator's mutating webhook: it generates
// spec.users[].password for users with storeCredsToSecret, so the live spec always carries a field
// the rendered manifest does not. Every reconcile would patch it away and trigger a fresh password.
func syncPostgresShape(current, target *unstructured.Unstructured) (bool, error) {
	targetType, _, err := unstructured.NestedString(target.Object, "spec", "type")
	if err != nil {
		return false, fmt.Errorf("read target spec.type: %w", err)
	}
	currentType, _, err := unstructured.NestedString(current.Object, "spec", "type")
	if err != nil {
		return false, fmt.Errorf("read current spec.type: %w", err)
	}

	targetCluster, targetHasCluster, err := unstructured.NestedMap(target.Object, "spec", "cluster")
	if err != nil {
		return false, fmt.Errorf("read target spec.cluster: %w", err)
	}
	currentCluster, currentHasCluster, err := unstructured.NestedMap(current.Object, "spec", "cluster")
	if err != nil {
		return false, fmt.Errorf("read current spec.cluster: %w", err)
	}

	if currentType == targetType &&
		currentHasCluster == targetHasCluster &&
		equality.Semantic.DeepEqual(currentCluster, targetCluster) {
		return false, nil
	}

	if err := unstructured.SetNestedField(current.Object, targetType, "spec", "type"); err != nil {
		return false, fmt.Errorf("set spec.type: %w", err)
	}
	// The operator's CEL forbids spec.cluster on a Standalone, so going back removes the block
	// rather than blanking it.
	if targetHasCluster {
		if err := unstructured.SetNestedMap(current.Object, targetCluster, "spec", "cluster"); err != nil {
			return false, fmt.Errorf("set spec.cluster: %w", err)
		}
	} else {
		unstructured.RemoveNestedField(current.Object, "spec", "cluster")
	}

	return true, nil
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

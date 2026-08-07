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

package common

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// RegisteredInstanceClassGVKs returns the GVK every cloud provider registered its InstanceClass
// under: for each registration Secret, the instanceClassKind it names at the
// instanceClassAPIVersion it declares. Registrations are found by label rather than by the fixed
// legacy name, so a cluster with several providers yields every provider's kind at that
// provider's own version.
//
// A registration without the version contributes nothing — guessing a version is what this whole
// mechanism exists to prevent (see InstanceClassAPIVersionKey). The CRD may lag the Secret:
// callers hand the GVK to a watch that waits for it (source.Kind retries an unserved kind
// itself), they must not assume it is served.
func RegisteredInstanceClassGVKs(ctx context.Context, r client.Reader) ([]schema.GroupVersionKind, error) {
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets, client.InNamespace(CloudProviderSecretNamespace),
		client.HasLabels{CloudProviderRegistrationLabel}); err != nil {
		return nil, fmt.Errorf("list cloud provider registration secrets: %w", err)
	}

	seen := map[schema.GroupVersionKind]bool{}
	var gvks []schema.GroupVersionKind
	for i := range secrets.Items {
		kind := string(secrets.Items[i].Data[InstanceClassKindKey])
		version := string(secrets.Items[i].Data[InstanceClassAPIVersionKey])
		if kind == "" || version == "" {
			continue
		}

		gvk := schema.GroupVersionKind{Group: v1.GroupVersion.Group, Version: version, Kind: kind}
		// Providers publish the same registration twice, under the legacy fixed name and under
		// the per-provider one.
		if seen[gvk] {
			continue
		}
		seen[gvk] = true
		gvks = append(gvks, gvk)
	}

	sort.Slice(gvks, func(i, j int) bool {
		if gvks[i].Version != gvks[j].Version {
			return gvks[i].Version < gvks[j].Version
		}
		return gvks[i].Kind < gvks[j].Kind
	})
	return gvks, nil
}

// InstanceClassToNodeGroups maps an InstanceClass event to the NodeGroups whose classReference
// points at it. Matching on both kind and name keeps an edit of one provider's class from
// re-rendering NodeGroups that reference another.
func InstanceClassToNodeGroups(ctx context.Context, r client.Reader, obj client.Object) []reconcile.Request {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	ngList := &v1.NodeGroupList{}
	if err := r.List(ctx, ngList); err != nil {
		log.FromContext(ctx).Error(err, "list nodegroups for instance class event", "kind", u.GetKind(), "name", u.GetName())
		return nil
	}

	requests := make([]reconcile.Request, 0, 1)
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if ng.Spec.CloudInstances == nil {
			continue
		}
		ref := ng.Spec.CloudInstances.ClassReference
		if ref.Kind == u.GetKind() && ref.Name == u.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ng.Name}})
		}
	}
	return requests
}

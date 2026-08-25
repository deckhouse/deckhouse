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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

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

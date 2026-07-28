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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

const instanceClassKindSuffix = "InstanceClass"

// ServedInstanceClassKinds discovers the InstanceClass kinds the cluster actually serves, so the
// controllers can watch them. The kind is provider-specific (AWSInstanceClass, DVPInstanceClass,
// …) and therefore cannot be compiled in; discovery is done once at controller setup.
//
// Without these watches an InstanceClass edit reaches the nodes only on the next resync — up to
// ten minutes during which the rendered MachineClass, the machine template and the bashible
// context all describe the previous instance type. The helm implementation reacted immediately.
//
// Watching costs no extra informer: the derived-status service already reads these objects
// through the cached unstructured client, so the informer exists either way.
func ServedInstanceClassKinds(cfg *rest.Config) ([]schema.GroupVersionKind, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build discovery client: %w", err)
	}

	// Partial discovery is normal (an aggregated API can be momentarily unavailable) and must
	// not cost us the watches for the groups that did answer.
	lists, err := dc.ServerPreferredResources()
	if err != nil && len(lists) == 0 {
		return nil, fmt.Errorf("list served resources: %w", err)
	}

	var gvks []schema.GroupVersionKind
	for _, list := range lists {
		gv, parseErr := schema.ParseGroupVersion(list.GroupVersion)
		if parseErr != nil || gv.Group != v1.GroupVersion.Group {
			continue
		}
		for _, res := range list.APIResources {
			if strings.HasSuffix(res.Kind, instanceClassKindSuffix) && res.Kind != instanceClassKindSuffix {
				gvks = append(gvks, gv.WithKind(res.Kind))
			}
		}
	}
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

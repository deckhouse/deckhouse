/*
Copyright 2025 Flant JSC

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

package instance

import (
	"context"

	nodecontroller "github.com/deckhouse/node-controller/internal/controller/node"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func mapObjectNameToInstance(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: obj.GetName()}},
	}
}

func staticNodeEventPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			return ok && nodecontroller.IsStaticNode(node)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			return ok && nodecontroller.IsStaticNode(node)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			return ok && nodecontroller.IsStaticNode(node)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, oldOK := e.ObjectOld.(*corev1.Node)
			newNode, newOK := e.ObjectNew.(*corev1.Node)
			if !oldOK || !newOK {
				return false
			}

			if !apiequality.Semantic.DeepEqual(oldNode.Labels, newNode.Labels) {
				return nodecontroller.IsStaticNode(oldNode) || nodecontroller.IsStaticNode(newNode)
			}

			return false
		},
	}
}

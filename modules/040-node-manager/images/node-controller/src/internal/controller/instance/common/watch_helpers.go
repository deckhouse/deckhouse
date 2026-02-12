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

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// MapObjectNameToInstance maps any object to a reconcile.Request for an Instance
// with the same name. Used to trigger Instance reconcile when a linked Machine or Node changes.
func MapObjectNameToInstance(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: obj.GetName()}},
	}
}

// StaticNodeEventPredicate filters Node events to only pass through events
// for static (non-cloud-ephemeral) nodes, i.e. nodes without a CAPI machine annotation
// and with type Static or CloudPermanent.
func StaticNodeEventPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			return ok && IsStaticNode(node)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			return ok && IsStaticNode(node)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			return ok && IsStaticNode(node)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, oldOK := e.ObjectOld.(*corev1.Node)
			newNode, newOK := e.ObjectNew.(*corev1.Node)
			if !oldOK || !newOK {
				return false
			}

			oldStatic := IsStaticNode(oldNode)
			newStatic := IsStaticNode(newNode)
			if oldStatic != newStatic {
				return true
			}
			if !oldStatic {
				return false
			}
			return !apiequality.Semantic.DeepEqual(oldNode.Labels, newNode.Labels)
		},
	}
}

// IsStaticNode returns true if the node is a static or cloud-permanent node
// that is not managed by a CAPI Machine.
func IsStaticNode(node *corev1.Node) bool {
	if _, hasCAPIMachineAnnotation := node.Annotations[nodecommon.CAPIMachineAnnotation]; hasCAPIMachineAnnotation {
		return false
	}

	nodeType := deckhousev1.NodeType(node.Labels[nodecommon.NodeTypeLabel])
	return nodeType == deckhousev1.NodeTypeStatic || nodeType == deckhousev1.NodeTypeCloudPermanent
}

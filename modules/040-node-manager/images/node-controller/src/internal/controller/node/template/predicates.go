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

package template

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// nodeTemplatePredicate triggers reconciliation when node labels, annotations,
// or taints change. It also passes through create events for nodes that belong
// to a node group.
func nodeTemplatePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasNodeGroupLabel(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !hasNodeGroupLabel(e.ObjectNew) {
				return false
			}
			return labelsChanged(e) || annotationsChanged(e) || taintsChanged(e)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

// nodeGroupTemplateChangedPredicate triggers when the NodeGroup spec.nodeTemplate
// or spec.nodeType changes.
func nodeGroupTemplateChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNG, ok1 := e.ObjectOld.(*deckhousev1.NodeGroup)
			newNG, ok2 := e.ObjectNew.(*deckhousev1.NodeGroup)
			if !ok1 || !ok2 {
				return false
			}
			return !reflect.DeepEqual(oldNG.Spec.NodeTemplate, newNG.Spec.NodeTemplate) ||
				oldNG.Spec.NodeType != newNG.Spec.NodeType
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

func hasNodeGroupLabel(obj client.Object) bool {
	_, ok := obj.GetLabels()[nodeGroupNameLabel]
	return ok
}

func labelsChanged(e event.UpdateEvent) bool {
	return !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
}

func annotationsChanged(e event.UpdateEvent) bool {
	return !reflect.DeepEqual(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations())
}

func taintsChanged(e event.UpdateEvent) bool {
	oldNode, ok1 := e.ObjectOld.(*corev1.Node)
	newNode, ok2 := e.ObjectNew.(*corev1.Node)
	if !ok1 || !ok2 {
		return false
	}
	return !reflect.DeepEqual(oldNode.Spec.Taints, newNode.Spec.Taints)
}

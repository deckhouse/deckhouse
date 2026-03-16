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

package update

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// updateRelevantAnnotations lists the annotations relevant for the update workflow.
// The predicate only triggers reconciliation when one of these changes.
var updateRelevantAnnotations = []string{
	annotationWaitingForApproval,
	annotationApproved,
	annotationDisruptionRequired,
	annotationDisruptionApproved,
	annotationDraining,
	annotationDrained,
	annotationRollingUpdate,
	annotationConfigChecksum,
}

// nodeUpdatePredicate triggers reconciliation for Node events relevant to the
// update workflow: creation of nodes with a group label, annotation changes on
// update-related keys, readiness condition changes, and unschedulable field changes.
func nodeUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasNodeGroupLabelObj(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !hasNodeGroupLabelObj(e.ObjectNew) {
				return false
			}
			return updateAnnotationsChanged(e) ||
				nodeReadinessChanged(e) ||
				unschedulableChanged(e)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

// nodeGroupDisruptionOrUpdatePredicate triggers reconciliation when the NodeGroup
// disruption settings, update settings, status, or configuration checksum change.
func nodeGroupDisruptionOrUpdatePredicate() predicate.Predicate {
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
			return !reflect.DeepEqual(oldNG.Spec.Disruptions, newNG.Spec.Disruptions) ||
				!reflect.DeepEqual(oldNG.Spec.Update, newNG.Spec.Update) ||
				!reflect.DeepEqual(oldNG.Status, newNG.Status) ||
				configChecksumAnnotationChanged(oldNG, newNG)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

// hasNodeGroupLabelObj checks whether the object carries the node.deckhouse.io/group label.
func hasNodeGroupLabelObj(obj client.Object) bool {
	_, ok := obj.GetLabels()[nodeGroupLabel]
	return ok
}

// updateAnnotationsChanged returns true if any update-relevant annotation changed between
// the old and new objects.
func updateAnnotationsChanged(e event.UpdateEvent) bool {
	oldAnns := e.ObjectOld.GetAnnotations()
	newAnns := e.ObjectNew.GetAnnotations()

	for _, key := range updateRelevantAnnotations {
		if oldAnns[key] != newAnns[key] {
			return true
		}
	}
	return false
}

// nodeReadinessChanged returns true if the NodeReady condition status changed.
func nodeReadinessChanged(e event.UpdateEvent) bool {
	oldNode, ok1 := e.ObjectOld.(*corev1.Node)
	newNode, ok2 := e.ObjectNew.(*corev1.Node)
	if !ok1 || !ok2 {
		return false
	}
	return getNodeReadyStatus(oldNode) != getNodeReadyStatus(newNode)
}

// unschedulableChanged returns true if the node unschedulable field changed.
func unschedulableChanged(e event.UpdateEvent) bool {
	oldNode, ok1 := e.ObjectOld.(*corev1.Node)
	newNode, ok2 := e.ObjectNew.(*corev1.Node)
	if !ok1 || !ok2 {
		return false
	}
	return oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable
}

// getNodeReadyStatus returns the NodeReady condition status for a node.
func getNodeReadyStatus(node *corev1.Node) corev1.ConditionStatus {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status
		}
	}
	return corev1.ConditionUnknown
}

// configChecksumAnnotationChanged returns true if the configuration-checksum
// annotation on the NodeGroup changed.
func configChecksumAnnotationChanged(oldNG, newNG *deckhousev1.NodeGroup) bool {
	oldChecksum := oldNG.Annotations[annotationConfigChecksum]
	newChecksum := newNG.Annotations[annotationConfigChecksum]
	return oldChecksum != newChecksum
}

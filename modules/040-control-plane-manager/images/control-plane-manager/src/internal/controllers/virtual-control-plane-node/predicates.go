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

package virtualcontrolplanenode

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// controlPlaneNodePredicates filters ControlPlaneNode events for virtual nodes with spec changed.
func controlPlaneNodePredicates() ([]predicate.Predicate, error) {
	labelPred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneTypeLabelKey: string(constants.ControlPlaneTypeVirtual),
		},
	})
	if err != nil {
		return nil, err
	}
	return []predicate.Predicate{labelPred, predicate.GenerationChangedPredicate{}}, nil
}

// controlPlaneOperationPredicates filters owned ControlPlaneOperation events for virtual cpo with status changed.
func controlPlaneOperationPredicates() ([]predicate.Predicate, error) {
	labelPred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneTypeLabelKey: string(constants.ControlPlaneTypeVirtual),
		},
	})
	if err != nil {
		return nil, err
	}
	statusChanged := predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
			if !okOld || !okNew {
				return false
			}
			return !reflect.DeepEqual(oldOp.Status, newOp.Status)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return []predicate.Predicate{labelPred, statusChanged}, nil
}

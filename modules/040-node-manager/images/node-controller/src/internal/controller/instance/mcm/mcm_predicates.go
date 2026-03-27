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

package mcm

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

// MapInstanceToMCMMachine returns a MapFunc that maps an Instance event to a reconcile.Request
// for the MCM Machine referenced by the Instance.
func MapInstanceToMCMMachine() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		instance, ok := obj.(*deckhousev1alpha2.Instance)
		if !ok {
			return nil
		}

		ref := instance.Spec.MachineRef
		if ref == nil || ref.Name == "" {
			return nil
		}
		if ref.Kind != "" && ref.Kind != "Machine" {
			return nil
		}
		if ref.APIVersion != mcmv1alpha1.SchemeGroupVersion.String() {
			return nil
		}

		namespace := ref.Namespace
		if namespace == "" {
			namespace = machine.MachineNamespace
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      ref.Name,
			},
		}}
	}
}

// InstanceWatchPredicate returns a predicate that filters Instance events for MCM-backed instances.
// Update events are only passed when MCM-owned status fields are missing (self-heal mode).
func InstanceWatchPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return IsMCMMachineRef(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return IsMCMMachineRef(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			instance, ok := e.ObjectNew.(*deckhousev1alpha2.Instance)
			if !ok || !IsMCMMachineRef(instance) {
				return false
			}

			// self-heal - status is empty
			if instance.Status.Phase == "" || instance.Status.MachineStatus == "" {
				return true
			}

			// self-heal - MachineReady condition is absent
			_, hasMachineReady := instancecommon.GetInstanceConditionByType(
				instance.Status.Conditions,
				deckhousev1alpha2.InstanceConditionTypeMachineReady,
			)
			return !hasMachineReady
		},
	}
}

// IsMCMMachineRef returns true if obj is an Instance with an MCM Machine ref.
func IsMCMMachineRef(obj client.Object) bool {
	instance, ok := obj.(*deckhousev1alpha2.Instance)
	if !ok || instance == nil {
		return false
	}

	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return false
	}
	if ref.Kind != "" && ref.Kind != "Machine" {
		return false
	}

	return ref.APIVersion == mcmv1alpha1.SchemeGroupVersion.String()
}

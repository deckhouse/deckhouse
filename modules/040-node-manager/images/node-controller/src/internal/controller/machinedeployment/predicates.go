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

package machinedeployment

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// nodeGroupCloudInstancesChangedPredicate returns a predicate that filters NodeGroup
// events to only those where the cloudInstances or staticInstances spec has changed.
func nodeGroupCloudInstancesChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		DeleteFunc: func(_ event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNG, ok1 := e.ObjectOld.(*deckhousev1.NodeGroup)
			newNG, ok2 := e.ObjectNew.(*deckhousev1.NodeGroup)
			if !ok1 || !ok2 {
				return true
			}
			return !reflect.DeepEqual(oldNG.Spec.CloudInstances, newNG.Spec.CloudInstances) ||
				!reflect.DeepEqual(oldNG.Spec.StaticInstances, newNG.Spec.StaticInstances)
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

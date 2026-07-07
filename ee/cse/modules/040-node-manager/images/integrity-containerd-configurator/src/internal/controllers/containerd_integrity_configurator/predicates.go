/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package containerdintegrityconfigurator

//nolint:goimports,gci
import (
	"slices"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func policyPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPolicy, okOld := e.ObjectOld.(*deckhousev1alpha1.ContainerdIntegrityPolicy)
			newPolicy, okNew := e.ObjectNew.(*deckhousev1alpha1.ContainerdIntegrityPolicy)
			if !okOld || !okNew {
				return false
			}

			if oldPolicy.Spec.CA != newPolicy.Spec.CA {
				return true
			}

			return !slices.Equal(oldPolicy.Status.ProtectedNamespaces, newPolicy.Status.ProtectedNamespaces)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return true
		},
	}
}

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deleteChildIfOwned deletes obj only if it exists and is controller-owned by instance.
// Uses apiReader to bypass the label-filtered cache so it can discover orphaned children too.
func (r *WaypointController) deleteChildIfOwned(ctx context.Context, instance client.Object, obj client.Object) error {
	if err := r.apiReader.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return client.IgnoreNotFound(err)
	}

	if !metav1.IsControlledBy(obj, instance) {
		klog.V(4).InfoS("Skipping deletion: resource not owned by this instance",
			"name", obj.GetName(),
			"namespace", obj.GetNamespace(),
		)
		return nil
	}

	return client.IgnoreNotFound(
		r.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground)),
	)
}

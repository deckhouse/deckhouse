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

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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

func (r *InstanceReconciler) removeFinalizer(ctx context.Context, instance *deckhousev1alpha1.Instance) error {
	if !controllerutil.ContainsFinalizer(instance, finalizerName) {
		return nil
	}

	patch := client.MergeFrom(instance.DeepCopy())
	controllerutil.RemoveFinalizer(instance, finalizerName)

	if err := r.Client.Patch(ctx, instance, patch); err != nil {
		return fmt.Errorf("remove finalizer from instance %s: %w", instance.Name, err)
	}

	return nil
}

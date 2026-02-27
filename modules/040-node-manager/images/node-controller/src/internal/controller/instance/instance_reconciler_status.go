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

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *InstanceReconciler) reconcileBashibleStatus(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	desiredStatus := r.bashibleStatusFactory.FromConditions(instance.Status.Conditions)
	if instance.Status.BashibleStatus == desiredStatus {
		return nil
	}

	updated := instance.DeepCopy()
	updated.Status.BashibleStatus = desiredStatus
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("patch instance %q bashible status: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

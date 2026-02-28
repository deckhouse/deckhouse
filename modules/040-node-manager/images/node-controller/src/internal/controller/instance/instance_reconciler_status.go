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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const instanceBashibleStatusFieldOwner = "node-controller-instance-bashible-status"

func (r *InstanceReconciler) reconcileBashibleStatus(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	desiredStatus := bashibleStatusFromConditions(instance.Status.Conditions)
	desiredMessage := messageFromConditions(instance.Status.Conditions)
	if instance.Status.BashibleStatus == desiredStatus && instance.Status.Message == desiredMessage {
		return nil
	}

	applyObj := &deckhousev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha2.GroupVersion.String(),
			Kind:       "Instance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
		Status: deckhousev1alpha2.InstanceStatus{
			BashibleStatus: desiredStatus,
			Message:        desiredMessage,
		},
	}

	if err := r.Status().Patch(
		ctx,
		applyObj,
		client.Apply,
		client.FieldOwner(instanceBashibleStatusFieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("apply instance %q bashible status/message: %w", instance.Name, err)
	}

	instance.Status.BashibleStatus = desiredStatus
	instance.Status.Message = desiredMessage
	return nil
}

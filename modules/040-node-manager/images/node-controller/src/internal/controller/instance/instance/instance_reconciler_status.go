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

package instance

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

const instanceBashibleStatusFieldOwner = "node-controller-instance-bashible-status"

func (s *InstanceService) ReconcileBashibleStatus(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	desiredStatus := bashibleStatusFromConditions(instance.Status.Conditions)
	desiredMessage := messageFromConditions(instance.Status.Conditions)
	hasBashibleReady := hasBashibleReadyCondition(instance.Status.Conditions)
	clearBootstrapStatus := hasBashibleReady && instance.Status.BootstrapStatus != nil
	bashibleStatusChanged := instance.Status.BashibleStatus != desiredStatus || instance.Status.Message != desiredMessage
	if !bashibleStatusChanged && !clearBootstrapStatus {
		return nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.bashible_status.patch")

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

	if err := s.client.Status().Patch(
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
	if clearBootstrapStatus {
		if err := clearInstanceBootstrapStatus(ctx, s.client, instance.Name); err != nil {
			return err
		}
		instance.Status.BootstrapStatus = nil
	}
	return nil
}

func clearInstanceBootstrapStatus(ctx context.Context, c client.Client, instanceName string) error {
	obj := &deckhousev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha2.GroupVersion.String(),
			Kind:       "Instance",
		},
		ObjectMeta: metav1.ObjectMeta{Name: instanceName},
	}
	patch := client.RawPatch(types.MergePatchType, []byte(`{"status":{"bootstrapStatus":null}}`))
	if err := c.Status().Patch(ctx, obj, patch); err != nil {
		return fmt.Errorf("clear instance %q bootstrap status: %w", instanceName, err)
	}

	return nil
}

func hasBashibleReadyCondition(conditions []deckhousev1alpha2.InstanceCondition) bool {
	for i := range conditions {
		if conditions[i].Type == deckhousev1alpha2.InstanceConditionTypeBashibleReady {
			return true
		}
	}

	return false
}

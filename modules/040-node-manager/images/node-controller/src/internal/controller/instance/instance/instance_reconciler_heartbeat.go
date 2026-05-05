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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

const (
	instanceBashibleHeartbeatFieldOwner = "node-controller-instance-bashible-heartbeat"

	bashibleHeartbeatReason                   = "HeartBeat"
	bashibleHeartbeatWaitingApprovalReason    = deckhousev1alpha2.InstanceConditionTypeWaitingApproval
	bashibleHeartbeatWaitingDisruptionReason  = deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval
	bashibleHeartbeatMessage                  = "No Bashible reconciliation for 5m"
	bashibleHeartbeatWaitingApprovalMessage   = "No Bashible reconciliation for 20m: waiting for approval"
	bashibleHeartbeatWaitingDisruptionMessage = "No Bashible reconciliation for 20m: waiting for disruption approval"

	bashibleHeartbeatTimeout                  = 5 * time.Minute
	bashibleHeartbeatWaitingApprovalTimeout   = 20 * time.Minute
	bashibleHeartbeatWaitingDisruptionTimeout = 20 * time.Minute
)

func (s *InstanceService) ReconcileBashibleHeartbeat(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	desiredCondition, shouldPatch := desiredBashibleHeartbeatCondition(instance.Status.Conditions, time.Now())
	if !shouldPatch {
		return nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.bashible_heartbeat.patch")

	applyObj := &deckhousev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha2.GroupVersion.String(),
			Kind:       "Instance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
		Status: deckhousev1alpha2.InstanceStatus{
			Conditions: []deckhousev1alpha2.InstanceCondition{*desiredCondition},
		},
	}

	if err := s.client.Status().Patch(
		ctx,
		applyObj,
		client.Apply,
		client.FieldOwner(instanceBashibleHeartbeatFieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("apply heartbeat condition for instance %q: %w", instance.Name, err)
	}

	// Keep local conditions cache in sync to avoid an extra read after patch.
	upsertInstanceCondition(&instance.Status.Conditions, *desiredCondition)
	return nil
}

func desiredBashibleHeartbeatCondition(
	conditions []deckhousev1alpha2.InstanceCondition,
	now time.Time,
) (*deckhousev1alpha2.InstanceCondition, bool) {
	bashibleReady, hasBashibleReady := instancecommon.GetInstanceConditionByType(
		conditions,
		deckhousev1alpha2.InstanceConditionTypeBashibleReady,
	)
	if !hasBashibleReady {
		return nil, false
	}
	if bashibleReady.Status == metav1.ConditionFalse {
		// Skip heartbeat when bashible is already in error state to preserve the original failure reason
		return nil, false
	}
	probeTime := effectiveHeartbeatTime(bashibleReady)
	if probeTime == nil {
		return nil, false
	}

	waitingApproval := instancecommon.IsInstanceConditionTrue(conditions, deckhousev1alpha2.InstanceConditionTypeWaitingApproval)
	waitingDisruption := instancecommon.IsInstanceConditionTrue(conditions, deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval)

	elapsed := now.Sub(probeTime.Time)
	updated := bashibleReady
	switch {
	case waitingDisruption && elapsed >= bashibleHeartbeatWaitingDisruptionTimeout:
		updated.Status = metav1.ConditionUnknown
		updated.Reason = bashibleHeartbeatWaitingDisruptionReason
		updated.Message = bashibleHeartbeatWaitingDisruptionMessage
	case waitingApproval && elapsed >= bashibleHeartbeatWaitingApprovalTimeout:
		updated.Status = metav1.ConditionUnknown
		updated.Reason = bashibleHeartbeatWaitingApprovalReason
		updated.Message = bashibleHeartbeatWaitingApprovalMessage
	case !waitingApproval && !waitingDisruption && elapsed >= bashibleHeartbeatTimeout:
		updated.Status = metav1.ConditionUnknown
		updated.Reason = bashibleHeartbeatReason
		updated.Message = bashibleHeartbeatMessage
	default:
		return nil, false
	}

	if updated.Status == bashibleReady.Status &&
		updated.Reason == bashibleReady.Reason &&
		updated.Message == bashibleReady.Message {
		return nil, false
	}

	return &updated, true
}

func effectiveHeartbeatTime(condition deckhousev1alpha2.InstanceCondition) *metav1.Time {
	if condition.LastHeartbeatTime != nil && !condition.LastHeartbeatTime.IsZero() {
		return condition.LastHeartbeatTime
	}
	if condition.LastTransitionTime != nil && !condition.LastTransitionTime.IsZero() {
		return condition.LastTransitionTime
	}

	return nil
}

func upsertInstanceCondition(conditions *[]deckhousev1alpha2.InstanceCondition, desired deckhousev1alpha2.InstanceCondition) {
	for i := range *conditions {
		if (*conditions)[i].Type == desired.Type {
			(*conditions)[i] = desired
			return
		}
	}

	*conditions = append(*conditions, desired)
}

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
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

const (
	instanceNodeAgentFieldOwner = "node-controller-instance-node-agent"

	nodeAgentReportingReason  = "Reporting"
	nodeAgentReportingMessage = "the node agent reports"
	nodeAgentNoReportReason   = "NoReport"
)

// ReconcileNodeAgent sets NodeAgentReady on the Instance of a node that has a NodeConfig, on
// bashible nodes too: whether its node agent still reports to it. The periodic requeue of the
// controller flips the condition once the last report gets older than NodeAgentSilence.
func (s *InstanceService) ReconcileNodeAgent(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	nodeName := instance.Spec.NodeRef.Name
	if nodeName == "" {
		return nil
	}

	config, err := nodecommon.GetNodeConfig(ctx, s.client, nodeName)
	if err != nil || config == nil {
		return err
	}

	desiredCondition, shouldPatch := desiredNodeAgentCondition(instance.Status.Conditions, config, time.Now())
	if !shouldPatch {
		return nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.node_agent.patch")

	applyObj := instancecommon.InstanceApplyObject(instance.Name)
	applyObj.Status = deckhousev1alpha2.InstanceStatus{
		Conditions: []deckhousev1alpha2.InstanceCondition{*desiredCondition},
	}

	if err := s.client.Status().Patch(
		ctx,
		applyObj,
		client.Apply,
		client.FieldOwner(instanceNodeAgentFieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("apply node agent condition for instance %q: %w", instance.Name, err)
	}

	upsertInstanceCondition(&instance.Status.Conditions, *desiredCondition)
	return nil
}

// desiredNodeAgentCondition skips a node whose agent has not finished a pass yet: there is
// no report to age.
func desiredNodeAgentCondition(
	conditions []deckhousev1alpha2.InstanceCondition,
	config *internalv1alpha1.NodeConfig,
	now time.Time,
) (*deckhousev1alpha2.InstanceCondition, bool) {
	lastReport := config.Status.LastReconcileTime
	if lastReport.IsZero() {
		return nil, false
	}

	desired := deckhousev1alpha2.InstanceCondition{
		Type:    deckhousev1alpha2.InstanceConditionTypeNodeAgentReady,
		Status:  metav1.ConditionTrue,
		Reason:  nodeAgentReportingReason,
		Message: nodeAgentReportingMessage,
	}
	if now.Sub(lastReport.Time) > nodecommon.NodeAgentSilence {
		desired.Status = metav1.ConditionUnknown
		desired.Reason = nodeAgentNoReportReason
		desired.Message = "the node agent has not reported since " + lastReport.UTC().Format(time.RFC3339)
	}

	current, found := instancecommon.GetInstanceConditionByType(conditions, desired.Type)
	if found && sameConditionState(current, desired) {
		return nil, false
	}

	transitionTime := metav1.NewTime(now)
	desired.LastTransitionTime = &transitionTime
	statusKept := found && current.Status == desired.Status
	if statusKept && current.LastTransitionTime != nil {
		desired.LastTransitionTime = current.LastTransitionTime
	}

	return &desired, true
}

func sameConditionState(a, b deckhousev1alpha2.InstanceCondition) bool {
	if a.Status != b.Status || a.Reason != b.Reason {
		return false
	}
	return a.Message == b.Message
}

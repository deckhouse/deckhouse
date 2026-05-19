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

package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

type NodeType = v1.NodeType

const (
	NodeTypeCloudEphemeral = v1.NodeTypeCloudEphemeral
	NodeTypeCloudPermanent = v1.NodeTypeCloudPermanent
	NodeTypeCloudStatic    = v1.NodeTypeCloudStatic
	NodeTypeStatic         = v1.NodeTypeStatic
)

type ConditionStatus string

const (
	ConditionTrue  ConditionStatus = "True"
	ConditionFalse ConditionStatus = "False"
)

type NodeGroupConditionType string

const (
	NodeGroupConditionTypeReady                        NodeGroupConditionType = "Ready"
	NodeGroupConditionTypeUpdating                     NodeGroupConditionType = "Updating"
	NodeGroupConditionTypeWaitingForDisruptiveApproval NodeGroupConditionType = "WaitingForDisruptiveApproval"
	NodeGroupConditionTypeError                        NodeGroupConditionType = "Error"
	NodeGroupConditionTypeScaling                      NodeGroupConditionType = "Scaling"
	NodeGroupConditionTypeFrozen                       NodeGroupConditionType = "Frozen"
)

type NodeGroupCondition struct {
	Type               NodeGroupConditionType
	Status             ConditionStatus
	Message            string
	LastTransitionTime metav1.Time
}

func (in *NodeGroupCondition) DeepCopy() *NodeGroupCondition {
	if in == nil {
		return nil
	}
	out := new(NodeGroupCondition)
	*out = *in
	return out
}

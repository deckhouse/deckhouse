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
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// We consider that the node is ready in the first five minutes after bootstrap.
	nodeNotReadyGracePeriod = 5 * time.Minute
	machineGeneralError     = "Started Machine creation process"
)

type NodeGroup struct {
	Type      NodeType
	Instances int32
	Desired   int32

	HasFrozenMachineDeployment bool
}

type Node struct {
	Ready                     bool
	ShouldDeleted             bool
	Unschedulable             bool
	Updating                  bool
	CreationTimestamp         time.Time
	WaitingDisruptiveApproval bool
}

func NodeToConditionsNode(node *corev1.Node) *Node {
	res := &Node{}

	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			if c.Status == corev1.ConditionTrue {
				res.Ready = true
			}
			break
		}
	}

	res.CreationTimestamp = node.GetCreationTimestamp().Time

	for _, t := range node.Spec.Taints {
		if t.Key == "ToBeDeletedByClusterAutoscaler" {
			res.ShouldDeleted = true
			break
		}
	}

	_, disruptionRequired := node.Annotations["update.node.deckhouse.io/disruption-required"]
	_, disruptionApproved := node.Annotations["update.node.deckhouse.io/disruption-approved"]
	if disruptionRequired && !disruptionApproved {
		res.WaitingDisruptiveApproval = true
		res.Updating = true
	}

	if !res.Updating {
		for k := range node.Annotations {
			if strings.HasPrefix(k, "update.node.deckhouse.io") {
				res.Updating = true
				break
			}
		}
	}

	res.Unschedulable = node.Spec.Unschedulable

	return res
}

func boolToConditionStatus(b bool) ConditionStatus {
	if b {
		return ConditionTrue
	}

	return ConditionFalse
}

func conditionsEqual(curCondition *NodeGroupCondition, newCondition *NodeGroupCondition) bool {
	if curCondition == nil {
		return true
	}

	return curCondition.Status != newCondition.Status || curCondition.Message != newCondition.Message || curCondition.LastTransitionTime.IsZero()
}

func fillTransitionTime(currentConditions []NodeGroupCondition, newConditions []NodeGroupCondition, curTime time.Time) []NodeGroupCondition {
	cur := make(map[NodeGroupConditionType]*NodeGroupCondition)
	for _, c := range currentConditions {
		cc := c
		cur[c.Type] = &cc
	}

	res := make([]NodeGroupCondition, 0, len(newConditions))
	for i := 0; i < len(newConditions); i++ {
		curCondition := cur[newConditions[i].Type]
		different := conditionsEqual(curCondition, &newConditions[i])

		t := metav1.NewTime(curTime)
		if curCondition != nil && !different {
			t = curCondition.LastTransitionTime
		}

		res = append(res, NodeGroupCondition{
			Type:               newConditions[i].Type,
			Status:             newConditions[i].Status,
			Message:            newConditions[i].Message,
			LastTransitionTime: t,
		})
	}

	return res
}

func calcErrorCondition(ng *NodeGroup, currentConditions []NodeGroupCondition, errors []string) *NodeGroupCondition {
	var lastError *NodeGroupCondition
	for _, c := range currentConditions {
		if c.Type == NodeGroupConditionTypeError {
			lastError = c.DeepCopy()
			break
		}
	}

	errMsg := strings.TrimSpace(strings.Join(errors, "|"))
	isError := len(errors) > 0

	curError := &NodeGroupCondition{
		Type:    NodeGroupConditionTypeError,
		Status:  boolToConditionStatus(isError),
		Message: errMsg,
	}

	if lastError == nil {
		lastError = curError
	}

	// Keep previous error when deployment is frozen and failures list is temporarily empty.
	if len(errors) == 0 && ng.HasFrozenMachineDeployment {
		return lastError
	}

	if len(errors) == 1 && errors[0] == machineGeneralError {
		if lastError.Status == ConditionFalse || lastError.Message == "" {
			return curError
		}

		return lastError
	}

	return curError
}

func CalculateNodeGroupConditions(
	ng NodeGroup,
	nodes []*Node,
	currentConditions []NodeGroupCondition,
	errors []string,
	minPerAllZone int,
) []NodeGroupCondition {
	var inDownScale, isWaitingDisruptiveApproval, isUpdating bool

	readySchedulableNodes := 0

	curTime := time.Now()
	if timeStr, ok := os.LookupEnv("TEST_CONDITIONS_CALC_NOW_TIME"); ok {
		curTime, _ = time.Parse(time.RFC3339, timeStr)
	}

	for _, node := range nodes {
		if !node.Unschedulable {
			inGracePeriod := node.CreationTimestamp.Add(nodeNotReadyGracePeriod).After(curTime)
			if node.Ready || inGracePeriod {
				readySchedulableNodes++
			}
		}

		if node.Updating {
			isUpdating = true
		}

		if node.ShouldDeleted {
			inDownScale = true
		}

		if node.WaitingDisruptiveApproval {
			isWaitingDisruptiveApproval = true
		}
	}

	isReady := readySchedulableNodes >= minPerAllZone
	if ng.Type == NodeTypeStatic {
		if ng.Desired > 0 {
			isReady = readySchedulableNodes == int(ng.Desired)
		} else {
			isReady = readySchedulableNodes == len(nodes)
		}
	}

	errorCondition := calcErrorCondition(&ng, currentConditions, errors)

	newConditions := []NodeGroupCondition{
		{
			Type:   NodeGroupConditionTypeReady,
			Status: boolToConditionStatus(isReady),
		},
		{
			Type:   NodeGroupConditionTypeUpdating,
			Status: boolToConditionStatus(isUpdating),
		},
		{
			Type:   NodeGroupConditionTypeWaitingForDisruptiveApproval,
			Status: boolToConditionStatus(isWaitingDisruptiveApproval),
		},
		*errorCondition,
	}

	if ng.Type == NodeTypeCloudEphemeral {
		inUpScale := ng.Desired > int32(len(nodes))
		inDownScale = inDownScale || ng.Desired < ng.Instances

		isScaling := inDownScale || inUpScale

		newConditions = append(newConditions, NodeGroupCondition{
			Type:   NodeGroupConditionTypeScaling,
			Status: boolToConditionStatus(isScaling),
		})
	}

	return fillTransitionTime(currentConditions, newConditions, curTime)
}

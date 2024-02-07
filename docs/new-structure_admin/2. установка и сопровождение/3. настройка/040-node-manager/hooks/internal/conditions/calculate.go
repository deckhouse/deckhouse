/*
Copyright 2023 Flant JSC

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

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	// We consider that the node is ready in the first five minutes after the bootstrap
	nodeNotReadyGracePeriod = 5 * time.Minute
	machineGeneralError     = "Started Machine creation process"
)

type NodeGroup struct {
	Type      ngv1.NodeType
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

func boolToConditionStatus(b bool) ngv1.ConditionStatus {
	if b {
		return ngv1.ConditionTrue
	}

	return ngv1.ConditionFalse
}

func conditionsEqual(curCondition *ngv1.NodeGroupCondition, newCondition *ngv1.NodeGroupCondition) bool {
	if curCondition == nil {
		return true
	}

	return curCondition.Status != newCondition.Status || curCondition.Message != newCondition.Message || curCondition.LastTransitionTime.IsZero()
}

func fillTransitionTime(currentConditions []ngv1.NodeGroupCondition, newConditions []ngv1.NodeGroupCondition, curTime time.Time) []ngv1.NodeGroupCondition {
	cur := make(map[ngv1.NodeGroupConditionType]*ngv1.NodeGroupCondition)
	for _, c := range currentConditions {
		cc := c
		cur[c.Type] = &cc
	}

	res := make([]ngv1.NodeGroupCondition, 0, len(newConditions))
	for i := 0; i < len(newConditions); i++ {
		curCondition := cur[newConditions[i].Type]
		different := conditionsEqual(curCondition, &newConditions[i])

		t := metav1.NewTime(curTime)
		if curCondition != nil && !different {
			t = curCondition.LastTransitionTime
		}

		res = append(res, ngv1.NodeGroupCondition{
			Type:               newConditions[i].Type,
			Status:             newConditions[i].Status,
			Message:            newConditions[i].Message,
			LastTransitionTime: t,
		})
	}

	return res
}

func calcErrorCondition(ng *NodeGroup, currentConditions []ngv1.NodeGroupCondition, errors []string) *ngv1.NodeGroupCondition {
	var lastError *ngv1.NodeGroupCondition
	for _, c := range currentConditions {
		if c.Type == ngv1.NodeGroupConditionTypeError {
			lastError = c.DeepCopy()
			break
		}
	}

	errMsg := strings.TrimSpace(strings.Join(errors, "|"))
	isError := len(errors) > 0

	curError := &ngv1.NodeGroupCondition{
		Type:    ngv1.NodeGroupConditionTypeError,
		Status:  boolToConditionStatus(isError),
		Message: errMsg,
	}

	if lastError == nil {
		lastError = curError
	}

	// Machine deployment can be in 2 state
	// 1. Machine deployment has last operation with "Started Machine creation process"
	//    This message is ugly, and it doesn't make sense. We need to use previous message
	// 2. After some retries mcm freeze machine deployment for a short time and
	//    clear status machine deployment from failed machines. But ng is in incorrect state, and we need to continue
	//    set Error status on "True"

	if len(errors) == 0 && ng.HasFrozenMachineDeployment {
		return lastError
	}

	if len(errors) == 1 && errors[0] == machineGeneralError {
		if lastError.Status == ngv1.ConditionFalse || lastError.Message == "" {
			return curError
		}

		return lastError
	}

	return curError
}

func CalculateNodeGroupConditions(
	ng NodeGroup,
	nodes []*Node,
	currentConditions []ngv1.NodeGroupCondition,
	errors []string,
	minPerAllZone int,
) []ngv1.NodeGroupCondition {
	var inDownScale, isWaitingDisruptiveApproval, isUpdating bool

	schedulableNodes := 0
	readySchedulableNodes := 0

	curTime := time.Now()
	if timeStr, ok := os.LookupEnv("TEST_CONDITIONS_CALC_NOW_TIME"); ok {
		curTime, _ = time.Parse(time.RFC3339, timeStr)
	}

	for _, node := range nodes {
		if !node.Unschedulable {
			schedulableNodes++

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

	errorCondition := calcErrorCondition(&ng, currentConditions, errors)

	newConditions := []ngv1.NodeGroupCondition{
		{
			Type:   ngv1.NodeGroupConditionTypeReady,
			Status: boolToConditionStatus(isReady),
		},

		{
			Type:   ngv1.NodeGroupConditionTypeUpdating,
			Status: boolToConditionStatus(isUpdating),
		},

		{
			Type:   ngv1.NodeGroupConditionTypeWaitingForDisruptiveApproval,
			Status: boolToConditionStatus(isWaitingDisruptiveApproval),
		},

		*errorCondition,
	}

	if ng.Type == ngv1.NodeTypeCloudEphemeral {
		inUpScale := ng.Desired > int32(len(nodes))
		inDownScale = inDownScale || ng.Desired < ng.Instances

		isScaling := inDownScale || inUpScale

		newConditions = append(newConditions, ngv1.NodeGroupCondition{
			Type:   ngv1.NodeGroupConditionTypeScaling,
			Status: boolToConditionStatus(isScaling),
		})
	}

	return fillTransitionTime(currentConditions, newConditions, curTime)
}

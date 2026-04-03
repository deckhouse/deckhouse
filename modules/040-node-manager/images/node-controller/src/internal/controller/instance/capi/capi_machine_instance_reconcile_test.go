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

package capi

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

func TestGetConditionByType(t *testing.T) {
	t.Parallel()

	conditions := []deckhousev1alpha2.InstanceCondition{
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			Status: metav1.ConditionTrue,
			Reason: "StepsCompleted",
		},
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeMachineReady,
			Status: metav1.ConditionFalse,
			Reason: "OldReason",
		},
	}

	condition, ok := instancecommon.GetInstanceConditionByType(conditions, deckhousev1alpha2.InstanceConditionTypeMachineReady)
	if !ok {
		t.Fatalf("expected to find %q condition", deckhousev1alpha2.InstanceConditionTypeMachineReady)
	}
	if condition.Reason != "OldReason" {
		t.Fatalf("unexpected machine condition reason: %q", condition.Reason)
	}
}

func TestConditionEqualExceptLastTransitionTime(t *testing.T) {
	t.Parallel()

	left := deckhousev1alpha2.InstanceCondition{
		Type:               deckhousev1alpha2.InstanceConditionTypeMachineReady,
		Status:             metav1.ConditionFalse,
		Reason:             "WaitingForInfrastructure",
		Message:            "Waiting for infrastructure",
		Severity:           "Info",
		ObservedGeneration: 3,
	}
	leftTime := metav1.Now()
	left.LastTransitionTime = &leftTime
	right := left
	rightTime := metav1.Now()
	right.LastTransitionTime = &rightTime

	if !instancecommon.ConditionEqualExceptLastTransitionTime(left, right) {
		t.Fatal("expected conditions to be equal when only LastTransitionTime differs")
	}

	right.Reason = "InfrastructureReady"
	if instancecommon.ConditionEqualExceptLastTransitionTime(left, right) {
		t.Fatal("expected conditions to be different when non-time field differs")
	}
}

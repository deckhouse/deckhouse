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

package machine

import (
	"testing"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCalculateCAPIState_PrioritizesInfrastructureReadyWhenNotTrue(t *testing.T) {
	t.Parallel()

	infraMessage := "VM is not ready, state is Pending"
	readyMessage := "* InfrastructureReady: VM is not ready, state is Pending * NodeHealthy: Waiting for DeckhouseMachine to report spec.providerID"

	state := calculateCAPIState(
		[]metav1.Condition{
			{
				Type:    capi.InfrastructureReadyCondition,
				Status:  metav1.ConditionUnknown,
				Reason:  "VMNotReady",
				Message: infraMessage,
			},
			{
				Type:    capi.ReadyCondition,
				Status:  metav1.ConditionUnknown,
				Reason:  "ReadyUnknown",
				Message: readyMessage,
			},
		},
		capi.MachinePhasePending,
	)

	if state.reason != "VMNotReady" {
		t.Fatalf("expected reason from InfrastructureReady, got %q", state.reason)
	}
	if state.message != infraMessage {
		t.Fatalf("expected message from InfrastructureReady, got %q", state.message)
	}
	if state.sourceCondition == nil || state.sourceCondition.Type != capi.InfrastructureReadyCondition {
		t.Fatalf("expected source condition %q, got %#v", capi.InfrastructureReadyCondition, state.sourceCondition)
	}
}

func TestCalculateCAPIState_UsesReadyWhenInfrastructureIsTrue(t *testing.T) {
	t.Parallel()

	readyMessage := "Machine readiness is unknown"

	state := calculateCAPIState(
		[]metav1.Condition{
			{
				Type:   capi.InfrastructureReadyCondition,
				Status: metav1.ConditionTrue,
				Reason: "InfrastructureReady",
			},
			{
				Type:    capi.ReadyCondition,
				Status:  metav1.ConditionUnknown,
				Reason:  "ReadyUnknown",
				Message: readyMessage,
			},
		},
		capi.MachinePhasePending,
	)

	if state.reason != "ReadyUnknown" {
		t.Fatalf("expected reason from Ready condition, got %q", state.reason)
	}
	if state.message != readyMessage {
		t.Fatalf("expected message from Ready condition, got %q", state.message)
	}
	if state.sourceCondition == nil || state.sourceCondition.Type != capi.ReadyCondition {
		t.Fatalf("expected source condition %q, got %#v", capi.ReadyCondition, state.sourceCondition)
	}
}

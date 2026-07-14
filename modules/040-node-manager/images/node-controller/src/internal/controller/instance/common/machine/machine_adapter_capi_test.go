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
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func TestCAPIAccessors(t *testing.T) {
	t.Parallel()

	obj := &capi.Machine{}
	obj.Name = "capi-1"
	obj.Status.NodeRef = capi.MachineNodeReference{Name: "node-1"}
	m := &capiMachine{machine: obj}

	require.Equal(t, "capi-1", m.GetName())
	require.Equal(t, "node-1", m.GetNodeName())

	ref := m.GetMachineRef()
	require.NotNil(t, ref)
	require.Equal(t, "Machine", ref.Kind)
	require.Equal(t, capi.GroupVersion.String(), ref.APIVersion)
	require.Equal(t, "capi-1", ref.Name)
	require.Equal(t, MachineNamespace, ref.Namespace)
}

func TestCAPIGetStatusRunningIsReady(t *testing.T) {
	t.Parallel()

	obj := &capi.Machine{}
	obj.Name = "capi-running"
	obj.Status.Phase = string(capi.MachinePhaseRunning)
	m := &capiMachine{machine: obj}

	got := m.GetStatus()
	require.Equal(t, deckhousev1alpha2.InstancePhaseRunning, got.Phase)
	require.Equal(t, StatusReady, got.Status)
	require.NotNil(t, got.MachineReadyCondition)
	require.Equal(t, metav1.ConditionTrue, got.MachineReadyCondition.Status)
	require.Equal(t, reasonReady, got.MachineReadyCondition.Reason)
}

func TestCAPIGetStatusInfraNotReady(t *testing.T) {
	t.Parallel()

	obj := &capi.Machine{}
	obj.Name = "capi-pending"
	obj.Status.Phase = string(capi.MachinePhasePending)
	obj.Status.Conditions = []metav1.Condition{{
		Type:    capi.InfrastructureReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  reasonWaitingForInfra,
		Message: "Waiting   for\tinfrastructure",
	}}
	m := &capiMachine{machine: obj}

	got := m.GetStatus()
	require.Equal(t, StatusProgressing, got.Status)
	require.Equal(t, metav1.ConditionFalse, got.MachineReadyCondition.Status)
	// Source condition has no LastTransitionTime, so it stays the constructed "now".
	require.NotNil(t, got.MachineReadyCondition.LastTransitionTime)
}

func TestBuildMachineReadyConditionTransitionTimeSources(t *testing.T) {
	t.Parallel()

	t.Run("uses source condition transition time and observed generation", func(t *testing.T) {
		t.Parallel()

		transition := metav1.NewTime(metav1.Now().Add(-time.Hour))
		state := machineState{
			status:          StatusProgressing,
			conditionStatus: metav1.ConditionFalse,
			reason:          "Foo",
			sourceCondition: &metav1.Condition{
				LastTransitionTime: transition,
				ObservedGeneration: 7,
			},
		}

		cond := buildMachineReadyCondition(state)
		require.NotNil(t, cond.LastTransitionTime)
		require.True(t, cond.LastTransitionTime.Equal(&transition))
		require.Equal(t, int64(7), cond.ObservedGeneration)
	})

	t.Run("uses source transition time when no source condition", func(t *testing.T) {
		t.Parallel()

		transition := metav1.NewTime(metav1.Now().Add(-2 * time.Hour))
		state := machineState{
			status:               StatusReady,
			conditionStatus:      metav1.ConditionTrue,
			reason:               reasonReady,
			sourceTransitionTime: &transition,
		}

		cond := buildMachineReadyCondition(state)
		require.NotNil(t, cond.LastTransitionTime)
		require.True(t, cond.LastTransitionTime.Equal(&transition))
	})

	t.Run("falls back to now and normalizes message", func(t *testing.T) {
		t.Parallel()

		state := machineState{
			status:          StatusReady,
			conditionStatus: metav1.ConditionTrue,
			reason:          reasonReady,
			message:         "machine   is\n ready",
		}

		cond := buildMachineReadyCondition(state)
		require.NotNil(t, cond.LastTransitionTime)
		require.Equal(t, "machine is ready", cond.Message)
	})
}

func TestNormalizeMessage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", normalizeMessage(""))
	require.Equal(t, "a b c", normalizeMessage("  a   b\t\nc  "))
	require.Equal(t, "single", normalizeMessage("single"))
}

func TestConditionMessageOrReasonNil(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", conditionMessageOrReason(nil))
}

func TestStateFromInfraExpectedWaitDefaultsMessage(t *testing.T) {
	t.Parallel()

	// NotReady reason while provisioning is an expected wait: reason is kept,
	// message is replaced with the friendly waiting-for-infrastructure text.
	infra := &metav1.Condition{
		Type:    capi.InfrastructureReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  reasonNotReady,
		Message: "raw infra message",
	}
	state := stateFromInfra(capi.MachinePhaseProvisioning, infra)
	require.Equal(t, reasonNotReady, state.reason)
	require.Equal(t, waitingForInfrastructureMessage, state.message)
	require.Equal(t, StatusProgressing, state.status)
}

func TestStateFromInfraUnexpectedFailureKeepsMessage(t *testing.T) {
	t.Parallel()

	// Empty reason is not an expected wait, so the raw infra message is surfaced
	// as a warning instead of the friendly waiting text.
	infra := &metav1.Condition{
		Type:    capi.InfrastructureReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  "",
		Message: "vm boot failed",
	}
	state := stateFromInfra(capi.MachinePhaseRunning, infra)
	require.Equal(t, "vm boot failed", state.message)
	require.Equal(t, StatusProgressing, state.status)
	require.Equal(t, string(capi.ConditionSeverityWarning), state.severity)
}

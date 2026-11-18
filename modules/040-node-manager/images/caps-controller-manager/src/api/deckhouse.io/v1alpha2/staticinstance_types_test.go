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

package v1alpha2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// The connectivity conditions describe the host as seen by one StaticMachine, so they must not
// survive the instance going back to the pool: setStaticInstancePhaseToBootstrapping skips the
// TCP check outright on a CheckTcpConnection it finds already True, and the next machine would
// then go straight to ssh against a host that may be long gone.
//
// This cannot be guarded by ObservedGeneration instead. StaticInstance has a status
// subresource and its spec never changes - spec.address is rejected as immutable by the
// webhook - so metadata.generation stays put for the whole life of the object and every
// condition always looks current.
func TestToPendingDropsConnectivityConditions(t *testing.T) {
	instance := &StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "static-instance", Generation: 1},
		Status: StaticInstanceStatus{
			MachineRef: &corev1.ObjectReference{Name: "static-machine"},
			CurrentStatus: &StaticInstanceStatusCurrentStatus{
				Phase:          StaticInstanceStatusCurrentStatusPhaseBootstrapping,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour).UTC()),
			},
		},
	}

	for _, conditionType := range []string{
		StaticInstanceCheckTCPConnectionCondition,
		StaticInstanceCheckSSHConnectionCondition,
	} {
		conditions.Set(instance, metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             "CheckPassed",
			Message:            "check passed",
			LastTransitionTime: metav1.Now(),
		})
	}

	instance.ToPending()

	require.Nil(t, conditions.Get(instance, StaticInstanceCheckTCPConnectionCondition),
		"a stale CheckTcpConnection makes the next StaticMachine skip the TCP check")
	require.Nil(t, conditions.Get(instance, StaticInstanceCheckSSHConnectionCondition),
		"a stale CheckSshCondition makes the next StaticMachine skip the ssh check")
}

func TestSetPhaseKeepsLastUpdateTimeWithoutTransition(t *testing.T) {
	updatedAt := metav1.NewTime(time.Now().Add(-time.Hour).UTC())

	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			CurrentStatus: &StaticInstanceStatusCurrentStatus{
				Phase:          StaticInstanceStatusCurrentStatusPhaseBootstrapping,
				LastUpdateTime: updatedAt,
			},
		},
	}

	instance.SetPhase(StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	require.Equal(t, updatedAt, instance.Status.CurrentStatus.LastUpdateTime,
		"lastUpdateTime must not be rewritten when the phase does not change")
}

func TestSetPhaseUpdatesLastUpdateTimeOnTransition(t *testing.T) {
	updatedAt := metav1.NewTime(time.Now().Add(-time.Hour).UTC())

	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			CurrentStatus: &StaticInstanceStatusCurrentStatus{
				Phase:          StaticInstanceStatusCurrentStatusPhasePending,
				LastUpdateTime: updatedAt,
			},
		},
	}

	instance.SetPhase(StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	require.Equal(t, StaticInstanceStatusCurrentStatusPhaseBootstrapping, instance.GetPhase())
	require.True(t, instance.Status.CurrentStatus.LastUpdateTime.After(updatedAt.Time))
}

func TestToPendingIsIdempotent(t *testing.T) {
	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			MachineRef: &corev1.ObjectReference{Name: "static-machine"},
			NodeRef:    &corev1.ObjectReference{Name: "node"},
			CurrentStatus: &StaticInstanceStatusCurrentStatus{
				Phase:          StaticInstanceStatusCurrentStatusPhaseBootstrapping,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour).UTC()),
			},
		},
	}

	instance.ToPending()

	require.Nil(t, instance.Status.MachineRef)
	require.Nil(t, instance.Status.NodeRef)
	require.Equal(t, StaticInstanceStatusCurrentStatusPhasePending, instance.GetPhase())

	settled := instance.DeepCopy()

	instance.ToPending()

	require.Equal(t, settled, instance, "a repeated ToPending must not produce a diff to patch")
}

// A condition without a reason cannot pass the StaticInstance CRD. Conditions are an atomic
// list, so one such entry rejects every condition the controller writes in the same patch.
func TestSanitizeLegacyConditionsDropsEntriesWithoutReason(t *testing.T) {
	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, LastTransitionTime: metav1.Now()},
				{Type: "AddedToNodeGroup", Status: metav1.ConditionTrue, Reason: "AddedToNodeGroup", LastTransitionTime: metav1.Now()},
			},
		},
	}

	instance.SanitizeLegacyConditions()

	require.Len(t, instance.Status.Conditions, 1,
		"a legacy condition with an empty Reason must be dropped to let the next patch pass CRD validation")
	require.Equal(t, "AddedToNodeGroup", instance.Status.Conditions[0].Type,
		"only the condition carrying a valid Reason must survive")
}

// Before v1.76 the reason was not validated, and the controllers put arbitrary error strings
// there. The CRD pattern rejects spaces, dots and parentheses, so "non-empty" is not enough.
func TestSanitizeLegacyConditionsDropsFreeFormReasons(t *testing.T) {
	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			Conditions: []metav1.Condition{
				{Type: "CheckSshCondition", Status: metav1.ConditionFalse, Reason: "dial tcp 10.0.0.1:22: connect: connection refused", Message: "failed", LastTransitionTime: metav1.Now()},
				{Type: "AddedToNodeGroup", Status: metav1.ConditionTrue, Reason: "AddedToNodeGroup", Message: "added", LastTransitionTime: metav1.Now()},
			},
		},
	}

	instance.SanitizeLegacyConditions()

	require.Len(t, instance.Status.Conditions, 1)
	require.Equal(t, "AddedToNodeGroup", instance.Status.Conditions[0].Type)
}

func TestSanitizeLegacyConditionsKeepsReasonWithColonAndUnderscore(t *testing.T) {
	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			Conditions: []metav1.Condition{
				{Type: "CheckSshCondition", Status: metav1.ConditionFalse, Reason: "CheckFailed:ssh_timeout", Message: "failed", LastTransitionTime: metav1.Now()},
			},
		},
	}

	instance.SanitizeLegacyConditions()

	require.Len(t, instance.Status.Conditions, 1)
}

// The message was optional and omitted with omitempty, while the current CRD requires it.
// Dropping a condition that only lacks the message would lose valid state.
func TestSanitizeLegacyConditionsFillsEmptyMessageWithReason(t *testing.T) {
	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			Conditions: []metav1.Condition{
				{Type: "BootstrapSucceeded", Status: metav1.ConditionTrue, Reason: "InstanceReady", LastTransitionTime: metav1.Now()},
			},
		},
	}

	instance.SanitizeLegacyConditions()

	require.Len(t, instance.Status.Conditions, 1)
	require.Equal(t, "InstanceReady", instance.Status.Conditions[0].Message)
}

func TestSanitizeLegacyConditionsIsIdempotent(t *testing.T) {
	instance := &StaticInstance{
		Status: StaticInstanceStatus{
			Conditions: []metav1.Condition{
				{Type: "AddedToNodeGroup", Status: metav1.ConditionTrue, Reason: "AddedToNodeGroup", Message: "added", LastTransitionTime: metav1.Now()},
				{Type: "BootstrapSucceeded", Status: metav1.ConditionTrue, Reason: "InstanceReady", LastTransitionTime: metav1.Now()},
			},
		},
	}

	instance.SanitizeLegacyConditions()
	settled := instance.DeepCopy()

	instance.SanitizeLegacyConditions()

	require.Equal(t, settled, instance,
		"a repeated sanitize must not produce a diff, otherwise it would write to etcd on every reconcile")
}

func TestSanitizeLegacyConditionsHandlesEmptyConditions(t *testing.T) {
	instance := &StaticInstance{}

	require.NotPanics(t, func() { instance.SanitizeLegacyConditions() },
		"sanitize must be safe on freshly created objects that have no conditions yet")
	require.Empty(t, instance.Status.Conditions)
}

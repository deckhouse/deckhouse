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

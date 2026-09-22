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

package scope

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/conditions"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

func newInstanceScope(instance *deckhousev1.StaticInstance) *InstanceScope {
	return &InstanceScope{
		Scope:    &Scope{Logger: logr.Discard()},
		Instance: instance,
	}
}

func TestSetPhaseKeepsLastUpdateTimeWithoutTransition(t *testing.T) {
	updatedAt := metav1.NewTime(time.Now().Add(-time.Hour).UTC())

	instanceScope := newInstanceScope(&deckhousev1.StaticInstance{
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping,
				LastUpdateTime: updatedAt,
			},
		},
	})

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	require.Equal(t, updatedAt, instanceScope.Instance.Status.CurrentStatus.LastUpdateTime,
		"lastUpdateTime must not be rewritten when the phase does not change")
}

func TestSetPhaseUpdatesLastUpdateTimeOnTransition(t *testing.T) {
	updatedAt := metav1.NewTime(time.Now().Add(-time.Hour).UTC())

	instanceScope := newInstanceScope(&deckhousev1.StaticInstance{
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
				LastUpdateTime: updatedAt,
			},
		},
	})

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping)

	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, instanceScope.GetPhase())
	require.True(t, instanceScope.Instance.Status.CurrentStatus.LastUpdateTime.After(updatedAt.Time))
}

// The connectivity conditions describe the host as seen by one StaticMachine, so they must not
// survive the instance going back to the pool: setStaticInstancePhaseToBootstrapping skips the
// TCP check outright on a CheckTcpConnection it finds already True, and the next machine would
// then go straight to ssh against a host that may be long gone.
//
// This cannot be guarded by ObservedGeneration instead. StaticInstance has a status
// subresource and its spec never changes - spec.address is rejected as immutable by the
// webhook - so metadata.generation stays put for the whole life of the object and every
// condition always looks current.
func TestSetPendingDropsConnectivityConditions(t *testing.T) {
	instanceScope := newInstanceScope(&deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "static-instance", Generation: 1},
		Status: deckhousev1.StaticInstanceStatus{
			MachineRef: &corev1.ObjectReference{Name: "static-machine"},
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour).UTC()),
			},
		},
	})

	for _, conditionType := range []string{
		infrav1.StaticInstanceCheckTCPConnection,
		infrav1.StaticInstanceCheckSSHCondition,
	} {
		conditions.Set(instanceScope.Instance, metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.StaticInstanceCheckPassedReason,
			Message:            "check passed",
			LastTransitionTime: metav1.Now(),
		})
	}

	instanceScope.SetPending()

	require.Nil(t, conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckTCPConnection),
		"a stale CheckTcpConnection makes the next StaticMachine skip the TCP check")
	require.Nil(t, conditions.Get(instanceScope.Instance, infrav1.StaticInstanceCheckSSHCondition),
		"a stale CheckSshCondition makes the next StaticMachine skip the ssh check")
}

// A repeated SetPending must not produce anything for the patch helper to write: every
// write turns into a Pending watch event that re-enqueues the StaticMachines again.
func TestSetPendingIsIdempotent(t *testing.T) {
	instanceScope := newInstanceScope(&deckhousev1.StaticInstance{
		Status: deckhousev1.StaticInstanceStatus{
			MachineRef: &corev1.ObjectReference{Name: "static-machine"},
			NodeRef:    &corev1.ObjectReference{Name: "node"},
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour).UTC()),
			},
		},
	})

	instanceScope.SetPending()

	require.Nil(t, instanceScope.Instance.Status.MachineRef)
	require.Nil(t, instanceScope.Instance.Status.NodeRef)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, instanceScope.GetPhase())

	settled := instanceScope.Instance.DeepCopy()

	instanceScope.SetPending()

	require.Equal(t, settled, instanceScope.Instance, "a repeated SetPending must not produce a diff to patch")
}

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

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
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

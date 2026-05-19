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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func TestSyncInstanceStatusPreservesLastTransitionTimeWhenConditionSemanticsMatch(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	lastTransitionTime := metav1.NewTime(time.Unix(1700000000, 0).UTC())
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-preserve-transition-time"},
		Status: deckhousev1alpha2.InstanceStatus{
			Phase:         deckhousev1alpha2.InstancePhasePending,
			MachineStatus: string(machine.StatusProgressing),
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:               deckhousev1alpha2.InstanceConditionTypeMachineReady,
				Status:             metav1.ConditionFalse,
				Reason:             "WaitingForInfrastructure",
				Message:            "Waiting for infrastructure",
				LastTransitionTime: &lastTransitionTime,
			}},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(instance.DeepCopy()).
		Build()

	err := SyncInstanceStatus(context.Background(), c, instance, machine.MachineStatus{
		Phase:         deckhousev1alpha2.InstancePhaseRunning,
		Status:        machine.StatusReady,
		MachineReadyCondition: &deckhousev1alpha2.InstanceCondition{
			Type:               deckhousev1alpha2.InstanceConditionTypeMachineReady,
			Status:             metav1.ConditionFalse,
			Reason:             "WaitingForInfrastructure",
			Message:            "Waiting for infrastructure",
			LastTransitionTime: ptrToTime(metav1.NewTime(time.Unix(1800000000, 0).UTC())),
		},
	})
	require.NoError(t, err)

	persisted := &deckhousev1alpha2.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted)
	require.NoError(t, err)

	condition, ok := GetInstanceConditionByType(persisted.Status.Conditions, deckhousev1alpha2.InstanceConditionTypeMachineReady)
	require.True(t, ok)
	require.NotNil(t, condition.LastTransitionTime)
	require.True(t, condition.LastTransitionTime.Equal(&lastTransitionTime))
	require.Equal(t, deckhousev1alpha2.InstancePhaseRunning, persisted.Status.Phase)
	require.Equal(t, string(machine.StatusReady), persisted.Status.MachineStatus)
}

func TestSyncInstanceStatusUpdatesLastTransitionTimeWhenConditionStatusChanges(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	oldTransitionTime := metav1.NewTime(time.Unix(1700000000, 0).UTC())
	newTransitionTime := metav1.NewTime(time.Unix(1800000000, 0).UTC())
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-update-transition-time"},
		Status: deckhousev1alpha2.InstanceStatus{
			Phase:         deckhousev1alpha2.InstancePhasePending,
			MachineStatus: string(machine.StatusProgressing),
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:               deckhousev1alpha2.InstanceConditionTypeMachineReady,
				Status:             metav1.ConditionFalse,
				Reason:             "WaitingForInfrastructure",
				Message:            "Waiting for infrastructure",
				LastTransitionTime: &oldTransitionTime,
			}},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(instance.DeepCopy()).
		Build()

	err := SyncInstanceStatus(context.Background(), c, instance, machine.MachineStatus{
		Phase:         deckhousev1alpha2.InstancePhaseRunning,
		Status:        machine.StatusReady,
		MachineReadyCondition: &deckhousev1alpha2.InstanceCondition{
			Type:               deckhousev1alpha2.InstanceConditionTypeMachineReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Ready",
			Message:            "Machine is ready",
			LastTransitionTime: &newTransitionTime,
		},
	})
	require.NoError(t, err)

	persisted := &deckhousev1alpha2.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted)
	require.NoError(t, err)

	condition, ok := GetInstanceConditionByType(persisted.Status.Conditions, deckhousev1alpha2.InstanceConditionTypeMachineReady)
	require.True(t, ok)
	require.NotNil(t, condition.LastTransitionTime)
	require.True(t, condition.LastTransitionTime.Equal(&newTransitionTime))
	require.False(t, condition.LastTransitionTime.Equal(&oldTransitionTime))
}

func ptrToTime(t metav1.Time) *metav1.Time {
	return &t
}

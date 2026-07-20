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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func newStatusTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(objects...).
		Build()
}

func TestSyncInstanceStatusNilConditionFails(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: "nil-condition"}}
	c := newStatusTestClient(t, instance.DeepCopy())

	err := SyncInstanceStatus(context.Background(), c, instance, machine.MachineStatus{
		Phase:  deckhousev1alpha2.InstancePhaseRunning,
		Status: machine.StatusReady,
	})
	require.ErrorContains(t, err, `build desired MachineReady condition for instance "nil-condition": condition is nil`)
}

func TestSyncInstanceStatusNoChangeSkipsPatch(t *testing.T) {
	t.Parallel()

	condition := deckhousev1alpha2.InstanceCondition{
		Type:    deckhousev1alpha2.InstanceConditionTypeMachineReady,
		Status:  metav1.ConditionTrue,
		Reason:  "Ready",
		Message: "Machine is ready",
	}
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "no-change"},
		Status: deckhousev1alpha2.InstanceStatus{
			Phase:         deckhousev1alpha2.InstancePhaseRunning,
			MachineStatus: string(machine.StatusReady),
			Conditions:    []deckhousev1alpha2.InstanceCondition{*condition.DeepCopy()},
		},
	}

	patchCalled := false
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(instance.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
				patchCalled = true
				return nil
			},
		}).
		Build()

	desired := condition.DeepCopy()
	err := SyncInstanceStatus(context.Background(), c, instance, machine.MachineStatus{
		Phase:                 deckhousev1alpha2.InstancePhaseRunning,
		Status:                machine.StatusReady,
		MachineReadyCondition: desired,
	})
	require.NoError(t, err)
	require.False(t, patchCalled, "no patch expected when status is unchanged")
}

func TestSyncInstanceStatusAppendsConditionWhenAbsent(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "append-condition"},
		Status: deckhousev1alpha2.InstanceStatus{
			Phase:         deckhousev1alpha2.InstancePhasePending,
			MachineStatus: string(machine.StatusProgressing),
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:   "Unrelated",
				Status: metav1.ConditionTrue,
			}},
		},
	}
	c := newStatusTestClient(t, instance.DeepCopy())

	err := SyncInstanceStatus(context.Background(), c, instance, machine.MachineStatus{
		Phase:  deckhousev1alpha2.InstancePhaseRunning,
		Status: machine.StatusReady,
		MachineReadyCondition: &deckhousev1alpha2.InstanceCondition{
			Type:   deckhousev1alpha2.InstanceConditionTypeMachineReady,
			Status: metav1.ConditionTrue,
			Reason: "Ready",
		},
	})
	require.NoError(t, err)

	// Both the pre-existing unrelated condition and the new one are kept.
	_, hasUnrelated := GetInstanceConditionByType(instance.Status.Conditions, "Unrelated")
	require.True(t, hasUnrelated)
	_, hasMachineReady := GetInstanceConditionByType(instance.Status.Conditions, deckhousev1alpha2.InstanceConditionTypeMachineReady)
	require.True(t, hasMachineReady)
}

func TestApplyInstancePhase(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: "phase-ok"}}
	c := newStatusTestClient(t, instance.DeepCopy())

	require.NoError(t, ApplyInstancePhase(context.Background(), c, "phase-ok", deckhousev1alpha2.InstancePhaseRunning))

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "phase-ok"}, persisted))
	require.Equal(t, deckhousev1alpha2.InstancePhaseRunning, persisted.Status.Phase)
}

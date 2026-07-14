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

package instance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

func newStatusClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(objects...).
		Build()
}

func TestReconcileBashibleHeartbeatApplies(t *testing.T) {
	t.Parallel()

	stale := metav1.NewTime(time.Now().Add(-11 * time.Minute))
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "heartbeat-apply"},
		Status: deckhousev1alpha2.InstanceStatus{
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
				Status:            metav1.ConditionTrue,
				Reason:            "StepsCompleted",
				Message:           "ok",
				LastHeartbeatTime: &stale,
			}},
		},
	}
	c := newStatusClient(t, instance.DeepCopy())
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	require.NoError(t, svc.ReconcileBashibleHeartbeat(context.Background(), instance))

	condition, ok := findCondition(instance.Status.Conditions, func(cond deckhousev1alpha2.InstanceCondition) bool {
		return cond.Type == deckhousev1alpha2.InstanceConditionTypeBashibleReady
	})
	require.True(t, ok)
	require.Equal(t, metav1.ConditionUnknown, condition.Status)
	require.Equal(t, bashibleHeartbeatReason, condition.Reason)
}

func TestReconcileBashibleHeartbeatNoChange(t *testing.T) {
	t.Parallel()

	fresh := metav1.NewTime(time.Now())
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "heartbeat-fresh"},
		Status: deckhousev1alpha2.InstanceStatus{
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
				Status:            metav1.ConditionTrue,
				Reason:            "StepsCompleted",
				LastHeartbeatTime: &fresh,
			}},
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
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	require.NoError(t, svc.ReconcileBashibleHeartbeat(context.Background(), instance))
	require.False(t, patchCalled)
}

func TestDesiredBashibleHeartbeatDisruptionBranch(t *testing.T) {
	t.Parallel()

	now := time.Now()
	staleWaiting := metav1.NewTime(now.Add(-21 * time.Minute))

	conditions := []deckhousev1alpha2.InstanceCondition{
		{
			Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			Status:            metav1.ConditionTrue,
			Reason:            "StepsCompleted",
			LastHeartbeatTime: &staleWaiting,
		},
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval,
			Status: metav1.ConditionTrue,
		},
	}

	updated, shouldPatch := desiredBashibleHeartbeatCondition(conditions, now)
	require.True(t, shouldPatch)
	require.NotNil(t, updated)
	require.Equal(t, metav1.ConditionUnknown, updated.Status)
	require.Equal(t, bashibleHeartbeatWaitingDisruptionReason, updated.Reason)
	require.Equal(t, bashibleHeartbeatWaitingDisruptionMessage, updated.Message)
}

func TestDesiredBashibleHeartbeatNoBashibleCondition(t *testing.T) {
	t.Parallel()

	_, shouldPatch := desiredBashibleHeartbeatCondition(nil, time.Now())
	require.False(t, shouldPatch)
}

func TestDesiredBashibleHeartbeatNoProbeTime(t *testing.T) {
	t.Parallel()

	// BashibleReady true but with no heartbeat/transition time means we cannot
	// compute elapsed time, so no heartbeat is applied.
	conditions := []deckhousev1alpha2.InstanceCondition{{
		Type:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
		Status: metav1.ConditionTrue,
		Reason: "StepsCompleted",
	}}
	_, shouldPatch := desiredBashibleHeartbeatCondition(conditions, time.Now())
	require.False(t, shouldPatch)
}

func TestDesiredBashibleHeartbeatWaitingApprovalNotYetTimedOut(t *testing.T) {
	t.Parallel()

	now := time.Now()
	// Waiting for approval but elapsed below 20m and above 5m: the default branch
	// returns no patch because the waiting flag blocks the plain 5m timeout.
	recent := metav1.NewTime(now.Add(-10 * time.Minute))
	conditions := []deckhousev1alpha2.InstanceCondition{
		{
			Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			Status:            metav1.ConditionTrue,
			Reason:            "StepsCompleted",
			LastHeartbeatTime: &recent,
		},
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
			Status: metav1.ConditionTrue,
		},
	}
	_, shouldPatch := desiredBashibleHeartbeatCondition(conditions, now)
	require.False(t, shouldPatch)
}

func TestEffectiveHeartbeatTime(t *testing.T) {
	t.Parallel()

	heartbeat := metav1.NewTime(time.Unix(1700000000, 0).UTC())
	transition := metav1.NewTime(time.Unix(1800000000, 0).UTC())

	t.Run("prefers heartbeat time", func(t *testing.T) {
		t.Parallel()
		got := effectiveHeartbeatTime(deckhousev1alpha2.InstanceCondition{
			LastHeartbeatTime:  &heartbeat,
			LastTransitionTime: &transition,
		})
		require.NotNil(t, got)
		require.True(t, got.Equal(&heartbeat))
	})

	t.Run("falls back to transition time", func(t *testing.T) {
		t.Parallel()
		got := effectiveHeartbeatTime(deckhousev1alpha2.InstanceCondition{
			LastTransitionTime: &transition,
		})
		require.NotNil(t, got)
		require.True(t, got.Equal(&transition))
	})

	t.Run("nil when both are absent", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, effectiveHeartbeatTime(deckhousev1alpha2.InstanceCondition{}))
	})

	t.Run("nil when both are zero", func(t *testing.T) {
		t.Parallel()
		zero := metav1.Time{}
		require.Nil(t, effectiveHeartbeatTime(deckhousev1alpha2.InstanceCondition{
			LastHeartbeatTime:  &zero,
			LastTransitionTime: &zero,
		}))
	})
}

func TestUpsertInstanceConditionPointer(t *testing.T) {
	t.Parallel()

	t.Run("replaces existing", func(t *testing.T) {
		t.Parallel()
		conditions := []deckhousev1alpha2.InstanceCondition{
			{Type: "A", Reason: "old"},
			{Type: "B"},
		}
		upsertInstanceCondition(&conditions, deckhousev1alpha2.InstanceCondition{Type: "A", Reason: "new"})
		require.Len(t, conditions, 2)
		require.Equal(t, "new", conditions[0].Reason)
	})

	t.Run("appends when missing", func(t *testing.T) {
		t.Parallel()
		conditions := []deckhousev1alpha2.InstanceCondition{{Type: "A"}}
		upsertInstanceCondition(&conditions, deckhousev1alpha2.InstanceCondition{Type: "C"})
		require.Len(t, conditions, 2)
		require.Equal(t, "C", conditions[1].Type)
	})
}

func TestReconcileBashibleHeartbeatSyncsLocalCache(t *testing.T) {
	t.Parallel()

	stale := metav1.NewTime(time.Now().Add(-11 * time.Minute))
	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "heartbeat-cache"},
		Status: deckhousev1alpha2.InstanceStatus{
			Conditions: []deckhousev1alpha2.InstanceCondition{{
				Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
				Status:            metav1.ConditionTrue,
				Reason:            "StepsCompleted",
				LastHeartbeatTime: &stale,
			}},
		},
	}
	c := newStatusClient(t, instance.DeepCopy())
	svc := &InstanceService{client: c, machineFactory: machine.NewMachineFactory()}

	require.NoError(t, svc.ReconcileBashibleHeartbeat(context.Background(), instance))

	persisted := &deckhousev1alpha2.Instance{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted))
	condition, ok := findCondition(persisted.Status.Conditions, func(cond deckhousev1alpha2.InstanceCondition) bool {
		return cond.Type == deckhousev1alpha2.InstanceConditionTypeBashibleReady
	})
	require.True(t, ok)
	require.Equal(t, metav1.ConditionUnknown, condition.Status)
}

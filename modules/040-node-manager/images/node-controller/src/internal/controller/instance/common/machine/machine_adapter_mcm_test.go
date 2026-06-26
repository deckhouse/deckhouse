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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

func TestMCMAccessors(t *testing.T) {
	t.Parallel()

	obj := &mcmv1alpha1.Machine{}
	obj.Name = "mcm-1"
	obj.Status.Node = "node-1"
	obj.Spec.NodeTemplateSpec.Labels = map[string]string{nodecommon.NodeGroupLabel: "worker"}
	m := &mcmMachine{machine: obj}

	require.Equal(t, "mcm-1", m.GetName())
	require.Equal(t, "node-1", m.GetNodeName())
	require.Equal(t, "worker", m.GetNodeGroup())

	ref := m.GetMachineRef()
	require.NotNil(t, ref)
	require.Equal(t, "Machine", ref.Kind)
	require.Equal(t, mcmv1alpha1.SchemeGroupVersion.String(), ref.APIVersion)
	require.Equal(t, "mcm-1", ref.Name)
	require.Equal(t, MachineNamespace, ref.Namespace)
}

func TestMCMGetNodeGroupEmpty(t *testing.T) {
	t.Parallel()

	m := &mcmMachine{machine: &mcmv1alpha1.Machine{}}
	require.Empty(t, m.GetNodeGroup())
}

func TestMCMCalculatePhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		phase mcmv1alpha1.MachinePhase
		want  deckhousev1alpha2.InstancePhase
	}{
		{"pending", mcmv1alpha1.MachinePending, deckhousev1alpha2.InstancePhasePending},
		{"creating", mcmv1alpha1.MachineCreating, deckhousev1alpha2.InstancePhasePending},
		{"available", mcmv1alpha1.MachineAvailable, deckhousev1alpha2.InstancePhasePending},
		{"running", mcmv1alpha1.MachineRunning, deckhousev1alpha2.InstancePhaseRunning},
		{"terminating", mcmv1alpha1.MachineTerminating, deckhousev1alpha2.InstancePhaseTerminating},
		{"unknown", mcmv1alpha1.MachineUnknown, deckhousev1alpha2.InstancePhaseUnknown},
		{"failed", mcmv1alpha1.MachineFailed, deckhousev1alpha2.InstancePhaseUnknown},
		{"crashloop", mcmv1alpha1.MachineCrashLoopBackOff, deckhousev1alpha2.InstancePhaseUnknown},
		{"empty defaults to unknown", mcmv1alpha1.MachinePhase(""), deckhousev1alpha2.InstancePhaseUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obj := &mcmv1alpha1.Machine{}
			obj.Status.CurrentStatus.Phase = tt.phase
			m := &mcmMachine{machine: obj}
			require.Equal(t, tt.want, m.calculatePhase())
		})
	}
}

func TestMCMGetStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		phase       mcmv1alpha1.MachinePhase
		lastOp      mcmv1alpha1.LastOperation
		wantStatus  Status
		wantPhase   deckhousev1alpha2.InstancePhase
		wantCond    metav1.ConditionStatus
		wantReason  string
		wantMessage string
	}{
		{
			name:        "running successful is ready",
			phase:       mcmv1alpha1.MachineRunning,
			lastOp:      mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateSuccessful, Description: "all good"},
			wantStatus:  StatusReady,
			wantPhase:   deckhousev1alpha2.InstancePhaseRunning,
			wantCond:    metav1.ConditionTrue,
			wantReason:  mcmReasonReady,
			wantMessage: "all good",
		},
		{
			name:       "pending successful is progressing",
			phase:      mcmv1alpha1.MachinePending,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateSuccessful, Description: "creating"},
			wantStatus: StatusProgressing,
			wantPhase:  deckhousev1alpha2.InstancePhasePending,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonNotReady,
		},
		{
			name:       "failed not draining is error",
			phase:      mcmv1alpha1.MachineFailed,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateFailed, Description: "provisioning failed"},
			wantStatus: StatusError,
			wantPhase:  deckhousev1alpha2.InstancePhaseUnknown,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonNotReady,
		},
		{
			name:       "failed with drain message while not terminating is blocked",
			phase:      mcmv1alpha1.MachineFailed,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateFailed, Description: "drain failed: cannot evict pod"},
			wantStatus: StatusBlocked,
			wantPhase:  deckhousev1alpha2.InstancePhaseUnknown,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonNotReady,
		},
		{
			name:       "processing is progressing",
			phase:      mcmv1alpha1.MachinePending,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateProcessing, Description: "working"},
			wantStatus: StatusProgressing,
			wantPhase:  deckhousev1alpha2.InstancePhasePending,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonNotReady,
		},
		{
			name:       "running with empty op state is ready without message",
			phase:      mcmv1alpha1.MachineRunning,
			lastOp:     mcmv1alpha1.LastOperation{},
			wantStatus: StatusReady,
			wantPhase:  deckhousev1alpha2.InstancePhaseRunning,
			wantCond:   metav1.ConditionTrue,
			wantReason: mcmReasonReady,
		},
		{
			name:       "pending with empty op state is unknown progressing",
			phase:      mcmv1alpha1.MachinePending,
			lastOp:     mcmv1alpha1.LastOperation{},
			wantStatus: StatusProgressing,
			wantPhase:  deckhousev1alpha2.InstancePhasePending,
			wantCond:   metav1.ConditionUnknown,
			wantReason: mcmReasonUnknown,
		},
		{
			name:       "terminating successful is progressing deleting",
			phase:      mcmv1alpha1.MachineTerminating,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateProcessing, Description: "deleting vm"},
			wantStatus: StatusProgressing,
			wantPhase:  deckhousev1alpha2.InstancePhaseTerminating,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonDeleting,
		},
		{
			name:       "terminating failed is error delete failed",
			phase:      mcmv1alpha1.MachineTerminating,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateFailed, Description: "delete failed"},
			wantStatus: StatusError,
			wantPhase:  deckhousev1alpha2.InstancePhaseTerminating,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonDeleteFailed,
		},
		{
			name:       "terminating drain blocked is blocked deleting",
			phase:      mcmv1alpha1.MachineTerminating,
			lastOp:     mcmv1alpha1.LastOperation{State: mcmv1alpha1.MachineStateProcessing, Description: "Cannot evict pod as it would violate the pod's disruption budget"},
			wantStatus: StatusBlocked,
			wantPhase:  deckhousev1alpha2.InstancePhaseTerminating,
			wantCond:   metav1.ConditionFalse,
			wantReason: mcmReasonDeleting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obj := &mcmv1alpha1.Machine{}
			obj.Status.CurrentStatus.Phase = tt.phase
			obj.Status.LastOperation = tt.lastOp
			m := &mcmMachine{machine: obj}

			got := m.GetStatus()
			require.Equal(t, tt.wantPhase, got.Phase)
			require.Equal(t, tt.wantStatus, got.Status)
			require.NotNil(t, got.MachineReadyCondition)
			require.Equal(t, tt.wantCond, got.MachineReadyCondition.Status)
			require.Equal(t, tt.wantReason, got.MachineReadyCondition.Reason)
			if tt.wantMessage != "" {
				require.Equal(t, tt.wantMessage, got.MachineReadyCondition.Message)
			}
		})
	}
}

func TestMCMGetStatusUsesSourceTransitionTime(t *testing.T) {
	t.Parallel()

	lastOpTime := metav1.NewTime(time.Unix(1700000000, 0).UTC())
	obj := &mcmv1alpha1.Machine{}
	obj.Status.CurrentStatus.Phase = mcmv1alpha1.MachineRunning
	obj.Status.LastOperation = mcmv1alpha1.LastOperation{
		State:          mcmv1alpha1.MachineStateSuccessful,
		LastUpdateTime: lastOpTime,
		Description:    "ok",
	}
	m := &mcmMachine{machine: obj}

	got := m.GetStatus()
	require.NotNil(t, got.MachineReadyCondition.LastTransitionTime)
	require.True(t, got.MachineReadyCondition.LastTransitionTime.Equal(&lastOpTime))
}

func TestMCMReasonFromLastOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state mcmv1alpha1.MachineState
		phase deckhousev1alpha2.InstancePhase
		want  string
	}{
		{"terminating failed", mcmv1alpha1.MachineStateFailed, deckhousev1alpha2.InstancePhaseTerminating, mcmReasonDeleteFailed},
		{"terminating other", mcmv1alpha1.MachineStateProcessing, deckhousev1alpha2.InstancePhaseTerminating, mcmReasonDeleting},
		{"running successful", mcmv1alpha1.MachineStateSuccessful, deckhousev1alpha2.InstancePhaseRunning, mcmReasonReady},
		{"successful not running", mcmv1alpha1.MachineStateSuccessful, deckhousev1alpha2.InstancePhasePending, mcmReasonNotReady},
		{"processing", mcmv1alpha1.MachineStateProcessing, deckhousev1alpha2.InstancePhasePending, mcmReasonNotReady},
		{"failed not terminating", mcmv1alpha1.MachineStateFailed, deckhousev1alpha2.InstancePhasePending, mcmReasonNotReady},
		{"empty state", mcmv1alpha1.MachineState(""), deckhousev1alpha2.InstancePhasePending, mcmReasonUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mcmReasonFromLastOperation(mcmv1alpha1.LastOperation{State: tt.state}, tt.phase)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildMCMSourceCondition(t *testing.T) {
	t.Parallel()

	lastOp := metav1.NewTime(time.Unix(1700000000, 0).UTC())
	currentStatus := metav1.NewTime(time.Unix(1800000000, 0).UTC())

	t.Run("prefers last operation time", func(t *testing.T) {
		t.Parallel()
		got := buildMCMSourceCondition(lastOp, currentStatus)
		require.NotNil(t, got)
		require.True(t, got.Equal(&lastOp))
	})

	t.Run("falls back to current status time", func(t *testing.T) {
		t.Parallel()
		got := buildMCMSourceCondition(metav1.Time{}, currentStatus)
		require.NotNil(t, got)
		require.True(t, got.Equal(&currentStatus))
	})

	t.Run("returns nil when both are zero", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, buildMCMSourceCondition(metav1.Time{}, metav1.Time{}))
	})
}

func TestIsMCMDrainBlocked(t *testing.T) {
	t.Parallel()

	tests := []struct {
		message string
		want    bool
	}{
		{"", false},
		{"all good", false},
		{"Drain failed for node", true},
		{"cannot evict pod foo", true},
		{"violates pod disruption budget", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.message, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, isMCMDrainBlocked(tt.message))
		})
	}
}

func TestMCMEnsureDeleted(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, mcmv1alpha1.AddToScheme(scheme))

	t.Run("already deleting -> not gone", func(t *testing.T) {
		t.Parallel()
		now := metav1.Now()
		obj := &mcmv1alpha1.Machine{}
		obj.Name = "m1"
		obj.Namespace = MachineNamespace
		obj.DeletionTimestamp = &now
		obj.Finalizers = []string{"keep"}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
		m := &mcmMachine{machine: obj}

		res, err := m.EnsureDeleted(context.Background(), c)
		require.NoError(t, err)
		require.False(t, res.Gone)
	})

	t.Run("missing machine -> gone", func(t *testing.T) {
		t.Parallel()
		obj := &mcmv1alpha1.Machine{}
		obj.Name = "absent"
		obj.Namespace = MachineNamespace
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		m := &mcmMachine{machine: obj}

		res, err := m.EnsureDeleted(context.Background(), c)
		require.NoError(t, err)
		require.True(t, res.Gone)
	})

	t.Run("present machine -> delete issued, not gone", func(t *testing.T) {
		t.Parallel()
		obj := &mcmv1alpha1.Machine{}
		obj.Name = "m2"
		obj.Namespace = MachineNamespace
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
		m := &mcmMachine{machine: obj}

		res, err := m.EnsureDeleted(context.Background(), c)
		require.NoError(t, err)
		require.False(t, res.Gone)
	})
}

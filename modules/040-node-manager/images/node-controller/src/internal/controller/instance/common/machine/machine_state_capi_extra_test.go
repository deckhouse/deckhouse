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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func TestCAPICalculatePhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		phase     capi.MachinePhase
		deleting  bool
		wantPhase deckhousev1alpha2.InstancePhase
	}{
		{"pending", capi.MachinePhasePending, false, deckhousev1alpha2.InstancePhasePending},
		{"provisioning", capi.MachinePhaseProvisioning, false, deckhousev1alpha2.InstancePhaseProvisioning},
		{"provisioned", capi.MachinePhaseProvisioned, false, deckhousev1alpha2.InstancePhaseProvisioned},
		{"running", capi.MachinePhaseRunning, false, deckhousev1alpha2.InstancePhaseRunning},
		{"deleting", capi.MachinePhaseDeleting, false, deckhousev1alpha2.InstancePhaseTerminating},
		{"deleted", capi.MachinePhaseDeleted, false, deckhousev1alpha2.InstancePhaseTerminating},
		{"unknown phase", capi.MachinePhase("Bogus"), false, deckhousev1alpha2.InstancePhaseUnknown},
		{"deletion timestamp overrides phase", capi.MachinePhaseRunning, true, deckhousev1alpha2.InstancePhaseTerminating},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obj := &capi.Machine{Status: capi.MachineStatus{Phase: string(tt.phase)}}
			if tt.deleting {
				now := metav1.Now()
				obj.DeletionTimestamp = &now
				obj.Finalizers = []string{"keep"}
			}
			m := &capiMachine{machine: obj}

			if got := m.calculatePhase(); got != tt.wantPhase {
				t.Fatalf("phase: got %q want %q", got, tt.wantPhase)
			}
		})
	}
}

func TestCAPIGetNodeGroup(t *testing.T) {
	t.Parallel()

	withLabel := &capiMachine{machine: &capi.Machine{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{"node-group": "worker"},
	}}}
	if got := withLabel.GetNodeGroup(); got != "worker" {
		t.Fatalf("node group: got %q want %q", got, "worker")
	}

	noLabels := &capiMachine{machine: &capi.Machine{}}
	if got := noLabels.GetNodeGroup(); got != "" {
		t.Fatalf("node group: got %q want empty", got)
	}
}

func TestCAPIEnsureDeleted(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := capi.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}

	t.Run("already deleting -> not gone", func(t *testing.T) {
		t.Parallel()
		now := metav1.Now()
		obj := &capi.Machine{ObjectMeta: metav1.ObjectMeta{
			Name: "c1", Namespace: MachineNamespace, DeletionTimestamp: &now, Finalizers: []string{"keep"},
		}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
		m := &capiMachine{machine: obj}

		res, err := m.EnsureDeleted(context.Background(), c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Gone {
			t.Fatalf("expected Gone=false for an already-deleting machine")
		}
	})

	t.Run("missing machine -> gone", func(t *testing.T) {
		t.Parallel()
		obj := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "absent", Namespace: MachineNamespace}}
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		m := &capiMachine{machine: obj}

		res, err := m.EnsureDeleted(context.Background(), c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Gone {
			t.Fatalf("expected Gone=true when the machine is already absent")
		}
	})

	t.Run("present machine -> delete issued, not gone", func(t *testing.T) {
		t.Parallel()
		obj := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "c2", Namespace: MachineNamespace}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
		m := &capiMachine{machine: obj}

		res, err := m.EnsureDeleted(context.Background(), c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Gone {
			t.Fatalf("expected Gone=false right after issuing the delete")
		}
	})
}

func TestCalculateCAPIStateBranches(t *testing.T) {
	t.Parallel()

	infraTrue := metav1.Condition{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionTrue, Reason: "InfrastructureReady"}

	tests := []struct {
		name       string
		conditions []metav1.Condition
		phase      capi.MachinePhase
		wantStatus Status
		wantCond   metav1.ConditionStatus
		wantReason string
	}{
		{
			name:       "running with no problem conditions is ready",
			conditions: nil,
			phase:      capi.MachinePhaseRunning,
			wantStatus: StatusReady,
			wantCond:   metav1.ConditionTrue,
			wantReason: reasonReady,
		},
		{
			name:       "no conditions and not running falls back to waiting for infrastructure",
			conditions: nil,
			phase:      capi.MachinePhasePending,
			wantStatus: StatusProgressing,
			wantCond:   metav1.ConditionUnknown,
			wantReason: reasonWaitingForInfra,
		},
		{
			name:       "infra ready and ready condition true is ready",
			conditions: []metav1.Condition{infraTrue, {Type: capi.ReadyCondition, Status: metav1.ConditionTrue, Reason: "Ready"}},
			phase:      capi.MachinePhaseProvisioned,
			wantStatus: StatusReady,
			wantCond:   metav1.ConditionTrue,
			wantReason: reasonReady,
		},
		{
			name:       "infra ready and ready condition false is progressing",
			conditions: []metav1.Condition{infraTrue, {Type: capi.ReadyCondition, Status: metav1.ConditionFalse, Reason: "NodeStartupTimeout", Message: "node did not start"}},
			phase:      capi.MachinePhaseProvisioned,
			wantStatus: StatusProgressing,
			wantCond:   metav1.ConditionFalse,
			wantReason: "NodeStartupTimeout",
		},
		{
			name:       "infra not ready with WaitingForInfrastructure reason is an expected wait",
			conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionFalse, Reason: reasonWaitingForInfra}},
			phase:      capi.MachinePhasePending,
			wantStatus: StatusProgressing,
			wantCond:   metav1.ConditionFalse,
			wantReason: reasonWaitingForInfra,
		},
		{
			name:       "infra not ready with NotReady reason while provisioning is an expected wait",
			conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionFalse, Reason: reasonNotReady}},
			phase:      capi.MachinePhaseProvisioning,
			wantStatus: StatusProgressing,
			wantCond:   metav1.ConditionFalse,
			wantReason: reasonNotReady,
		},
		{
			name:       "infra not ready with NotReady reason while running is an unexpected problem",
			conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionFalse, Reason: reasonNotReady}},
			phase:      capi.MachinePhaseRunning,
			wantStatus: StatusProgressing,
			wantCond:   metav1.ConditionFalse,
			wantReason: reasonNotReady,
		},
		{
			name:       "deleting without drain block is progressing",
			conditions: []metav1.Condition{{Type: capi.DeletingCondition, Status: metav1.ConditionTrue, Reason: "WaitingForPreDrainHook", Message: "waiting"}},
			phase:      capi.MachinePhaseDeleting,
			wantStatus: StatusProgressing,
			wantCond:   metav1.ConditionFalse,
			wantReason: "WaitingForPreDrainHook",
		},
		{
			name:       "deleting with drain block is blocked",
			conditions: []metav1.Condition{{Type: capi.DeletingCondition, Status: metav1.ConditionTrue, Reason: capi.MachineDeletingDrainingNodeReason, Message: "cannot evict pod"}},
			phase:      capi.MachinePhaseDeleting,
			wantStatus: StatusBlocked,
			wantCond:   metav1.ConditionFalse,
			wantReason: capi.MachineDeletingDrainingNodeReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := calculateCAPIState(tt.conditions, tt.phase)
			if state.status != tt.wantStatus {
				t.Fatalf("status: got %q want %q", state.status, tt.wantStatus)
			}
			if state.conditionStatus != tt.wantCond {
				t.Fatalf("condition status: got %q want %q", state.conditionStatus, tt.wantCond)
			}
			if state.reason != tt.wantReason {
				t.Fatalf("reason: got %q want %q", state.reason, tt.wantReason)
			}
		})
	}
}

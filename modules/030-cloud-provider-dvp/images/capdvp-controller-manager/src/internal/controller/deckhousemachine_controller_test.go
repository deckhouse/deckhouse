/*
Copyright 2025 Flant JSC

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

package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	"github.com/deckhouse/virtualization/api/core/v1alpha2/vmcondition"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1b2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrastructurev1alpha1 "cluster-api-provider-dvp/api/v1alpha1"
)

var _ = Describe("DeckhouseMachine Controller", func() {
	Context("When reconciling a resource that does not exist", func() {
		It("should return no error (idempotent not-found)", func() {
			controllerReconciler := &DeckhouseMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "nonexistent-machine",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})

	Context("When reconciling a resource that has no owner Machine", func() {
		const resourceName = "test-resource"

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		deckhousemachine := &infrastructurev1alpha1.DeckhouseMachine{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind DeckhouseMachine")
			err := k8sClient.Get(ctx, typeNamespacedName, deckhousemachine)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1alpha1.DeckhouseMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &infrastructurev1alpha1.DeckhouseMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DeckhouseMachine")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should return no error and not add a finalizer when owner Machine is absent", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DeckhouseMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// No owner Machine → controller must not add the MachineFinalizer.
			fetched := &infrastructurev1alpha1.DeckhouseMachine{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, fetched)).To(Succeed())
			Expect(fetched.Finalizers).NotTo(ContainElement(infrastructurev1alpha1.MachineFinalizer))
		})
	})
})

func TestEvaluateDiskStorageClassMigration(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	started := now.Add(-10 * time.Minute)
	timeout := 2 * time.Hour

	tests := []struct {
		name       string
		in         diskSCMigrationEvalInput
		wantStep   diskSCMigrationStep
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "returns wait when disk is provisioning",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskProvisioning,
				DesiredSC: "fast",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep: diskSCStepWait,
		},
		{
			name: "returns error when disk failed",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskFailed,
				DesiredSC: "fast",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep:   diskSCStepComplete,
			wantErr:    true,
			wantErrSub: "Failed",
		},
		{
			name: "returns complete when spec and status match desired",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskReady,
				SpecSC:    "fast",
				StatusSC:  "fast",
				DesiredSC: "fast",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep: diskSCStepComplete,
		},
		{
			name: "returns wait when spec patched and status lags",
			in: diskSCMigrationEvalInput{
				Phase:               v1alpha2.DiskReady,
				SpecSC:              "fast",
				StatusSC:            "slow",
				DesiredSC:           "fast",
				HasMigrationStarted: true,
				MigrationStartedAt:  started,
				Now:                 now,
				Timeout:             timeout,
			},
			wantStep: diskSCStepWait,
		},
		{
			name: "returns error when migration times out",
			in: diskSCMigrationEvalInput{
				Phase:               v1alpha2.DiskReady,
				SpecSC:              "fast",
				StatusSC:            "slow",
				DesiredSC:           "fast",
				HasMigrationStarted: true,
				MigrationStartedAt:  now.Add(-3 * time.Hour),
				Now:                 now,
				Timeout:             timeout,
			},
			wantStep:   diskSCStepComplete,
			wantErr:    true,
			wantErrSub: "timed out",
		},
		{
			name: "returns apply patch when spec differs from desired",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskReady,
				SpecSC:    "slow",
				StatusSC:  "slow",
				DesiredSC: "fast",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep: diskSCStepApplyPatch,
		},
		{
			name: "returns complete for legacy disk with empty status and no migration marker",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskReady,
				SpecSC:    "fast",
				StatusSC:  "",
				DesiredSC: "fast",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep: diskSCStepComplete,
		},
		{
			name: "returns error when desired storage class is empty and migration is needed",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskReady,
				SpecSC:    "slow",
				StatusSC:  "slow",
				DesiredSC: "",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep:   diskSCStepComplete,
			wantErr:    true,
			wantErrSub: "must not be empty",
		},
		{
			name: "returns complete when desired storage class is empty and disk already matches",
			in: diskSCMigrationEvalInput{
				Phase:     v1alpha2.DiskReady,
				SpecSC:    "",
				StatusSC:  "",
				DesiredSC: "",
				Now:       now,
				Timeout:   timeout,
			},
			wantStep: diskSCStepComplete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := evaluateDiskStorageClassMigration(tt.in)
			require.Equal(t, tt.wantStep, step)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrSub != "" {
					assert.Contains(t, err.Error(), tt.wantErrSub)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDiskMigrationStartedAnnotationKey(t *testing.T) {
	key := diskMigrationStartedAnnotationKey("worker-1-boot")
	assert.Equal(t, "dvp.deckhouse.io/disk-migration-started-worker-1-boot", key)

	startedAt, ok := parseDiskMigrationStartedAt(map[string]string{
		key: "2026-05-19T10:00:00Z",
	}, "worker-1-boot")
	require.True(t, ok)
	assert.Equal(t, 2026, startedAt.Year())
}

// The Machine watch exists for exactly one field. A predicate that lets the
// constant CAPI status rewrites through re-runs a pass that talks to the DVP
// API on every one of them; a predicate that misses the dataSecretName change
// parks the DeckhouseMachine until the requeue backstop.
func TestDataSecretNameChanged(t *testing.T) {
	machine := func(secretName *string, phase string) *clusterv1b2.Machine {
		m := &clusterv1b2.Machine{}
		m.Spec.Bootstrap.DataSecretName = secretName
		m.Status.Phase = phase
		return m
	}

	tests := []struct {
		name   string
		before *clusterv1b2.Machine
		after  *clusterv1b2.Machine
		want   bool
	}{
		{
			name:   "the bootstrap provider filled the secret in",
			before: machine(nil, ""),
			after:  machine(ptr.To("worker-abc-bootstrap"), ""),
			want:   true,
		},
		{
			name:   "the secret was replaced",
			before: machine(ptr.To("old"), ""),
			after:  machine(ptr.To("new"), ""),
			want:   true,
		},
		{
			name:   "a status-only rewrite is noise",
			before: machine(ptr.To("worker-abc-bootstrap"), "Provisioning"),
			after:  machine(ptr.To("worker-abc-bootstrap"), "Running"),
			want:   false,
		},
		{
			name:   "nothing set on either side",
			before: machine(nil, "Pending"),
			after:  machine(nil, "Provisioning"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dataSecretNameChanged(tt.before, tt.after); got != tt.want {
				t.Fatalf("dataSecretNameChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A machine without GPUs must leave the field absent from the VM manifest rather
// than send an empty list: DVP rejects `gpus: []` on a VM, and a machine that
// never asked for a GPU would stop being creatable.
func TestBuildGPUs(t *testing.T) {
	machine := func(gpus ...string) *infrastructurev1alpha1.DeckhouseMachine {
		m := &infrastructurev1alpha1.DeckhouseMachine{}
		for _, name := range gpus {
			m.Spec.GPUs = append(m.Spec.GPUs, infrastructurev1alpha1.GPUDevice{GPUClassName: name})
		}
		return m
	}

	tests := []struct {
		name string
		in   *infrastructurev1alpha1.DeckhouseMachine
		want []v1alpha2.GPUDeviceSpec
	}{
		{
			name: "no gpus requested",
			in:   machine(),
			want: nil,
		},
		{
			name: "single gpu",
			in:   machine("nvidia-h100"),
			want: []v1alpha2.GPUDeviceSpec{{GPUClassName: "nvidia-h100"}},
		},
		{
			name: "several devices of the same class",
			in:   machine("nvidia-h100", "nvidia-h100"),
			want: []v1alpha2.GPUDeviceSpec{{GPUClassName: "nvidia-h100"}, {GPUClassName: "nvidia-h100"}},
		},
		{
			name: "devices of different classes keep their order",
			in:   machine("nvidia-h100", "nvidia-a100"),
			want: []v1alpha2.GPUDeviceSpec{{GPUClassName: "nvidia-h100"}, {GPUClassName: "nvidia-a100"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildGPUs(tt.in))
		})
	}
}

// The reason a GPU machine is stuck lives in the parent cluster, where the owner of the
// DeckhouseMachine usually cannot look. handleGPUClassNotReady copies it onto the machine —
// but only for machines that asked for a GPU, and never as a FailureReason, because both
// GPUClassNotFound and GPUClassNotReady are fixed in the parent cluster without recreating
// the machine.
func TestHandleGPUClassNotReady(t *testing.T) {
	machine := func(gpus ...string) *infrastructurev1alpha1.DeckhouseMachine {
		m := &infrastructurev1alpha1.DeckhouseMachine{}
		for _, name := range gpus {
			m.Spec.GPUs = append(m.Spec.GPUs, infrastructurev1alpha1.GPUDevice{GPUClassName: name})
		}
		return m
	}

	vmWithCondition := func(status metav1.ConditionStatus, reason, message string) *v1alpha2.VirtualMachine {
		vm := &v1alpha2.VirtualMachine{}
		vm.Status.Conditions = []metav1.Condition{{
			Type:    vmcondition.TypeGPUClassReady.String(),
			Status:  status,
			Reason:  reason,
			Message: message,
		}}
		return vm
	}

	tests := []struct {
		name        string
		machine     *infrastructurev1alpha1.DeckhouseMachine
		vm          *v1alpha2.VirtualMachine
		wantHandled bool
		wantReason  string
		wantMessage string
	}{
		{
			name:        "machine without gpus is left to the generic path",
			machine:     machine(),
			vm:          vmWithCondition(metav1.ConditionFalse, vmcondition.ReasonGPUClassNotFound.String(), "no such GPUClass"),
			wantHandled: false,
		},
		{
			name:        "no GPUClassReady condition yet",
			machine:     machine("nvidia-h100"),
			vm:          &v1alpha2.VirtualMachine{},
			wantHandled: false,
		},
		{
			name:        "classes are ready, the VM waits for something else",
			machine:     machine("nvidia-h100"),
			vm:          vmWithCondition(metav1.ConditionTrue, vmcondition.ReasonGPUClassReady.String(), ""),
			wantHandled: false,
		},
		{
			name:        "missing class",
			machine:     machine("nvidia-h100"),
			vm:          vmWithCondition(metav1.ConditionFalse, vmcondition.ReasonGPUClassNotFound.String(), `GPUClass "nvidia-h100" not found`),
			wantHandled: true,
			wantReason:  infrastructurev1alpha1.GPUClassNotFoundReason,
			wantMessage: `GPUClass "nvidia-h100" not found. Requested GPU classes: nvidia-h100`,
		},
		{
			name:        "class exists but is not ready",
			machine:     machine("nvidia-h100"),
			vm:          vmWithCondition(metav1.ConditionFalse, vmcondition.ReasonGPUClassNotReady.String(), "GPUClass is not ready"),
			wantHandled: true,
			wantReason:  infrastructurev1alpha1.GPUClassNotReadyReason,
			wantMessage: "GPUClass is not ready. Requested GPU classes: nvidia-h100",
		},
		{
			name:        "the VM message already ends with a period",
			machine:     machine("nvidia-h100", "nvidia-t4"),
			vm:          vmWithCondition(metav1.ConditionFalse, vmcondition.ReasonGPUClassNotReady.String(), "GPUClass is not ready yet."),
			wantHandled: true,
			wantReason:  infrastructurev1alpha1.GPUClassNotReadyReason,
			wantMessage: "GPUClass is not ready yet. Requested GPU classes: nvidia-h100, nvidia-t4",
		},
		{
			name:        "unknown status counts as not ready",
			machine:     machine("nvidia-h100"),
			vm:          vmWithCondition(metav1.ConditionUnknown, "", ""),
			wantHandled: true,
			wantReason:  infrastructurev1alpha1.GPUClassNotReadyReason,
			wantMessage: fmt.Sprintf("VM condition %s is Unknown. Requested GPU classes: nvidia-h100",
				vmcondition.TypeGPUClassReady.String()),
		},
		{
			name:        "no message, but the reason is kept in the fallback",
			machine:     machine("nvidia-h100"),
			vm:          vmWithCondition(metav1.ConditionFalse, vmcondition.ReasonGPUClassNotReady.String(), ""),
			wantHandled: true,
			wantReason:  infrastructurev1alpha1.GPUClassNotReadyReason,
			wantMessage: fmt.Sprintf("VM condition %s is False (%s). Requested GPU classes: nvidia-h100",
				vmcondition.TypeGPUClassReady.String(), vmcondition.ReasonGPUClassNotReady.String()),
		},
	}

	r := &DeckhouseMachineReconciler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, handled := r.handleGPUClassNotReady(logr.Discard(), tt.machine, tt.vm)
			require.Equal(t, tt.wantHandled, handled)
			if !tt.wantHandled {
				assert.Empty(t, tt.machine.Status.Conditions)
				return
			}

			assert.Equal(t, 30*time.Second, result.RequeueAfter)

			cond := meta.FindStatusCondition(tt.machine.Status.Conditions, string(infrastructurev1alpha1.VMReadyCondition))
			require.NotNil(t, cond)
			assert.Equal(t, metav1.ConditionFalse, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.Contains(t, cond.Message, "nvidia-h100")
			assert.Equal(t, tt.wantMessage, cond.Message)
			assert.NotContains(t, cond.Message, "..", "an already terminated VM message must not gain a second period")
			assert.NotContains(t, cond.Message, "()", "a condition without a reason must not render empty parens")

			assert.Nil(t, tt.machine.Status.FailureReason, "a missing GPUClass is fixed in the parent cluster, the machine must stay recoverable")
			assert.Nil(t, tt.machine.Status.FailureMessage)
		})
	}
}

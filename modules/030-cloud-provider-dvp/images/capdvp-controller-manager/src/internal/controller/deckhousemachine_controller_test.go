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
	"testing"
	"time"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

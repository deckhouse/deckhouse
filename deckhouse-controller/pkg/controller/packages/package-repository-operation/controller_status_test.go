// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package packagerepositoryoperation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	fakeRegistry "github.com/deckhouse/deckhouse/pkg/registry/fake"
)

func TestUpdateRepositoryStatus(t *testing.T) {
	for _, tc := range []struct {
		name          string
		deleting      bool
		phase         string
		expectedPhase string
	}{
		{
			name:          "successful scan activates repository",
			expectedPhase: v1alpha1.PackageRepositoryPhaseActive,
		},
		{
			name:          "deleting repository keeps terminating phase",
			deleting:      true,
			phase:         v1alpha1.PackageRepositoryPhaseTerminating,
			expectedPhase: v1alpha1.PackageRepositoryPhaseTerminating,
		},
		{
			name:     "deleting repository is not activated before terminating phase is set",
			deleting: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			repo := &v1alpha1.PackageRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-repository",
					Finalizers: []string{v1alpha1.PackageRepositoryFinalizerPackageVersionExists},
				},
				Status: v1alpha1.PackageRepositoryStatus{Phase: tc.phase},
			}
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			cl := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&v1alpha1.PackageRepository{}).WithObjects(repo).Build()
			if tc.deleting {
				require.NoError(t, cl.Delete(ctx, repo))
			}
			require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), repo))
			svc := &OperationService{client: cl, repo: repo}

			err := svc.UpdateRepositoryStatus(ctx, []v1alpha1.PackageRepositoryOperationStatusPackage{
				{Name: "test-package", Type: "Application", NewVersions: 2},
			})
			require.NoError(t, err)

			actual := new(v1alpha1.PackageRepository)
			require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), actual))
			assert.Equal(t, tc.expectedPhase, actual.Status.Phase)
			assert.Equal(t, []v1alpha1.PackageRepositoryStatusPackage{
				{Name: "test-package", Type: "Application"},
			}, actual.Status.Packages)
			assert.Equal(t, 1, actual.Status.PackagesCount)
			assert.Equal(t, 2, actual.Status.LastNewVersions)
			assert.NotNil(t, actual.Status.LastScanTime)
			assert.NotNil(t, actual.Status.LastChangeTime)
		})
	}
}

func TestUpdateRepositoryStatusRejectsStaleRepository(t *testing.T) {
	for _, phase := range []string{"", v1alpha1.PackageRepositoryPhaseActive} {
		t.Run("phase="+phase, func(t *testing.T) {
			ctx := t.Context()
			repo := &v1alpha1.PackageRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-repository",
					Finalizers: []string{v1alpha1.PackageRepositoryFinalizerPackageVersionExists},
				},
				Status: v1alpha1.PackageRepositoryStatus{Phase: phase},
			}
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))
			cl := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&v1alpha1.PackageRepository{}).WithObjects(repo).Build()
			require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), repo))
			stale := repo.DeepCopy()
			require.NoError(t, cl.Delete(ctx, repo))
			require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), repo))
			repo.Status.Phase = v1alpha1.PackageRepositoryPhaseTerminating
			require.NoError(t, cl.Status().Update(ctx, repo))
			svc := &OperationService{client: cl, repo: stale}

			err := svc.UpdateRepositoryStatus(ctx, []v1alpha1.PackageRepositoryOperationStatusPackage{
				{Name: "test-package", Type: "Application", NewVersions: 1},
			})
			require.Error(t, err)
			assert.True(t, apierrors.IsConflict(err), "expected a resource version conflict, got %v", err)

			actual := new(v1alpha1.PackageRepository)
			require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), actual))
			assert.Equal(t, repo.Status, actual.Status)
			assert.Equal(t, v1alpha1.PackageRepositoryPhaseTerminating, actual.Status.Phase)
			assert.False(t, actual.DeletionTimestamp.IsZero())
		})
	}
}

func TestReconcileRetriesRepositoryStatusConflict(t *testing.T) {
	ctx := t.Context()
	repo := &v1alpha1.PackageRepository{
		ObjectMeta: metav1.ObjectMeta{Name: "test-repository"},
		Spec: v1alpha1.PackageRepositorySpec{
			Registry: v1alpha1.PackageRepositorySpecRegistry{Repo: registryHost},
		},
	}
	op := &v1alpha1.PackageRepositoryOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-scan",
			Labels: map[string]string{
				v1alpha1.PackagesRepositoryOperationLabelOperationTrigger: v1alpha1.PackagesRepositoryTriggerManual,
				v1alpha1.PackagesRepositoryOperationLabelOperationType:    string(v1alpha1.PackageRepositoryOperationTypeUpdate),
				v1alpha1.PackagesRepositoryOperationLabelRepository:       repo.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       v1alpha1.PackageRepositoryKind,
				Name:       repo.Name,
			}},
		},
		Spec: v1alpha1.PackageRepositoryOperationSpec{
			PackageRepositoryName: repo.Name,
			Type:                  v1alpha1.PackageRepositoryOperationTypeUpdate,
		},
		Status: v1alpha1.PackageRepositoryOperationStatus{
			Packages: &v1alpha1.PackageRepositoryOperationStatusPackages{
				Total: 1,
				Processed: []v1alpha1.PackageRepositoryOperationStatusPackage{
					{Name: "test-package", Type: "Application", NewVersions: 2},
				},
			},
			Conditions: []metav1.Condition{{
				Type:   v1alpha1.PackageRepositoryOperationConditionCompleted,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.PackageRepositoryOperationReasonProcessing,
			}},
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	metadataUpdated := false
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&v1alpha1.PackageRepository{}, &v1alpha1.PackageRepositoryOperation{}).
		WithObjects(repo, op).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(ctx context.Context, cl client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				if _, ok := obj.(*v1alpha1.PackageRepository); ok && !metadataUpdated {
					latest := new(v1alpha1.PackageRepository)
					require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(obj), latest))
					latest.Labels = map[string]string{"test.deckhouse.io/updated": "true"}
					require.NoError(t, cl.Update(ctx, latest))
					metadataUpdated = true
				}
				return cl.SubResource(subresource).Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := &reconciler{
		client: cl,
		dc:     dependency.NewMockedContainer(),
		psm:    createFakePSM(newInternalClient(fakeRegistry.NewRegistry(registryHost))),
		logger: log.NewNop(),
	}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(op)}

	_, err := r.Reconcile(ctx, req)
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err), "expected a resource version conflict, got %v", err)
	require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(op), op))
	assert.Equal(t, v1alpha1.PackageRepositoryOperationReasonProcessing, op.GetStateByCondition())
	assert.False(t, op.IsCompleted())
	assert.Nil(t, op.Status.CompletionTime)
	require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), repo))
	assert.Nil(t, repo.Status.LastScanTime)
	assert.Empty(t, repo.Status.Packages)

	_, err = r.Reconcile(ctx, req)
	require.NoError(t, err)
	require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(op), op))
	assert.True(t, op.IsCompleted())
	assert.Equal(t, v1alpha1.PackageRepositoryOperationReasonScanSucceeded, op.GetStateByCondition())
	assert.NotNil(t, op.Status.CompletionTime)
	require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(repo), repo))
	assert.Equal(t, "true", repo.Labels["test.deckhouse.io/updated"])
	assert.Equal(t, v1alpha1.PackageRepositoryPhaseActive, repo.Status.Phase)
	assert.Equal(t, []v1alpha1.PackageRepositoryStatusPackage{
		{Name: "test-package", Type: "Application"},
	}, repo.Status.Packages)
	assert.Equal(t, 1, repo.Status.PackagesCount)
	assert.Equal(t, 2, repo.Status.LastNewVersions)
	assert.NotNil(t, repo.Status.LastScanTime)
}

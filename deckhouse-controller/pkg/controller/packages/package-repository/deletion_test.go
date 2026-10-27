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

package packagerepository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestReconcileDeletion(t *testing.T) {
	const holdFinalizer = "test.deckhouse.io/hold-deletion"

	scheme, err := project.Scheme()
	require.NoError(t, err)

	for _, tc := range []struct {
		name           string
		initialPhase   string
		failCleanup    bool
		failPhasePatch bool
		wantError      string
	}{
		{name: "active repository", initialPhase: v1alpha1.PackageRepositoryPhaseActive},
		{name: "repository before first scan"},
		{
			name:         "cleanup failure preserves terminating phase and finalizer",
			initialPhase: v1alpha1.PackageRepositoryPhaseActive,
			failCleanup:  true,
			wantError:    "list application packages: cleanup failed",
		},
		{
			name:           "phase patch failure prevents cleanup",
			initialPhase:   v1alpha1.PackageRepositoryPhaseActive,
			failPhasePatch: true,
			wantError:      "set terminating phase: status patch failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scanTime := metav1.NewTime(time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC))
			repo := &v1alpha1.PackageRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "deckhouse",
					Finalizers: []string{v1alpha1.PackageRepositoryFinalizerPackageVersionExists, holdFinalizer},
				},
				Status: v1alpha1.PackageRepositoryStatus{
					Phase:           tc.initialPhase,
					LastScanTime:    &scanTime,
					LastChangeTime:  &scanTime,
					LastNewVersions: 2,
					Packages:        []v1alpha1.PackageRepositoryStatusPackage{{Name: "example", Type: "Module"}},
					PackagesCount:   1,
					Message:         "scan completed",
					Conditions: []metav1.Condition{{
						Type:               v1alpha1.PackageRepositoryConditionLastScanSucceeded,
						Status:             metav1.ConditionTrue,
						Reason:             "ScanSucceeded",
						LastTransitionTime: scanTime,
					}},
				},
			}
			var cleanupCalls, phasePatchCalls int
			cl := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&v1alpha1.PackageRepository{}).
				WithObjects(repo).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
						cleanupCalls++
						if tc.failCleanup {
							return errors.New("cleanup failed")
						}
						return cl.List(ctx, list, opts...)
					},
					SubResourcePatch: func(ctx context.Context, cl client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
						if _, ok := obj.(*v1alpha1.PackageRepository); ok && subResource == "status" {
							phasePatchCalls++
							if tc.failPhasePatch {
								return errors.New("status patch failed")
							}
						}
						return cl.SubResource(subResource).Patch(ctx, obj, patch, opts...)
					},
				}).Build()
			r := &reconciler{client: cl, logger: log.NewNop()}
			key := client.ObjectKeyFromObject(repo)
			require.NoError(t, cl.Get(ctx, key, repo))
			wantStatus := repo.DeepCopy().Status
			if !tc.failPhasePatch {
				wantStatus.Phase = v1alpha1.PackageRepositoryPhaseTerminating
			}

			require.NoError(t, cl.Delete(ctx, repo))
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
			}

			after := new(v1alpha1.PackageRepository)
			require.NoError(t, cl.Get(ctx, key, after))
			assert.False(t, after.DeletionTimestamp.IsZero())
			assert.Equal(t, wantStatus, after.Status)
			assert.Contains(t, after.Finalizers, holdFinalizer)
			assert.Equal(t, 1, phasePatchCalls)

			if tc.wantError != "" {
				assert.Contains(t, after.Finalizers, v1alpha1.PackageRepositoryFinalizerPackageVersionExists)
			} else {
				assert.NotContains(t, after.Finalizers, v1alpha1.PackageRepositoryFinalizerPackageVersionExists)
			}
			switch {
			case tc.failPhasePatch:
				assert.Zero(t, cleanupCalls)
			case tc.failCleanup:
				assert.Equal(t, 1, cleanupCalls)
			default:
				assert.Equal(t, 2, cleanupCalls)
				_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
				require.NoError(t, err)
				assert.Equal(t, 1, phasePatchCalls)
			}
		})
	}
}

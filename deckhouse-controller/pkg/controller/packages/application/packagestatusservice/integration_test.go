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

package packagestatusservice

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	applicationpackage "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/application-package"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// mockPackageStatusOperator implements PackageStatusOperator for testing.
type mockPackageStatusOperator struct {
	statuses map[string][]applicationpackage.PackageStatus
	errors   map[string]error
}

func newMockPackageStatusOperator() *mockPackageStatusOperator {
	return &mockPackageStatusOperator{
		statuses: make(map[string][]applicationpackage.PackageStatus),
		errors:   make(map[string]error),
	}
}

func (m *mockPackageStatusOperator) setStatus(key string, statuses []applicationpackage.PackageStatus) {
	m.statuses[key] = statuses
}

func (m *mockPackageStatusOperator) setError(key string, err error) {
	m.errors[key] = err
}

func (m *mockPackageStatusOperator) GetApplicationStatus(ctx context.Context, packageName, appName, namespace string) ([]applicationpackage.PackageStatus, error) {
	key := namespace + "/" + appName
	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if statuses, ok := m.statuses[key]; ok {
		return statuses, nil
	}
	return []applicationpackage.PackageStatus{}, nil
}

func TestServiceIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	tests := []struct {
		name   string
		setup  func(*testing.T, client.Client, *mockPackageStatusOperator)
		event  PackageEvent
		verify func(*testing.T, client.Client)
	}{
		{
			name: "happy path - create conditions",
			setup: func(t *testing.T, kube client.Client, op *mockPackageStatusOperator) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main",
						Namespace: "foobar",
					},
					Spec: v1alpha1.ApplicationSpec{
						ApplicationPackageName: "postgres",
						Version:                "1.0.0",
					},
					Status: v1alpha1.ApplicationStatus{
						Conditions: []v1alpha1.ApplicationStatusCondition{},
					},
				}
				require.NoError(t, kube.Create(context.Background(), app))

				op.setStatus("foobar/main", []applicationpackage.PackageStatus{
					{Type: "requirementsMet", Status: true, Reason: "AllRequirementsMet", Message: "OK"},
					{Type: "manifestsDeployed", Status: true, Reason: "Deployed", Message: "OK"},
				})
			},
			event: PackageEvent{
				PackageName: "postgres",
				Name:        "main",
				Namespace:   "foobar",
				Version:     "1.0.0",
				Type:        "Created",
			},
			verify: func(t *testing.T, kube client.Client) {
				var app v1alpha1.Application
				require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: "main", Namespace: "foobar"}, &app))

				require.Len(t, app.Status.Conditions, 2)
				// Conditions should be sorted
				assert.Equal(t, v1alpha1.ApplicationConditionManifestsDeployed, app.Status.Conditions[0].Type)
				assert.Equal(t, corev1.ConditionTrue, app.Status.Conditions[0].Status)
				assert.Equal(t, v1alpha1.ApplicationConditionRequirementsMet, app.Status.Conditions[1].Type)
				assert.Equal(t, corev1.ConditionTrue, app.Status.Conditions[1].Status)
				assert.False(t, app.Status.Conditions[0].LastTransitionTime.IsZero())
				assert.False(t, app.Status.Conditions[1].LastTransitionTime.IsZero())
			},
		},
		{
			name: "update condition - status changes",
			setup: func(t *testing.T, kube client.Client, op *mockPackageStatusOperator) {
				past := metav1.NewTime(time.Now().Add(-1 * time.Hour))
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main",
						Namespace: "foobar",
					},
					Spec: v1alpha1.ApplicationSpec{
						ApplicationPackageName: "postgres",
						Version:                "1.0.0",
					},
					Status: v1alpha1.ApplicationStatus{
						Conditions: []v1alpha1.ApplicationStatusCondition{
							{
								Type:               v1alpha1.ApplicationConditionRequirementsMet,
								Status:             corev1.ConditionFalse,
								Reason:             "Old",
								Message:            "Old",
								LastTransitionTime: past,
							},
						},
					},
				}
				require.NoError(t, kube.Create(context.Background(), app))

				op.setStatus("foobar/main", []applicationpackage.PackageStatus{
					{Type: "requirementsMet", Status: true, Reason: "New", Message: "New"},
				})
			},
			event: PackageEvent{
				PackageName: "postgres",
				Name:        "main",
				Namespace:   "foobar",
				Type:        "Updated",
			},
			verify: func(t *testing.T, kube client.Client) {
				var app v1alpha1.Application
				require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: "main", Namespace: "foobar"}, &app))

				require.Len(t, app.Status.Conditions, 1)
				assert.Equal(t, corev1.ConditionTrue, app.Status.Conditions[0].Status)
				assert.Equal(t, "New", app.Status.Conditions[0].Reason)
				// LastTransitionTime should be updated
				assert.True(t, app.Status.Conditions[0].LastTransitionTime.After(time.Now().Add(-1*time.Minute)))
			},
		},
		{
			name: "update condition - only reason/message changes",
			setup: func(t *testing.T, kube client.Client, op *mockPackageStatusOperator) {
				past := metav1.NewTime(time.Now().Add(-1 * time.Hour))
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main",
						Namespace: "foobar",
					},
					Spec: v1alpha1.ApplicationSpec{
						ApplicationPackageName: "postgres",
						Version:                "1.0.0",
					},
					Status: v1alpha1.ApplicationStatus{
						Conditions: []v1alpha1.ApplicationStatusCondition{
							{
								Type:               v1alpha1.ApplicationConditionRequirementsMet,
								Status:             corev1.ConditionTrue,
								Reason:             "Old",
								Message:            "Old",
								LastTransitionTime: past,
							},
						},
					},
				}
				require.NoError(t, kube.Create(context.Background(), app))

				op.setStatus("foobar/main", []applicationpackage.PackageStatus{
					{Type: "requirementsMet", Status: true, Reason: "New", Message: "New"},
				})
			},
			event: PackageEvent{
				PackageName: "postgres",
				Name:        "main",
				Namespace:   "foobar",
				Type:        "Updated",
			},
			verify: func(t *testing.T, kube client.Client) {
				var app v1alpha1.Application
				require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: "main", Namespace: "foobar"}, &app))

				require.Len(t, app.Status.Conditions, 1)
				assert.Equal(t, "New", app.Status.Conditions[0].Reason)
				assert.Equal(t, "New", app.Status.Conditions[0].Message)
				// LastTransitionTime should NOT change (should be old)
				assert.True(t, app.Status.Conditions[0].LastTransitionTime.Time.Before(time.Now().Add(-30*time.Minute)))
			},
		},
		{
			name: "application not found - event is dropped",
			setup: func(t *testing.T, kube client.Client, op *mockPackageStatusOperator) {
				// Don't create application
			},
			event: PackageEvent{
				PackageName: "postgres",
				Name:        "main",
				Namespace:   "foobar",
				Type:        "Created",
			},
			verify: func(t *testing.T, kube client.Client) {
				var app v1alpha1.Application
				err := kube.Get(context.Background(), types.NamespacedName{Name: "main", Namespace: "foobar"}, &app)
				assert.Error(t, err)
			},
		},
		{
			name: "no changes - no patch",
			setup: func(t *testing.T, kube client.Client, op *mockPackageStatusOperator) {
				past := metav1.NewTime(time.Now().Add(-1 * time.Hour))
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main",
						Namespace: "foobar",
					},
					Spec: v1alpha1.ApplicationSpec{
						ApplicationPackageName: "postgres",
						Version:                "1.0.0",
					},
					Status: v1alpha1.ApplicationStatus{
						Conditions: []v1alpha1.ApplicationStatusCondition{
							{
								Type:               v1alpha1.ApplicationConditionRequirementsMet,
								Status:             corev1.ConditionTrue,
								Reason:             "OK",
								Message:            "OK",
								LastTransitionTime: past,
							},
						},
					},
				}
				require.NoError(t, kube.Create(context.Background(), app))

				op.setStatus("foobar/main", []applicationpackage.PackageStatus{
					{Type: "requirementsMet", Status: true, Reason: "OK", Message: "OK"},
				})
			},
			event: PackageEvent{
				PackageName: "postgres",
				Name:        "main",
				Namespace:   "foobar",
				Type:        "Updated",
			},
			verify: func(t *testing.T, kube client.Client) {
				var app v1alpha1.Application
				require.NoError(t, kube.Get(context.Background(), types.NamespacedName{Name: "main", Namespace: "foobar"}, &app))

				require.Len(t, app.Status.Conditions, 1)
				// LastTransitionTime should remain unchanged
				assert.True(t, app.Status.Conditions[0].LastTransitionTime.Time.Before(time.Now().Add(-30*time.Minute)))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&v1alpha1.Application{}).
				Build()

			op := newMockPackageStatusOperator()
			tt.setup(t, kubeClient, op)

			events := make(chan PackageEvent, 1)
			events <- tt.event
			close(events)

			service := NewPackageStatusService(
				log.NewNop(),
				kubeClient,
				op,
				events,
				nil, // no metrics for integration test
				1,   // single worker
			)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Run service in background
			done := make(chan struct{})
			go func() {
				service.Run(ctx)
				close(done)
			}()

			// Give service time to process event
			time.Sleep(500 * time.Millisecond)

			// Cancel context to stop service
			cancel()

			// Wait for service to stop
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("service did not stop in time")
			}

			tt.verify(t, kubeClient)
		})
	}
}

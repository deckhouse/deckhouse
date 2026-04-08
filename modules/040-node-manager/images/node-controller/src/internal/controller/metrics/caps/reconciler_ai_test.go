//go:build ai_tests

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

package caps

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = mcmv1alpha1.AddToScheme(s)
	return s
}

func resetMetrics() {
	capsReplicas.Reset()
	capsDesired.Reset()
	capsReady.Reset()
	capsUnavailable.Reset()
	capsPhase.Reset()
}

// TestAI_MDFoundMetricsSet verifies that when a CAPS MachineDeployment is found,
// metrics are set with correct values.
func TestAI_MDFoundMetricsSet(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	md := &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Labels:    map[string]string{"app": "caps-controller-manager"},
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: 3,
		},
		Status: mcmv1alpha1.MachineDeploymentStatus{
			Replicas:            3,
			ReadyReplicas:       2,
			UnavailableReplicas: 1,
			Conditions: []mcmv1alpha1.MachineDeploymentCondition{
				{
					Type:   mcmv1alpha1.MachineDeploymentAvailable,
					Status: mcmv1alpha1.ConditionTrue,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(md).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-md", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	labels := prometheus.Labels{"machine_deployment_name": "test-md"}
	assert.Equal(t, float64(3), testutil.ToFloat64(capsReplicas.With(labels)))
	assert.Equal(t, float64(3), testutil.ToFloat64(capsDesired.With(labels)))
	assert.Equal(t, float64(2), testutil.ToFloat64(capsReady.With(labels)))
	assert.Equal(t, float64(1), testutil.ToFloat64(capsUnavailable.With(labels)))
	assert.Equal(t, float64(1), testutil.ToFloat64(capsPhase.With(labels))) // 1 = Running
}

// TestAI_MDNotFoundSkip verifies that reconciling a non-existent MachineDeployment
// returns no error and clears metrics.
func TestAI_MDNotFoundSkip(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_MDDeletedMetricsCleared verifies that when a MachineDeployment is deleted,
// its metrics are removed.
func TestAI_MDDeletedMetricsCleared(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	md := &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "delete-md",
			Namespace: "default",
			Labels:    map[string]string{"app": "caps-controller-manager"},
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: 2,
		},
		Status: mcmv1alpha1.MachineDeploymentStatus{
			Replicas:      2,
			ReadyReplicas: 2,
			Conditions: []mcmv1alpha1.MachineDeploymentCondition{
				{
					Type:   mcmv1alpha1.MachineDeploymentAvailable,
					Status: mcmv1alpha1.ConditionTrue,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(md).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	// First reconcile: set metrics
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "delete-md", Namespace: "default"},
	})
	require.NoError(t, err)

	labels := prometheus.Labels{"machine_deployment_name": "delete-md"}
	assert.Equal(t, float64(2), testutil.ToFloat64(capsReplicas.With(labels)))

	// Delete the object and reconcile again
	err = fakeClient.Delete(context.Background(), md)
	require.NoError(t, err)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "delete-md", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// After deletion, the metric series for "delete-md" should be deleted.
	// Collecting from the GaugeVec should yield 0 metrics for that label set.
	assert.Equal(t, 0, testutil.CollectAndCount(capsReplicas))
}

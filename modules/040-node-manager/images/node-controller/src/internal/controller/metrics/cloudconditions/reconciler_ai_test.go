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

package cloudconditions

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func resetMetrics() {
	unmetCloudConditionsGauge.Reset()
	cloudConditionStatusGauge.Reset()
}

// TestAI_ConfigMapWithConditionsMetricSet verifies that when the cloud conditions
// ConfigMap exists with conditions, metrics are set with correct values.
func TestAI_ConfigMapWithConditionsMetricSet(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
		},
		Data: map[string]string{
			"conditions": `[{"name":"quota-check","message":"Quota is sufficient","ok":true},{"name":"network-check","message":"Network unreachable","ok":false}]`,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: configMapName, Namespace: configMapNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// There are unmet conditions (network-check is not ok)
	assert.Equal(t, float64(1), testutil.ToFloat64(unmetCloudConditionsGauge.With(prometheus.Labels{})))

	// quota-check: ok=true -> status=1
	assert.Equal(t, float64(1), testutil.ToFloat64(cloudConditionStatusGauge.With(prometheus.Labels{
		"name":    "quota-check",
		"message": "Quota is sufficient",
	})))

	// network-check: ok=false -> status=0
	assert.Equal(t, float64(0), testutil.ToFloat64(cloudConditionStatusGauge.With(prometheus.Labels{
		"name":    "network-check",
		"message": "Network unreachable",
	})))
}

// TestAI_ConfigMapWithAllConditionsMet verifies that when all conditions are met,
// unmet_cloud_conditions is set to 0.
func TestAI_ConfigMapWithAllConditionsMet(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
		},
		Data: map[string]string{
			"conditions": `[{"name":"quota-check","message":"All good","ok":true}]`,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: configMapName, Namespace: configMapNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	assert.Equal(t, float64(0), testutil.ToFloat64(unmetCloudConditionsGauge.With(prometheus.Labels{})))
	assert.Equal(t, float64(1), testutil.ToFloat64(cloudConditionStatusGauge.With(prometheus.Labels{
		"name":    "quota-check",
		"message": "All good",
	})))
}

// TestAI_ConfigMapNotFoundSkip verifies that when the ConfigMap does not exist,
// the metrics are reset and unmet_cloud_conditions is set to 0.
func TestAI_ConfigMapNotFoundSkip(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: configMapName, Namespace: configMapNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// When ConfigMap is not found, unmet conditions should be 0
	assert.Equal(t, float64(0), testutil.ToFloat64(unmetCloudConditionsGauge.With(prometheus.Labels{})))
	// No per-condition metrics should exist
	assert.Equal(t, 0, testutil.CollectAndCount(cloudConditionStatusGauge))
}

// TestAI_ConfigMapWithEmptyConditions verifies that when the ConfigMap exists
// but has no conditions data, metrics are reset.
func TestAI_ConfigMapWithEmptyConditions(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
		},
		Data: map[string]string{},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: configMapName, Namespace: configMapNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	assert.Equal(t, float64(0), testutil.ToFloat64(unmetCloudConditionsGauge.With(prometheus.Labels{})))
	assert.Equal(t, 0, testutil.CollectAndCount(cloudConditionStatusGauge))
}

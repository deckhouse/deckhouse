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

package nodegroupconfigurations

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

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = deckhousev1.AddToScheme(s)
	return s
}

// TestAI_NGCFoundMetricSet verifies that when NodeGroupConfigurations exist,
// the count metric is set with correct values per node group.
func TestAI_NGCFoundMetricSet(t *testing.T) {
	ngConfigTotal.Reset()
	scheme := newTestScheme()

	ngc1 := &deckhousev1.NodeGroupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ngc-1",
		},
		Spec: deckhousev1.NodeGroupConfigurationSpec{
			Content:    "#!/bin/bash\necho hello",
			NodeGroups: []string{"worker", "master"},
		},
	}

	ngc2 := &deckhousev1.NodeGroupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ngc-2",
		},
		Spec: deckhousev1.NodeGroupConfigurationSpec{
			Content:    "#!/bin/bash\necho world",
			NodeGroups: []string{"worker"},
		},
	}

	ngc3 := &deckhousev1.NodeGroupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ngc-3",
		},
		Spec: deckhousev1.NodeGroupConfigurationSpec{
			Content:    "#!/bin/bash\necho all",
			NodeGroups: []string{}, // applies to all groups -> "*"
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ngc1, ngc2, ngc3).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ngc-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// worker: ngc-1 + ngc-2 = 2
	assert.Equal(t, float64(2), testutil.ToFloat64(ngConfigTotal.With(prometheus.Labels{"node_group": "worker"})))
	// master: ngc-1 = 1
	assert.Equal(t, float64(1), testutil.ToFloat64(ngConfigTotal.With(prometheus.Labels{"node_group": "master"})))
	// *: ngc-3 = 1
	assert.Equal(t, float64(1), testutil.ToFloat64(ngConfigTotal.With(prometheus.Labels{"node_group": "*"})))
}

// TestAI_NoNGCsMetricZero verifies that when no NodeGroupConfigurations exist,
// the metric is reset (zero series).
func TestAI_NoNGCsMetricZero(t *testing.T) {
	ngConfigTotal.Reset()
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "anything"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// No series should exist after reset
	assert.Equal(t, 0, testutil.CollectAndCount(ngConfigTotal))
}

// TestAI_NGCNotFoundTriggersFullRecalc verifies that reconciling when the
// specific NGC is not found still triggers a full recalculation. Since the
// reconciler lists all NGCs regardless of the request, it handles missing objects gracefully.
func TestAI_NGCNotFoundTriggersFullRecalc(t *testing.T) {
	ngConfigTotal.Reset()
	scheme := newTestScheme()

	// One existing NGC
	ngc := &deckhousev1.NodeGroupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "existing-ngc",
		},
		Spec: deckhousev1.NodeGroupConfigurationSpec{
			Content:    "#!/bin/bash\necho test",
			NodeGroups: []string{"infra"},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ngc).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	// Reconcile with a non-existent NGC name - the reconciler lists all NGCs anyway
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent-ngc"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// The existing NGC should still be counted
	assert.Equal(t, float64(1), testutil.ToFloat64(ngConfigTotal.With(prometheus.Labels{"node_group": "infra"})))
}

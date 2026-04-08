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

package containerd

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
	cntrdV2Unsupported.Reset()
	cgroupV2Unsupported.Reset()
}

// TestAI_NodeWithContainerdUnsupportedLabel verifies that when a node has
// the containerd-v2-unsupported label, the d8_nodes_cntrd_v2_unsupported metric is set to 1.
func TestAI_NodeWithContainerdUnsupportedLabel(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				nodeGroupLabel:               "worker",
				containerdV2UnsupportedLabel: "true",
				cgroupLabel:                  "cgroupfs",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	labels := prometheus.Labels{
		"node":           "node-1",
		"node_group":     "worker",
		"cgroup_version": "cgroupfs",
	}
	assert.Equal(t, float64(1), testutil.ToFloat64(cntrdV2Unsupported.With(labels)))
	assert.Equal(t, float64(1), testutil.ToFloat64(cgroupV2Unsupported.With(labels)))
}

// TestAI_NodeWithCgroupV2 verifies that when a node has cgroup2fs, the
// d8_node_cgroup_v2_unsupported metric is not set, and if no containerd-v2-unsupported
// label, the d8_nodes_cntrd_v2_unsupported metric is also not set.
func TestAI_NodeWithCgroupV2(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
			Labels: map[string]string{
				nodeGroupLabel: "master",
				cgroupLabel:    cgroupV2Value,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-2"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// No containerd unsupported label and cgroup is v2 -> both metrics should have 0 series
	assert.Equal(t, 0, testutil.CollectAndCount(cntrdV2Unsupported))
	assert.Equal(t, 0, testutil.CollectAndCount(cgroupV2Unsupported))
}

// TestAI_NodeNotFoundSkip verifies that reconciling a non-existent Node
// returns no error (triggers reconcileAll which rebuilds all metrics).
func TestAI_NodeNotFoundSkip(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Metrics should be reset with no series
	assert.Equal(t, 0, testutil.CollectAndCount(cntrdV2Unsupported))
	assert.Equal(t, 0, testutil.CollectAndCount(cgroupV2Unsupported))
}

// TestAI_NodeWithoutNodeGroupLabel verifies that a node without the
// node.deckhouse.io/group label is skipped.
func TestAI_NodeWithoutNodeGroupLabel(t *testing.T) {
	resetMetrics()
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-no-ng",
			Labels: map[string]string{
				containerdV2UnsupportedLabel: "true",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-no-ng"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Node without node group label should be skipped - no metrics set
	assert.Equal(t, 0, testutil.CollectAndCount(cntrdV2Unsupported))
}

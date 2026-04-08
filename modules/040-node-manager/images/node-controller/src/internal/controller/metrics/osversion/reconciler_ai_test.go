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

package osversion

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

// TestAI_NodeWithUbuntuOSImage verifies that when nodes with Ubuntu OS images exist,
// the minimal Ubuntu version metric is set correctly.
func TestAI_NodeWithUbuntuOSImage(t *testing.T) {
	minOSVersionGauge.Reset()
	scheme := newTestScheme()

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{nodeGroupLabel: "worker"},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OSImage: "Ubuntu 22.04 LTS",
			},
		},
	}

	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-2",
			Labels: map[string]string{nodeGroupLabel: "worker"},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OSImage: "Ubuntu 20.04.3 LTS",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1, node2).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Minimum Ubuntu version: 20.04.3 -> normalized to 20.4.3
	labels := prometheus.Labels{"os": "ubuntu", "version": "20.4.3"}
	assert.Equal(t, float64(1), testutil.ToFloat64(minOSVersionGauge.With(labels)))
}

// TestAI_NodeWithDebianOSImage verifies that when nodes with Debian OS images exist,
// the minimal Debian version metric is set correctly.
// Note: Debian versions must be valid semver (X.Y.Z) to be parsed by blang/semver.
func TestAI_NodeWithDebianOSImage(t *testing.T) {
	minOSVersionGauge.Reset()
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-deb",
			Labels: map[string]string{nodeGroupLabel: "master"},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OSImage: "Debian GNU/Linux 11.0.0 bullseye",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-deb"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	labels := prometheus.Labels{"os": "debian", "version": "11.0.0"}
	assert.Equal(t, float64(1), testutil.ToFloat64(minOSVersionGauge.With(labels)))
}

// TestAI_NodeNotFoundSkip verifies that reconciling when the node is not found
// does not produce an error (it lists all nodes regardless).
func TestAI_NodeNotFoundSkip(t *testing.T) {
	minOSVersionGauge.Reset()
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// No nodes -> no metrics
	assert.Equal(t, 0, testutil.CollectAndCount(minOSVersionGauge))
}

// TestAI_MixedUbuntuAndDebianNodes verifies that both Ubuntu and Debian minimum
// versions are tracked independently.
func TestAI_MixedUbuntuAndDebianNodes(t *testing.T) {
	minOSVersionGauge.Reset()
	scheme := newTestScheme()

	ubuntuNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ubuntu-node",
			Labels: map[string]string{nodeGroupLabel: "worker"},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OSImage: "Ubuntu 22.04 LTS",
			},
		},
	}

	debianNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "debian-node",
			Labels: map[string]string{nodeGroupLabel: "worker"},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OSImage: "Debian GNU/Linux 12.0.0 bookworm",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ubuntuNode, debianNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ubuntu-node"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	ubuntuLabels := prometheus.Labels{"os": "ubuntu", "version": "22.4.0"}
	assert.Equal(t, float64(1), testutil.ToFloat64(minOSVersionGauge.With(ubuntuLabels)))

	debianLabels := prometheus.Labels{"os": "debian", "version": "12.0.0"}
	assert.Equal(t, float64(1), testutil.ToFloat64(minOSVersionGauge.With(debianLabels)))
}

// TestAI_NonSemverDebianVersionSkipped verifies that Debian versions that are not
// valid semver (e.g. "11.0" without patch) are skipped gracefully.
func TestAI_NonSemverDebianVersionSkipped(t *testing.T) {
	minOSVersionGauge.Reset()
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-deb-nosemver",
			Labels: map[string]string{nodeGroupLabel: "worker"},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OSImage: "Debian GNU/Linux 11 bullseye",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-deb-nosemver"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// "11" is not valid semver, so no Debian metric should be set
	assert.Equal(t, 0, testutil.CollectAndCount(minOSVersionGauge))
}

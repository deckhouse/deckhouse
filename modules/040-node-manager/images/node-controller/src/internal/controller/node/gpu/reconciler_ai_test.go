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

package gpu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	return s
}

// TestAI_GPULabelsSetOnNewNode verifies that GPU labels are applied to a node
// that belongs to a NodeGroup with GPU mode configured but does not yet have GPU labels.
func TestAI_GPULabelsSetOnNewNode(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu-0",
			Labels: map[string]string{
				"node.deckhouse.io/group": "worker-gpu",
			},
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			GPU: &deckhousev1.GPUSpec{
				Mode: "time-slicing",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, ng).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-gpu-0"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-gpu-0"}, updatedNode)
	require.NoError(t, err)

	assert.Equal(t, "", updatedNode.Labels[gpuEnabledLabel])
	assert.Equal(t, "time-slicing", updatedNode.Labels[devicePluginLabel])
}

// TestAI_GPULabelsUpdatedWhenModeChanges verifies that when a node already has GPU labels
// but with a different mode, the device plugin label is updated to match the NodeGroup.
func TestAI_GPULabelsUpdatedWhenModeChanges(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu-1",
			Labels: map[string]string{
				"node.deckhouse.io/group":             "worker-gpu",
				"node.deckhouse.io/gpu":               "",
				"node.deckhouse.io/device-gpu.config": "exclusive",
			},
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			GPU: &deckhousev1.GPUSpec{
				Mode: "time-slicing",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, ng).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-gpu-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-gpu-1"}, updatedNode)
	require.NoError(t, err)

	assert.Equal(t, "time-slicing", updatedNode.Labels[devicePluginLabel])
}

// TestAI_MIGConfigLabelSet verifies that the MIG config label is set on a node
// when the NodeGroup has MIG strategy configured.
func TestAI_MIGConfigLabelSet(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu-2",
			Labels: map[string]string{
				"node.deckhouse.io/group": "worker-gpu-mig",
			},
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu-mig",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			GPU: &deckhousev1.GPUSpec{
				Mode: "mig",
				MIG: &deckhousev1.MIGSpec{
					Strategy: "all-1g.5gb",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, ng).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-gpu-2"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-gpu-2"}, updatedNode)
	require.NoError(t, err)

	assert.Equal(t, "", updatedNode.Labels[gpuEnabledLabel])
	assert.Equal(t, "mig", updatedNode.Labels[devicePluginLabel])
	assert.Equal(t, "all-1g.5gb", updatedNode.Labels[migConfigLabel])
}

// TestAI_MIGConfigDisabledWhenNotMIG verifies that the MIG config label is set to
// "all-disabled" when a node has MIG config but the NodeGroup does not specify MIG.
func TestAI_MIGConfigDisabledWhenNotMIG(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu-1",
			Labels: map[string]string{
				"node.deckhouse.io/group":             "worker-gpu",
				"node.deckhouse.io/gpu":               "",
				"node.deckhouse.io/device-gpu.config": "time-slicing",
				"nvidia.com/mig.config":               "all-1g.5gb",
			},
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			GPU: &deckhousev1.GPUSpec{
				Mode: "time-slicing",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, ng).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-gpu-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-gpu-1"}, updatedNode)
	require.NoError(t, err)

	assert.Equal(t, migDisabled, updatedNode.Labels[migConfigLabel])
}

// TestAI_NodeWithoutNodeGroupLabel verifies that a node without the node group label
// is skipped without error.
func TestAI_NodeWithoutNodeGroupLabel(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-0",
			Labels: map[string]string{},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-0"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-0"}, updatedNode)
	require.NoError(t, err)

	_, hasGPU := updatedNode.Labels[gpuEnabledLabel]
	assert.False(t, hasGPU)
}

// TestAI_NodeGroupWithoutGPU verifies that a node in a NodeGroup without GPU config
// is skipped without error and no GPU labels are added.
func TestAI_NodeGroupWithoutGPU(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-0",
			Labels: map[string]string{
				"node.deckhouse.io/group": "worker",
			},
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, ng).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-0"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-0"}, updatedNode)
	require.NoError(t, err)

	_, hasGPU := updatedNode.Labels[gpuEnabledLabel]
	assert.False(t, hasGPU)
}

// TestAI_NodeNotFound verifies that reconciling a non-existent node returns no error.
func TestAI_NodeNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_GPULabelsAlreadyUpToDate verifies that no patch error occurs when labels
// are already correct.
func TestAI_GPULabelsAlreadyUpToDate(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu-0",
			Labels: map[string]string{
				"node.deckhouse.io/group":             "worker-gpu",
				"node.deckhouse.io/gpu":               "",
				"node.deckhouse.io/device-gpu.config": "time-slicing",
			},
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-gpu",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			GPU: &deckhousev1.GPUSpec{
				Mode: "time-slicing",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, ng).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-gpu-0"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-gpu-0"}, updatedNode)
	require.NoError(t, err)

	assert.Equal(t, "", updatedNode.Labels[gpuEnabledLabel])
	assert.Equal(t, "time-slicing", updatedNode.Labels[devicePluginLabel])
}

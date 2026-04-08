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

package nodeuser

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	return s
}

func nodeWithGroup(name, group string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{nodeGroupLabel: group},
		},
	}
}

func TestAI_NodeUser_NoErrors_Noop(t *testing.T) {
	nu := &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: "user1"},
		Spec: deckhousev1.NodeUserSpec{
			UID:      1000,
			IsSudoer: false,
		},
		Status: deckhousev1.NodeUserStatus{},
	}

	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(nu).
		WithStatusSubresource(nu).
		Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "user1"},
	})
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, result.RequeueAfter)
}

func TestAI_NodeUser_ClearStaleErrors(t *testing.T) {
	nu := &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: "user1"},
		Spec: deckhousev1.NodeUserSpec{
			UID:      1000,
			IsSudoer: false,
		},
		Status: deckhousev1.NodeUserStatus{
			Errors: map[string]string{
				"existing-node": "some error",
				"removed-node":  "another error",
			},
		},
	}

	existingNode := nodeWithGroup("existing-node", "worker")

	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(nu, existingNode).
		WithStatusSubresource(nu).
		Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "user1"},
	})
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, result.RequeueAfter)

	got := &deckhousev1.NodeUser{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "user1"}, got))

	assert.Contains(t, got.Status.Errors, "existing-node", "errors for existing nodes should remain")
	assert.NotContains(t, got.Status.Errors, "removed-node", "errors for removed nodes should be cleared")
}

func TestAI_NodeUser_AllErrorsStale(t *testing.T) {
	nu := &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: "user1"},
		Spec: deckhousev1.NodeUserSpec{
			UID:      1000,
			IsSudoer: false,
		},
		Status: deckhousev1.NodeUserStatus{
			Errors: map[string]string{
				"removed-node-1": "error1",
				"removed-node-2": "error2",
			},
		},
	}

	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(nu).
		WithStatusSubresource(nu).
		Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "user1"},
	})
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, result.RequeueAfter)

	got := &deckhousev1.NodeUser{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "user1"}, got))

	assert.Empty(t, got.Status.Errors, "all stale errors should be cleared")
}

func TestAI_NodeUser_AllErrorsValid(t *testing.T) {
	nu := &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: "user1"},
		Spec: deckhousev1.NodeUserSpec{
			UID:      1000,
			IsSudoer: false,
		},
		Status: deckhousev1.NodeUserStatus{
			Errors: map[string]string{
				"node-1": "error1",
				"node-2": "error2",
			},
		},
	}

	node1 := nodeWithGroup("node-1", "worker")
	node2 := nodeWithGroup("node-2", "worker")

	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(nu, node1, node2).
		WithStatusSubresource(nu).
		Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "user1"},
	})
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, result.RequeueAfter)

	got := &deckhousev1.NodeUser{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "user1"}, got))

	assert.Len(t, got.Status.Errors, 2, "no errors should be cleared when all nodes exist")
}

func TestAI_NodeUser_NotFound(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestAI_NodeUser_NodeWithoutGroupLabel_NotCounted(t *testing.T) {
	nu := &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: "user1"},
		Spec: deckhousev1.NodeUserSpec{
			UID:      1000,
			IsSudoer: false,
		},
		Status: deckhousev1.NodeUserStatus{
			Errors: map[string]string{
				"unlabeled-node": "some error",
			},
		},
	}

	// Node exists but without the required label
	unlabeledNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unlabeled-node",
		},
	}

	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(nu, unlabeledNode).
		WithStatusSubresource(nu).
		Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "user1"},
	})
	require.NoError(t, err)

	got := &deckhousev1.NodeUser{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "user1"}, got))

	// Node without the group label should not be counted as existing,
	// so its error entry should be cleared
	assert.Empty(t, got.Status.Errors, "error for node without group label should be cleared")
}

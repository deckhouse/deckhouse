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

package bashible

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
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func bashiblePod(name, hostIP string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   bashibleNamespace,
			Labels:      map[string]string{"app": bashibleAppLabel},
			Annotations: annotations,
		},
		Status: corev1.PodStatus{
			HostIP: hostIP,
		},
	}
}

func TestAI_BashiblePod_SetInitialHostIP(t *testing.T) {
	pod := bashiblePod("bashible-apiserver-test", "1.2.3.4", nil)

	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "bashible-apiserver-test",
			Namespace: bashibleNamespace,
		},
	})
	require.NoError(t, err)

	got := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      "bashible-apiserver-test",
		Namespace: bashibleNamespace,
	}, got))

	assert.Equal(t, "1.2.3.4", got.Annotations[initialHostIPAnnotation])
}

func TestAI_BashiblePod_HostIPChanged_DeletePod(t *testing.T) {
	pod := bashiblePod("bashible-apiserver-test", "4.5.6.7", map[string]string{
		initialHostIPAnnotation: "1.2.3.4",
	})

	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "bashible-apiserver-test",
			Namespace: bashibleNamespace,
		},
	})
	require.NoError(t, err)

	got := &corev1.Pod{}
	err = c.Get(context.Background(), types.NamespacedName{
		Name:      "bashible-apiserver-test",
		Namespace: bashibleNamespace,
	}, got)
	assert.True(t, err != nil, "pod should have been deleted")
}

func TestAI_BashiblePod_SameHostIP_NoDeletion(t *testing.T) {
	pod := bashiblePod("bashible-apiserver-test", "1.2.3.4", map[string]string{
		initialHostIPAnnotation: "1.2.3.4",
	})

	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "bashible-apiserver-test",
			Namespace: bashibleNamespace,
		},
	})
	require.NoError(t, err)

	got := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      "bashible-apiserver-test",
		Namespace: bashibleNamespace,
	}, got))

	assert.Equal(t, "1.2.3.4", got.Annotations[initialHostIPAnnotation])
}

func TestAI_BashiblePod_EmptyHostIP_NoAction(t *testing.T) {
	pod := bashiblePod("bashible-apiserver-test", "", map[string]string{
		initialHostIPAnnotation: "1.2.3.4",
	})

	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "bashible-apiserver-test",
			Namespace: bashibleNamespace,
		},
	})
	require.NoError(t, err)

	got := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      "bashible-apiserver-test",
		Namespace: bashibleNamespace,
	}, got))

	assert.Equal(t, "1.2.3.4", got.Annotations[initialHostIPAnnotation],
		"pod with empty hostIP should not be modified")
}

func TestAI_BashiblePod_WrongNamespace_NoAction(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bashible-apiserver-test",
			Namespace: "kube-system",
			Labels:    map[string]string{"app": bashibleAppLabel},
		},
		Status: corev1.PodStatus{
			HostIP: "1.2.3.4",
		},
	}

	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "bashible-apiserver-test",
			Namespace: "kube-system",
		},
	})
	require.NoError(t, err)

	got := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      "bashible-apiserver-test",
		Namespace: "kube-system",
	}, got))

	assert.Empty(t, got.Annotations, "pod in wrong namespace should not get annotation")
}

func TestAI_BashiblePod_WrongLabel_NoAction(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-other-pod",
			Namespace: bashibleNamespace,
			Labels:    map[string]string{"app": "other"},
		},
		Status: corev1.PodStatus{
			HostIP: "1.2.3.4",
		},
	}

	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "some-other-pod",
			Namespace: bashibleNamespace,
		},
	})
	require.NoError(t, err)

	got := &corev1.Pod{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      "some-other-pod",
		Namespace: bashibleNamespace,
	}, got))

	assert.Empty(t, got.Annotations, "pod with wrong label should not get annotation")
}

func TestAI_BashiblePod_NotFound(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: bashibleNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

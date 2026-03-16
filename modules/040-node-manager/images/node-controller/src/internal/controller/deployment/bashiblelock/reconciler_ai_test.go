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

package bashiblelock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
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
	_ = appsv1.AddToScheme(s)
	return s
}

// TestAI_ImageMismatchLocksSecret verifies that when the deployment image digest
// does not match the expected digest annotation, the secret is locked.
func TestAI_ImageMismatchLocksSecret(t *testing.T) {
	scheme := newTestScheme()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleName,
			Namespace: bashibleNamespace,
			Annotations: map[string]string{
				"node.deckhouse.io/bashible-apiserver-image-digest": "sha256:expected123",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  bashibleName,
							Image: "registry.example.com/bashible-apiserver@sha256:current456",
						},
					},
				},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bashibleNamespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dep, secret).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: bashibleName, Namespace: bashibleNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the secret was locked.
	updatedSecret := &corev1.Secret{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      secretName,
		Namespace: bashibleNamespace,
	}, updatedSecret)
	require.NoError(t, err)
	assert.Equal(t, "true", updatedSecret.Annotations[lockedAnnotation])
}

// TestAI_ImageMatchUnlocksSecret verifies that when the deployment image digest
// matches the expected annotation and rollout is complete, the secret is unlocked.
func TestAI_ImageMatchUnlocksSecret(t *testing.T) {
	scheme := newTestScheme()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleName,
			Namespace: bashibleNamespace,
			Annotations: map[string]string{
				"node.deckhouse.io/bashible-apiserver-image-digest": "sha256:abc123",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  bashibleName,
							Image: "registry.example.com/bashible-apiserver@sha256:abc123",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:        1,
			UpdatedReplicas: 1,
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bashibleNamespace,
			Annotations: map[string]string{
				lockedAnnotation: "true",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dep, secret).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: bashibleName, Namespace: bashibleNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the lock annotation was removed.
	updatedSecret := &corev1.Secret{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      secretName,
		Namespace: bashibleNamespace,
	}, updatedSecret)
	require.NoError(t, err)
	_, hasLock := updatedSecret.Annotations[lockedAnnotation]
	assert.False(t, hasLock, "lock annotation should be removed")
}

// TestAI_RolloutNotComplete verifies that when the image matches but the rollout
// is not complete (replicas != updatedReplicas), the secret is not unlocked.
func TestAI_RolloutNotComplete(t *testing.T) {
	scheme := newTestScheme()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleName,
			Namespace: bashibleNamespace,
			Annotations: map[string]string{
				"node.deckhouse.io/bashible-apiserver-image-digest": "sha256:abc123",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  bashibleName,
							Image: "registry.example.com/bashible-apiserver@sha256:abc123",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:        2,
			UpdatedReplicas: 1,
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bashibleNamespace,
			Annotations: map[string]string{
				lockedAnnotation: "true",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dep, secret).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: bashibleName, Namespace: bashibleNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the lock annotation is still present (not yet unlocked).
	updatedSecret := &corev1.Secret{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      secretName,
		Namespace: bashibleNamespace,
	}, updatedSecret)
	require.NoError(t, err)
	assert.Equal(t, "true", updatedSecret.Annotations[lockedAnnotation])
}

// TestAI_DeploymentNotFound verifies that reconciling a non-existent deployment
// returns no error.
func TestAI_DeploymentNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: bashibleName, Namespace: bashibleNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_WrongDeploymentName verifies that a deployment with a different name
// is skipped.
func TestAI_WrongDeploymentName(t *testing.T) {
	scheme := newTestScheme()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-deployment",
			Namespace: bashibleNamespace,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dep).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "other-deployment", Namespace: bashibleNamespace},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

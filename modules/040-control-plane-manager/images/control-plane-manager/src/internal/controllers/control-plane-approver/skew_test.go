/*
Copyright 2026 Flant JSC

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

package controlplaneapprover

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func TestReconciler_VersionSkew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// run reconciles masters n1, n2 with a pending kcm operation; desired "-" means no config Secret.
	run := func(t *testing.T, desired string, apiServers map[string]string) (bool, reconcile.Result) {
		t.Helper()
		kcm := newOperation("kcm", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		objs := []client.Object{
			testControlPlaneNode("n1"),
			testControlPlaneNode("n2"),
			&kcm,
		}
		if desired != "-" {
			objs = append(objs, testConfigSecret(desired))
		}
		for node, version := range apiServers {
			objs = append(objs, testAPIServerPod(node, version))
		}
		cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(objs...).Build()

		res, err := newReconciler(cl, cl).Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "kcm", Namespace: constants.KubeSystemNamespace}, &updated))
		return updated.Spec.Approved, res
	}

	t.Run("approves kcm when every kube-apiserver runs its version", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, "1.36", map[string]string{"n1": "1.36", "n2": "1.36"})
		require.True(t, approved)
		require.Zero(t, res.RequeueAfter)
	})

	t.Run("holds back kcm while a kube-apiserver runs an older version", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, "1.36", map[string]string{"n1": "1.36", "n2": "1.35"})
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back kcm while a master has no kube-apiserver pod", func(t *testing.T) {
		t.Parallel()
		approved, _ := run(t, "1.36", map[string]string{"n1": "1.36"})
		require.False(t, approved)
	})

	t.Run("holds back kcm while a kube-apiserver on a not ready master runs an older version", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, "1.36", map[string]string{"n1": "1.36", "n2": "1.36", "n3": "1.35"})
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back kcm when its template has no kubernetes-version", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, "", map[string]string{"n1": "1.36", "n2": "1.36"})
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back kcm without the config Secret", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, "-", map[string]string{"n1": "1.36", "n2": "1.36"})
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("approves etcd without the config Secret", func(t *testing.T) {
		t.Parallel()
		etcd := newOperation("etcd", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		kcm := newOperation("kcm", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		cl := fake.NewClientBuilder().WithScheme(testScheme).
			WithObjects(testControlPlaneNode("n1"), testControlPlaneNode("n2"), &etcd, &kcm).Build()
		r := newReconciler(cl, cl)
		r.etcdLeader = noEtcdLeader

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "etcd", Namespace: constants.KubeSystemNamespace}, &updated))
		require.True(t, updated.Spec.Approved)
	})
}

func TestManifestKubernetesVersion(t *testing.T) {
	t.Parallel()
	require.Equal(t, "1.36", manifestKubernetesVersion([]byte(testManifest("1.36"))))
	require.Empty(t, manifestKubernetesVersion(nil))
	require.Empty(t, manifestKubernetesVersion([]byte("{")))
}

func testManifest(version string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: %q
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - --bind-address=$MY_IP
`, version)
}

func testConfigSecret(version string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: constants.ControlPlaneManagerConfigSecretName, Namespace: constants.KubeSystemNamespace},
		Data: map[string][]byte{
			controlplanev1alpha1.OperationComponentKubeControllerManager.SecretKey(): []byte(testManifest(version)),
		},
	}
}

func testAPIServerPod(node, version string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "kube-apiserver-" + node,
			Namespace:   constants.KubeSystemNamespace,
			Labels:      map[string]string{constants.StaticPodComponentLabelKey: "kube-apiserver"},
			Annotations: map[string]string{constants.KubernetesVersionAnnotationKey: version},
		},
		Spec: corev1.PodSpec{NodeName: node},
	}
}

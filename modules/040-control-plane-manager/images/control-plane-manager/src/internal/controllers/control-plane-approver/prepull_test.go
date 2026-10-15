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

const (
	testTemplateImage = "registry.example.com/deckhouse/ce@sha256:new"
	// Same digest as testTemplateImage, different repository.
	testHeldImage = "registry.example.com/deckhouse/ce/modules@sha256:new"
	testOldImage  = "registry.example.com/deckhouse/ce/modules@sha256:old"
)

var (
	running = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	pulling = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}
)

func TestReconciler_UnpulledImages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// run reconciles op on master n1 and reports whether it got approved. template "-" means no config Secret.
	run := func(t *testing.T, op controlplanev1alpha1.ControlPlaneOperation, template string, extra ...client.Object) (bool, reconcile.Result) {
		t.Helper()
		objs := []client.Object{testControlPlaneNode("n1"), &op}
		if template != "-" {
			objs = append(objs, testImageConfigSecret(op.Spec.Component, template))
		}
		objs = append(objs, extra...)
		cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(objs...).Build()
		r := newReconciler(cl, cl)
		r.etcdLeader = noEtcdLeader

		res, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(&op), &updated))
		return updated.Spec.Approved, res
	}
	apiServer := testSyncOperation("api", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer)

	t.Run("approves when a holder on its node runs the template image", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, apiServer, testImageManifest(testTemplateImage), testManagerPod("cpm-n1", "n1", testHeldImage, running))
		require.True(t, approved)
		require.Zero(t, res.RequeueAfter)
	})

	t.Run("holds back while the holder on its node runs a previous image", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, apiServer, testImageManifest(testTemplateImage), testManagerPod("cpm-n1", "n1", testOldImage, running))
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("approves when the static pod on its node runs the template image", func(t *testing.T) {
		t.Parallel()
		// Certificate renewal: the template keeps the image the component runs.
		approved, res := run(t, apiServer, testImageManifest(testTemplateImage),
			testManagerPod("cpm-n1", "n1", testOldImage, running),
			testStaticPod("n1", controlplanev1alpha1.OperationComponentKubeAPIServer, testTemplateImage, running))
		require.True(t, approved)
		require.Zero(t, res.RequeueAfter)
	})

	t.Run("holds back while the static pod on its node runs a previous image", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, apiServer, testImageManifest(testTemplateImage),
			testManagerPod("cpm-n1", "n1", testOldImage, running),
			testStaticPod("n1", controlplanev1alpha1.OperationComponentKubeAPIServer, testOldImage, running))
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back while the holder still pulls the template image", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, apiServer, testImageManifest(testTemplateImage), testManagerPod("cpm-n1", "n1", testHeldImage, pulling))
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back while the image is held on another node only", func(t *testing.T) {
		t.Parallel()
		approved, _ := run(t, apiServer, testImageManifest(testTemplateImage), testManagerPod("cpm-n2", "n2", testHeldImage, running))
		require.False(t, approved)
	})

	t.Run("holds back while only a terminating Pod holds the image", func(t *testing.T) {
		t.Parallel()
		terminating := testManagerPod("cpm-n1", "n1", testHeldImage, running)
		now := metav1.Now()
		terminating.DeletionTimestamp = &now
		terminating.Finalizers = []string{"test"}
		approved, _ := run(t, apiServer, testImageManifest(testTemplateImage), terminating)
		require.False(t, approved)
	})

	t.Run("holds back when its template has no image", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, apiServer, testImageManifest(""), testManagerPod("cpm-n1", "n1", testHeldImage, running))
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back without the config Secret", func(t *testing.T) {
		t.Parallel()
		approved, res := run(t, apiServer, "-", testManagerPod("cpm-n1", "n1", testHeldImage, running))
		require.False(t, approved)
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("holds back an etcd join until its image is held", func(t *testing.T) {
		t.Parallel()
		join := newOperation("join", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		join.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepJoinEtcdCluster, controlplanev1alpha1.StepWaitPodReady}
		approved, _ := run(t, join, testImageManifest(testTemplateImage), testManagerPod("cpm-n1", "n1", testOldImage, running))
		require.False(t, approved)
	})

	t.Run("approves a checksum-only manifest sync without holders", func(t *testing.T) {
		t.Parallel()
		pkiOnly := testSyncOperation("pki", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer)
		pkiOnly.Spec.DesiredConfigChecksum = ""
		pkiOnly.Spec.DesiredPKIChecksum = "pki"
		approved, _ := run(t, pkiOnly, testImageManifest(testTemplateImage))
		require.True(t, approved)
	})

	t.Run("an operation waiting for its images keeps its slot", func(t *testing.T) {
		t.Parallel()
		// One kube-apiserver at a time: n1 goes first and waits for its image, so n2 waits too.
		api1 := testSyncOperation("api-1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer)
		api2 := testSyncOperation("api-2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer)
		cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
			testControlPlaneNode("n1"),
			testControlPlaneNode("n2"),
			&api1,
			&api2,
			testImageConfigSecret(controlplanev1alpha1.OperationComponentKubeAPIServer, testImageManifest(testTemplateImage)),
			testManagerPod("cpm-n1", "n1", testOldImage, running),
			testManagerPod("cpm-n2", "n2", testHeldImage, running),
		).Build()

		res, err := newReconciler(cl, cl).Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
		require.Equal(t, recheckInterval, res.RequeueAfter)

		for _, name := range []string{"api-1", "api-2"} {
			var updated controlplanev1alpha1.ControlPlaneOperation
			require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: name, Namespace: constants.KubeSystemNamespace}, &updated))
			require.False(t, updated.Spec.Approved, name)
		}
	})
}

func TestManifestImages(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{testTemplateImage}, manifestImages([]byte(testImageManifest(testTemplateImage))))
	require.Empty(t, manifestImages([]byte(testImageManifest(""))))
	require.Empty(t, manifestImages(nil))
	require.Empty(t, manifestImages([]byte("{")))
}

func TestImageDigest(t *testing.T) {
	t.Parallel()
	require.Equal(t, "sha256:new", imageDigest(testTemplateImage))
	require.Equal(t, imageDigest(testTemplateImage), imageDigest(testHeldImage))
	require.Equal(t, "registry.example.com/pause:3.10", imageDigest("registry.example.com/pause:3.10"))
}

func testSyncOperation(name, node string, component controlplanev1alpha1.OperationComponent) controlplanev1alpha1.ControlPlaneOperation {
	op := newOperation(name, node, component, false)
	op.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests, controlplanev1alpha1.StepWaitPodReady}
	op.Spec.DesiredConfigChecksum = "config"
	return op
}

func testImageManifest(image string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    image: %q
    command:
    - --advertise-address=$MY_IP
`, image)
}

func testImageConfigSecret(component controlplanev1alpha1.OperationComponent, manifest string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: constants.ControlPlaneManagerConfigSecretName, Namespace: constants.KubeSystemNamespace},
		Data:       map[string][]byte{component.SecretKey(): []byte(manifest)},
	}
}

func testManagerPod(name, node, holderImage string, holderState corev1.ContainerState) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.KubeSystemNamespace,
			Labels:    map[string]string{constants.StaticPodComponentLabelKey: constants.ControlPlaneManagerComponentLabelValue},
		},
		Spec: corev1.PodSpec{
			NodeName: node,
			Containers: []corev1.Container{
				{Name: "control-plane-manager", Image: "registry.example.com/deckhouse/ce/modules@sha256:cpm"},
				{Name: "image-holder", Image: holderImage},
			},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "control-plane-manager", State: running},
				{Name: "image-holder", State: holderState},
			},
		},
	}
}

// testStaticPod is the mirror pod of component's static pod on node, with its container in state.
func testStaticPod(node string, component controlplanev1alpha1.OperationComponent, image string, state corev1.ContainerState) *corev1.Pod {
	name := component.PodComponentName()
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.MirrorPodName(node),
			Namespace: constants.KubeSystemNamespace,
			Labels:    map[string]string{constants.StaticPodComponentLabelKey: name},
		},
		Spec:   corev1.PodSpec{NodeName: node, Containers: []corev1.Container{{Name: name, Image: image}}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: name, State: state}}},
	}
}

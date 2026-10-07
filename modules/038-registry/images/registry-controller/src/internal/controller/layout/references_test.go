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

package layout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

const (
	upstreamImage  = "registry.deckhouse.io/deckhouse/ee/modules/prometheus@sha256:412337f27c62daffbcd5c12350e5d261fbfe4434adca8b31a6dda59002226502"
	inClusterImage = "registry.d8-system.svc:5001/system/deckhouse/modules/prometheus@sha256:412337f27c62daffbcd5c12350e5d261fbfe4434adca8b31a6dda59002226502"
)

func TestNamesUpstream(t *testing.T) {
	names := namesUpstream(upstream("registry.deckhouse.io"))

	for image, want := range map[string]bool{
		upstreamImage: true,
		// The platform's own image has nothing between the path and the digest.
		"registry.deckhouse.io/deckhouse/ee@sha256:53a4ec47d23bd30b069883f0a099bb9f1e34ed68702b49deed72948dbedfe2a4": true,
		"registry.deckhouse.io/deckhouse/ee:v1.70.0":                                                                 true,
		inClusterImage: false,
		// Another repository on the same host is not the upstream, and neither is a path that
		// merely starts with the same letters.
		"registry.deckhouse.io/deckhouse/ce/modules/prometheus:v1":  false,
		"registry.deckhouse.io/deckhouse/eee/modules/prometheus:v1": false,
		"registry.deckhouse.io.example.com/deckhouse/ee/x:v1":       false,
		// The same registry written another way: hostnames ignore case, and 443 is implied.
		"REGISTRY.Deckhouse.io/deckhouse/ee/modules/prometheus:v1":     true,
		"registry.deckhouse.io:443/deckhouse/ee/modules/prometheus:v1": true,
	} {
		assert.Equalf(t, want, names(image), "%s", image)
	}

	// An upstream with no path is a bare host, and what follows a colon after it is a port: that
	// is another registry.
	bare := namesUpstream(&registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{Host: "registry.example.com"}})
	assert.True(t, bare("registry.example.com/deckhouse/ee:v1"))
	assert.False(t, bare("registry.example.com:5000/deckhouse/ee:v1"))
}

func TestPodWouldPull(t *testing.T) {
	names := namesUpstream(upstream("registry.deckhouse.io"))
	running := []corev1.ContainerStatus{{
		Name: "promxy", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}

	pod := func(image string, mutate func(*corev1.Pod)) *corev1.Pod {
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "d8-monitoring", Name: "aggregating-proxy"},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "promxy", Image: image, ImagePullPolicy: corev1.PullIfNotPresent,
			}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
		if mutate != nil {
			mutate(p)
		}
		return p
	}

	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "not started yet: its image is still to be pulled",
			pod:  pod(upstreamImage, nil),
			want: true,
		},
		{
			name: "started, no pull secrets, pulled if not present: a restart finds the image on its node",
			pod:  pod(upstreamImage, func(p *corev1.Pod) { p.Status.ContainerStatuses = running }),
			want: false,
		},
		{
			// The measured case: the kubelet verifies the credentials an image was pulled with by
			// pulling again, and in air-gap there is nothing to pull from.
			name: "started, with pull secrets: the next start pulls again",
			pod: pod(upstreamImage, func(p *corev1.Pod) {
				p.Status.ContainerStatuses = running
				p.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "deckhouse-registry"}}
			}),
			want: true,
		},
		{
			name: "started, pulled always",
			pod: pod(upstreamImage, func(p *corev1.Pod) {
				p.Status.ContainerStatuses = running
				p.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
			}),
			want: true,
		},
		{
			name: "on its way out",
			pod: pod(upstreamImage, func(p *corev1.Pod) {
				p.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				p.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "deckhouse-registry"}}
			}),
			want: false,
		},
		{
			name: "finished",
			pod:  pod(upstreamImage, func(p *corev1.Pod) { p.Status.Phase = corev1.PodSucceeded }),
			want: false,
		},
		{
			// The store's own replica would be let off by the rule above; it is counted on its own.
			// See TestStoreReplicaCountsWhileItNamesTheUpstream.
			name: "a started store replica without pull secrets, by the general rule",
			pod: pod(upstreamImage, func(p *corev1.Pod) {
				p.Namespace, p.Labels = Namespace, map[string]string{"app": storeAppLabel}
				p.Status.ContainerStatuses = running
			}),
			want: false,
		},
		{
			// Crash-looping between restarts reads Waiting, and the image is on the node all the same.
			name: "crash-looping without pull secrets: the image is already on the node",
			pod: pod(upstreamImage, func(p *corev1.Pod) {
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name: "promxy", RestartCount: 4,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				}}
			}),
			want: false,
		},
		{
			// Someone debugging a pod is not the platform's workload.
			name: "an ephemeral debug container on the upstream",
			pod: pod(inClusterImage, func(p *corev1.Pod) {
				p.Status.ContainerStatuses = running
				p.Spec.EphemeralContainers = []corev1.EphemeralContainer{{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug", Image: upstreamImage},
				}}
			}),
			want: false,
		},
		{
			name: "already on the in-cluster address",
			pod: pod(inClusterImage, func(p *corev1.Pod) {
				p.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "deckhouse-registry"}}
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, podWouldPull(tt.pod, names))
		})
	}
}

// The scenario measured on the `scenarios` unit on 2026-10-05: the leader is full and air-gap is
// asked for, but a superseded monitoring pod still names the upstream and will pull again on its
// next start. The upstream is held, the status says why, and the controller keeps asking — then
// lets go once the pod has moved.
func TestReconcileAirGapWaitsForThePlatformToLeaveTheUpstream(t *testing.T) {
	cfg := registryConfig(registryv1alpha1.RegistryConfigSpec{
		Mode:    registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: upstream("registry.deckhouse.io")},
		Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: source()},
	})
	stale := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "d8-monitoring", Name: "aggregating-proxy-98d4bcfb6-7cpc8"},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{{Name: "promxy", Image: upstreamImage}},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "deckhouse-registry"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "promxy", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	// A replica of the store created before the in-cluster address was published. It has no pull
	// secrets and has started, which would let any other pod off — but a replica replaced after
	// the upstream is gone has to pull itself through the store, so it counts until it has moved.
	replica := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace, Name: "registry-storage-0", Labels: map[string]string{"app": storeAppLabel},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "distribution", Image: upstreamImage}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "distribution", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	// Its StatefulSet is already rendered from the in-cluster address; the replica is what lags.
	store := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: "registry-storage"},
		Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "distribution", Image: inClusterImage}},
		}}},
	}

	namespaces := []client.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-monitoring"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: Namespace}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	}
	r, c := newReconciler(t, append(namespaces,
		cfg, accessSecret(), node("master-0"), storageLease("master-0"), stale, replica, store)...)
	clock := time.Date(2026, 10, 5, 12, 23, 35, 0, time.UTC)
	r.now = func() time.Time { return clock }
	ctx := context.Background()
	runReconcile(t, r)

	live := &registryv1alpha1.RegistryConfig{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, live))
	live.Spec.Primary.Upstream = nil
	live.Generation = 2
	require.NoError(t, c.Update(ctx, live))
	runReconcile(t, r)

	storage := getStorage(t, c)
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459},
	}
	require.NoError(t, c.Status().Update(ctx, storage))

	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}})
	require.NoError(t, err)

	storage = getStorage(t, c)
	require.NotNil(t, storage.Spec.Upstream, "a full cache does not answer a reference that names the upstream")
	assert.False(t, storage.Status.SafeToDropUpstream, "and the status may not claim otherwise")
	require.NotNil(t, storage.Status.UpstreamReferences)
	assert.EqualValues(t, 2, *storage.Status.UpstreamReferences, "the superseded pod and the store's own replica")
	assert.Len(t, listNodes(t, c)["master-0"].Spec.Backends, 2, "the nodes keep their fallback")
	assert.Equal(t, referenceScanInterval, result.RequeueAfter,
		"nothing watched here changes when the rollout finishes, so the controller asks again")

	// The rollouts finish: the pod is replaced by one on the in-cluster address.
	require.NoError(t, c.Delete(ctx, stale))
	moved := stale.DeepCopy()
	moved.ResourceVersion = ""
	moved.Name = "aggregating-proxy-7f69ffb5c8-b7x5s"
	moved.Spec.Containers[0].Image = inClusterImage
	require.NoError(t, c.Create(ctx, moved))

	// And the storage update controller replaces the replica onto the new revision.
	require.NoError(t, c.Delete(ctx, replica))
	replaced := replica.DeepCopy()
	replaced.ResourceVersion = ""
	replaced.Spec.Containers[0].Image = inClusterImage
	require.NoError(t, c.Create(ctx, replaced))

	// A count that holds the upstream is reused for a while, so the next pass inside the interval
	// still waits, and the one after it lets go.
	runReconcile(t, r)
	require.NotNil(t, getStorage(t, c).Spec.Upstream, "a recent nonzero count is reused")

	clock = clock.Add(referenceScanInterval)
	runReconcile(t, r)

	storage = getStorage(t, c)
	assert.Nil(t, storage.Spec.Upstream, "nothing names the upstream any more, so it goes")
	assert.True(t, storage.Status.SafeToDropUpstream)
	assert.Nil(t, storage.Status.UpstreamReferences, "with no upstream left there is nothing to count")
	assert.Len(t, listNodes(t, c)["master-0"].Spec.Backends, 1)
}

func TestStoreReplicaCountsWhileItNamesTheUpstream(t *testing.T) {
	names := namesUpstream(upstream("registry.deckhouse.io"))
	replica := func(image string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: Namespace, Name: "registry-storage-1", Labels: map[string]string{"app": storeAppLabel},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "distribution", Image: image}}},
		}
	}

	upstreamReplica := replica(upstreamImage)
	assert.True(t, isStorePod(upstreamReplica) && podNames(upstreamReplica, names),
		"a replica on the upstream's address has to move before the upstream goes")

	moved := replica(inClusterImage)
	assert.False(t, isStorePod(moved) && podNames(moved, names))

	leaving := replica(upstreamImage)
	leaving.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	assert.False(t, podNames(leaving, names), "a replica on its way out is not waited on")

	elsewhere := replica(upstreamImage)
	elsewhere.Namespace = "d8-monitoring"
	assert.False(t, isStorePod(elsewhere), "only the store in the module's namespace is the store")
}

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

package cloudprovider

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// IsRegistration is the one definition of a registration: the watch predicates, the lazy
// InstanceClass source, RegistrationRequests and Load all resolve through it, so every condition it
// checks decides both what is watched and what is loaded.
func TestIsRegistrationSecret(t *testing.T) {
	secret := func(namespace, name string, labels map[string]string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: name, Labels: labels,
		}}
	}
	labelled := map[string]string{RegistrationSecretLabel: ""}

	tests := []struct {
		name string
		obj  *corev1.Secret
		want bool
	}{
		{
			name: "the copy under the bare prefix",
			obj:  secret(RegistrationSecretNamespace, RegistrationSecretNamePrefix, labelled),
			want: true,
		},
		{
			name: "the per-provider copy",
			obj:  secret(RegistrationSecretNamespace, RegistrationSecretNamePrefix+"-yandex", labelled),
			want: true,
		},
		{
			// The label alone is not enough: it is an empty-valued label anyone can copy, and a
			// Secret outside the prefix is not something a provider module publishes.
			name: "labelled, but named outside the prefix",
			obj:  secret(RegistrationSecretNamespace, "some-other-secret", labelled),
		},
		{
			name: "named with the prefix, but not labelled",
			obj:  secret(RegistrationSecretNamespace, RegistrationSecretNamePrefix+"-aws", nil),
		},
		{
			// Registrations live in one namespace; the same name elsewhere is somebody else's.
			name: "right name and label, wrong namespace",
			obj:  secret("default", RegistrationSecretNamePrefix, labelled),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsRegistrationSecret(tc.obj))
		})
	}
}

// A controller whose queue mixes registration keys with keys of other kinds routes on this, so a
// key that is not a registration must not pass — it names no provider to act on.
func TestIsRegistrationSecretKey(t *testing.T) {
	assert.True(t, IsRegistrationSecretKey(types.NamespacedName{
		Namespace: RegistrationSecretNamespace, Name: RegistrationSecretNamePrefix + "-yandex",
	}))
	assert.False(t, IsRegistrationSecretKey(types.NamespacedName{Name: "worker"}), "a NodeGroup key")
	assert.False(t, IsRegistrationSecretKey(types.NamespacedName{
		Namespace: "default", Name: RegistrationSecretNamePrefix,
	}), "the right name in the wrong namespace")
}

// NodeGroupHandler is what keeps a registration event affordable: one status reconcile lists every
// Machine and MachineDeployment in the cluster, so enqueueing the NodeGroups of other providers
// costs cluster-size work per group for nothing.
func TestNodeGroupHandler(t *testing.T) {
	awsData := map[string][]byte{
		"type":              []byte("aws"),
		"instanceClassKind": []byte("AWSInstanceClass"),
	}
	yandexData := map[string][]byte{
		"type":              []byte("yandex"),
		"instanceClassKind": []byte("YandexInstanceClass"),
	}
	aws := registrationSecret(RegistrationSecretNamePrefix+"-aws", awsData)
	yandex := registrationSecret(RegistrationSecretNamePrefix+"-yandex", yandexData)

	// The cluster provider is yandex, so the CloudPermanent group resolves to it and nothing else.
	newHandler := func(t *testing.T) (handler.EventHandler, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		t.Helper()
		c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(
			aws, yandex, cloudCluster("Yandex"),
			cloudEphemeral("worker-aws", "AWSInstanceClass"),
			cloudEphemeral("worker-yandex", "YandexInstanceClass"),
			// Names a kind nobody registered. The kind does not route a NodeGroup while the
			// cluster runs one cloud, so this group hangs off the cluster provider like the rest.
			cloudEphemeral("worker-unknown", "VsphereInstanceClass"),
			nodeGroupOfType("master", v1.NodeTypeCloudPermanent),
			nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
			nodeGroupOfType("static", v1.NodeTypeStatic),
		).Build()
		queue := workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		t.Cleanup(queue.ShutDown)
		return NodeGroupHandler(c), queue
	}

	drain := func(t *testing.T, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) []string {
		t.Helper()
		names := make([]string, 0, queue.Len())
		for queue.Len() > 0 {
			req, _ := queue.Get()
			queue.Done(req)
			names = append(names, req.Name)
		}
		sort.Strings(names)
		return names
	}

	// A rotated credential or an edited zone list — what registration updates overwhelmingly are —
	// changes what the NodeGroups of the cluster's own cloud render. Static is not among them: it
	// runs outside every cloud.
	t.Run("the cluster provider takes every non-Static group with it", func(t *testing.T) {
		h, queue := newHandler(t)
		rotated := yandex.DeepCopy()
		rotated.Data["zones"] = []byte(`["ru-central1-a"]`)

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: yandex, ObjectNew: rotated}, queue)

		assert.Equal(t,
			[]string{"cloudstatic", "master", "worker-aws", "worker-unknown", "worker-yandex"},
			drain(t, queue))
	})

	// A registration is published by every enabled cloud-provider module, but only the one named
	// by the cluster configuration has NodeGroups on it.
	t.Run("a registration that is not the cluster provider reaches nothing", func(t *testing.T) {
		h, queue := newHandler(t)
		rotated := aws.DeepCopy()
		rotated.Data["zones"] = []byte(`["eu-central-1a"]`)

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: aws, ObjectNew: rotated}, queue)

		assert.Empty(t, drain(t, queue))
	})

	// The provider is decoded from the event object, which on a delete is the only place it
	// still exists.
	t.Run("create and delete resolve from the event object", func(t *testing.T) {
		want := []string{"cloudstatic", "master", "worker-aws", "worker-unknown", "worker-yandex"}

		h, queue := newHandler(t)
		h.Create(context.Background(), event.CreateEvent{Object: yandex}, queue)
		assert.Equal(t, want, drain(t, queue))

		h, queue = newHandler(t)
		h.Delete(context.Background(), event.DeleteEvent{Object: yandex}, queue)
		assert.Equal(t, want, drain(t, queue))
	})

	// An informer resync replays the object, and helm rewrites annotations without touching the
	// data — which is why the check is on the raw data and not on the object.
	t.Run("an update that changes no data enqueues nothing", func(t *testing.T) {
		h, queue := newHandler(t)
		// The cluster provider on purpose: a data change on this one does enqueue, so an empty
		// queue here can only come from the equality check.
		relabelled := yandex.DeepCopy()
		relabelled.ResourceVersion = "2"
		relabelled.Annotations = map[string]string{"meta.helm.sh/release-name": "cloud-provider-yandex"}

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: yandex, ObjectNew: relabelled}, queue)

		assert.Empty(t, drain(t, queue))
	})

	t.Run("an object that is not a Secret carries no registration", func(t *testing.T) {
		h, queue := newHandler(t)
		var obj client.Object = &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "not-a-secret"}}

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: obj, ObjectNew: obj}, queue)

		assert.Empty(t, drain(t, queue))
	})
}

// An edit of any data key has to pass the raw comparison, including a key that stops being
// published: the CAPI keys default when absent, and the default is not what the old value was.
func TestNodeGroupHandler_EveryDataEditPasses(t *testing.T) {
	edits := map[string]func(*corev1.Secret){
		"an edited zone list":     func(s *corev1.Secret) { s.Data["zones"] = []byte(`["eu-central-1b"]`) },
		"a new key":               func(s *corev1.Secret) { s.Data["capiClusterName"] = []byte("aws") },
		"a key that is dropped":   func(s *corev1.Secret) { delete(s.Data, "machineClassKind") },
		"an edited nested tree":   func(s *corev1.Secret) { s.Data["aws"] = []byte(`{"keyName":"other"}`) },
		"a value emptied in-tree": func(s *corev1.Secret) { s.Data["aws"] = []byte(`{}`) },
	}

	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			before := registrationSecret(RegistrationSecretNamePrefix+"-aws", map[string][]byte{
				"type":              []byte("aws"),
				"instanceClassKind": []byte("AWSInstanceClass"),
				"machineClassKind":  []byte("AWSMachineClass"),
				"zones":             []byte(`["eu-central-1a"]`),
				"aws":               []byte(`{"keyName":"kn"}`),
			})
			after := before.DeepCopy()
			edit(after)

			c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(
				before, cloudCluster("AWS"),
				cloudEphemeral("worker-aws", "AWSInstanceClass"),
			).Build()
			queue := workqueue.NewTypedRateLimitingQueue(
				workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer queue.ShutDown()

			NodeGroupHandler(c).Update(context.Background(),
				event.UpdateEvent{ObjectOld: before, ObjectNew: after}, queue)

			assert.Equal(t, 1, queue.Len())
		})
	}
}

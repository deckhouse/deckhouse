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
	labelled := map[string]string{SecretLabel: ""}

	tests := []struct {
		name string
		obj  *corev1.Secret
		want bool
	}{
		{
			name: "the copy under the bare prefix",
			obj:  secret(SecretNamespace, SecretNamePrefix, labelled),
			want: true,
		},
		{
			name: "the per-provider copy",
			obj:  secret(SecretNamespace, SecretNamePrefix+"-yandex", labelled),
			want: true,
		},
		{
			// The label alone is not enough: it is an empty-valued label anyone can copy, and a
			// Secret outside the prefix is not something a provider module publishes.
			name: "labelled, but named outside the prefix",
			obj:  secret(SecretNamespace, "some-other-secret", labelled),
		},
		{
			name: "named with the prefix, but not labelled",
			obj:  secret(SecretNamespace, SecretNamePrefix+"-aws", nil),
		},
		{
			// Registrations live in one namespace; the same name elsewhere is somebody else's.
			name: "right name and label, wrong namespace",
			obj:  secret("default", SecretNamePrefix, labelled),
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
		Namespace: SecretNamespace, Name: SecretNamePrefix + "-yandex",
	}))
	assert.False(t, IsRegistrationSecretKey(types.NamespacedName{Name: "worker"}), "a NodeGroup key")
	assert.False(t, IsRegistrationSecretKey(types.NamespacedName{
		Namespace: "default", Name: SecretNamePrefix,
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
	aws := registrationSecret(SecretNamePrefix+"-aws", awsData)
	yandex := registrationSecret(SecretNamePrefix+"-yandex", yandexData)

	// The cluster provider is yandex, so the CloudPermanent group resolves to it and nothing else.
	newHandler := func(t *testing.T) (handler.EventHandler, workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		t.Helper()
		c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(
			aws, yandex, clusterConfigurationSecret("Yandex"),
			cloudEphemeral("worker-aws", "AWSInstanceClass"),
			cloudEphemeral("worker-yandex", "YandexInstanceClass"),
			// Names a kind nobody registered: its status carries the list of kinds that do exist,
			// so it belongs to no provider and still depends on the set.
			cloudEphemeral("worker-unknown", "VsphereInstanceClass"),
			nodeGroupOfType("master", v1.NodeTypeCloudPermanent),
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
	// changes what one provider's NodeGroups render, and nothing else in the cluster.
	t.Run("an update reaches only the NodeGroups of that provider", func(t *testing.T) {
		h, queue := newHandler(t)
		rotated := aws.DeepCopy()
		rotated.Data["zones"] = []byte(`["eu-central-1a"]`)

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: aws, ObjectNew: rotated}, queue)

		assert.Equal(t, []string{"worker-aws"}, drain(t, queue))
	})

	// CloudPermanent names no InstanceClass, so it hangs off the cluster provider — an event on
	// that registration has to reach it, and an event on the other one must not.
	t.Run("the cluster provider takes the CloudPermanent group with it", func(t *testing.T) {
		h, queue := newHandler(t)
		rotated := yandex.DeepCopy()
		rotated.Data["zones"] = []byte(`["ru-central1-a"]`)

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: yandex, ObjectNew: rotated}, queue)

		assert.Equal(t, []string{"master", "worker-yandex"}, drain(t, queue))
	})

	// The provider is decoded from the event object, which on a delete is the only place it
	// still exists.
	t.Run("create and delete resolve from the event object", func(t *testing.T) {
		h, queue := newHandler(t)
		h.Create(context.Background(), event.CreateEvent{Object: aws}, queue)
		assert.Equal(t, []string{"worker-aws"}, drain(t, queue))

		h, queue = newHandler(t)
		h.Delete(context.Background(), event.DeleteEvent{Object: aws}, queue)
		assert.Equal(t, []string{"worker-aws"}, drain(t, queue))
	})

	// The kind is what routes a NodeGroup to a provider: the group that matched the old kind has to
	// be told it no longer resolves, and it is in no set the new object can produce.
	t.Run("a re-kinded provider reaches the groups of both sides", func(t *testing.T) {
		h, queue := newHandler(t)
		rekinded := aws.DeepCopy()
		rekinded.Data["instanceClassKind"] = []byte("VsphereInstanceClass")

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: aws, ObjectNew: rekinded}, queue)

		assert.Equal(t, []string{"worker-aws", "worker-unknown"}, drain(t, queue))
	})

	// An informer resync replays the object, and helm rewrites annotations without touching the
	// data — which is why the check is on the raw data and not on the object.
	t.Run("an update that changes no data enqueues nothing", func(t *testing.T) {
		h, queue := newHandler(t)
		relabelled := aws.DeepCopy()
		relabelled.ResourceVersion = "2"
		relabelled.Annotations = map[string]string{"meta.helm.sh/release-name": "cloud-provider-aws"}

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: aws, ObjectNew: relabelled}, queue)

		assert.Empty(t, drain(t, queue))
	})

	// Static and CloudStatic run in no cloud at all, and a group of another provider is not moved
	// by this one.
	t.Run("a registration nobody runs on enqueues nothing", func(t *testing.T) {
		h, queue := newHandler(t)
		orphan := registrationSecret(SecretNamePrefix+"-gcp", map[string][]byte{
			"type":              []byte("gcp"),
			"instanceClassKind": []byte("GCPInstanceClass"),
		})
		edited := orphan.DeepCopy()
		edited.Data["zones"] = []byte(`["europe-west1-b"]`)

		h.Update(context.Background(), event.UpdateEvent{ObjectOld: orphan, ObjectNew: edited}, queue)

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
			before := registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{
				"type":              []byte("aws"),
				"instanceClassKind": []byte("AWSInstanceClass"),
				"machineClassKind":  []byte("AWSMachineClass"),
				"zones":             []byte(`["eu-central-1a"]`),
				"aws":               []byte(`{"keyName":"kn"}`),
			})
			after := before.DeepCopy()
			edit(after)

			c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(
				before, clusterConfigurationSecret("AWS"),
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

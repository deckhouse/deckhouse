// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orphans

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/metadata"
	fakemetadata "k8s.io/client-go/metadata/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	testNamespace = "tenant"
	testInstance  = "app"
)

var (
	jobs       = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	cronJobs   = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	pods       = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	secrets    = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	namespaces = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	widgets    = schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
)

// declared is the declaration the service is built for: Job and CronJob live in the batch group,
// Pod and Secret in the core one, and all four are declared by a bare "v1".
var declared = []Resource{
	{Kind: "Job", Version: "v1"},
	{Kind: "CronJob", Version: "v1"},
	{Kind: "Pod", Version: "v1"},
	{Kind: "Secret", Version: "v1"},
}

var allVerbs = metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"}

func served() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "bindings", Kind: "Binding", Namespaced: true, Verbs: metav1.Verbs{"create"}},
				{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: allVerbs},
				{Name: "pods/log", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"get"}},
				{Name: "secrets", Kind: "Secret", Namespaced: true, Verbs: allVerbs},
				{Name: "namespaces", Kind: "Namespace", Verbs: allVerbs},
			},
		},
		{
			GroupVersion: "batch/v1",
			APIResources: []metav1.APIResource{
				{Name: "jobs", Kind: "Job", Namespaced: true, Verbs: allVerbs},
				{Name: "cronjobs", Kind: "CronJob", Namespaced: true, Verbs: allVerbs},
			},
		},
	}
}

// fakeClient serves discovery from a memory cache over a fake, so tests can make it stale.
type fakeClient struct {
	discovery *fakediscovery.FakeDiscovery
	cached    discovery.CachedDiscoveryInterface
	metadata  *fakemetadata.FakeMetadataClient
}

func (c *fakeClient) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.cached, nil
}

func (c *fakeClient) Metadata() metadata.Interface {
	return c.metadata
}

func newClient(t *testing.T, objects ...runtime.Object) *fakeClient {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, metav1.AddMetaToScheme(scheme))

	disc := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: served()}}

	return &fakeClient{
		discovery: disc,
		cached:    memory.NewMemCacheClient(disc),
		metadata:  fakemetadata.NewSimpleMetadataClient(scheme, objects...),
	}
}

func newObject(apiVersion, kind, namespace, name, instance string, annotations map[string]string) *metav1.PartialObjectMetadata {
	obj := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{APIVersion: apiVersion, Kind: kind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			UID:         types.UID(namespace + "/" + name),
			Annotations: annotations,
		},
	}

	if instance != "" {
		obj.Labels = map[string]string{v1alpha1.PackageLabelInstance: instance}
	}

	return obj
}

// left returns the names of the objects of the resource left in the namespace.
func left(t *testing.T, client *fakeClient, gvr schema.GroupVersionResource, namespace string) []string {
	t.Helper()

	list, err := client.metadata.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.GetName())
	}

	return names
}

func deletes(client *fakeClient) []clienttesting.DeleteAction {
	var res []clienttesting.DeleteAction
	for _, action := range client.metadata.Actions() {
		if del, ok := action.(clienttesting.DeleteAction); ok {
			res = append(res, del)
		}
	}

	return res
}

func TestDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the instance's objects of the declared kinds in its namespace", func(t *testing.T) {
		client := newClient(t,
			newObject("batch/v1", "Job", testNamespace, "d8a-app-migrate", testInstance, nil),
			newObject("batch/v1", "CronJob", testNamespace, "d8a-app-backup", testInstance, nil),
			newObject("v1", "Pod", testNamespace, "d8a-app-migrate-x7k2p", testInstance, nil),
			newObject("v1", "Secret", testNamespace, "d8a-app-credentials", testInstance, nil),
			// another instance in the same namespace
			newObject("batch/v1", "Job", testNamespace, "d8a-other-migrate", "other", nil),
			// an instance of the same name in another namespace
			newObject("batch/v1", "Job", "neighbour", "d8a-app-migrate", testInstance, nil),
			// not labeled
			newObject("v1", "Secret", testNamespace, "user-credentials", "", nil),
		)

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))

		assert.ElementsMatch(t, []string{"d8a-other-migrate"}, left(t, client, jobs, testNamespace))
		assert.Empty(t, left(t, client, cronJobs, testNamespace))
		assert.Empty(t, left(t, client, pods, testNamespace))
		assert.ElementsMatch(t, []string{"user-credentials"}, left(t, client, secrets, testNamespace))
		assert.ElementsMatch(t, []string{"d8a-app-migrate"}, left(t, client, jobs, "neighbour"))
	})

	t.Run("deletes in the background, pinned to the listed object", func(t *testing.T) {
		client := newClient(t, newObject("batch/v1", "Job", testNamespace, "d8a-app-migrate", testInstance, nil))

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))

		actions := deletes(client)
		require.Len(t, actions, 1)

		opts := actions[0].GetDeleteOptions()
		require.NotNil(t, opts.PropagationPolicy)
		assert.Equal(t, metav1.DeletePropagationBackground, *opts.PropagationPolicy)
		require.NotNil(t, opts.Preconditions)
		require.NotNil(t, opts.Preconditions.UID)
		assert.Equal(t, types.UID(testNamespace+"/d8a-app-migrate"), *opts.Preconditions.UID)
	})

	t.Run("a group-qualified version names the group", func(t *testing.T) {
		client := newClient(t, newObject("batch/v1", "Job", testNamespace, "d8a-app-migrate", testInstance, nil))

		svc := NewService(client, log.NewNop())

		require.NoError(t, svc.Delete(ctx, testNamespace, testInstance, []Resource{{Kind: "Job", Version: "apps/v1"}}))
		assert.ElementsMatch(t, []string{"d8a-app-migrate"}, left(t, client, jobs, testNamespace))

		require.NoError(t, svc.Delete(ctx, testNamespace, testInstance, []Resource{{Kind: "Job", Version: "batch/v1"}}))
		assert.Empty(t, left(t, client, jobs, testNamespace))
	})

	t.Run("deletes rendered objects the uninstall leaves", func(t *testing.T) {
		client := newClient(t,
			// shared by nelm's default for hooks
			newObject("batch/v1", "Job", testNamespace, "d8a-app-hook", testInstance,
				map[string]string{"helm.sh/hook": "post-install"}),
			// deployed on install only, so shared by an annotation to survive an upgrade
			newObject("batch/v1", "Job", testNamespace, "d8a-app-post-install", testInstance,
				map[string]string{"werf.io/deploy-on": "post-install", "werf.io/ownership": "anyone"}),
			newObject("v1", "Secret", testNamespace, "d8a-app-shared", testInstance,
				map[string]string{"werf.io/ownership": "anyone"}),
			// kept on uninstall by the werf policy, which protects the object from nelm alone
			newObject("v1", "Secret", testNamespace, "d8a-app-werf-kept", testInstance,
				map[string]string{"werf.io/resource-policy": "keep"}),
			newObject("v1", "Secret", testNamespace, "d8a-app-werf-protected", testInstance,
				map[string]string{"werf.io/resource-policy": "skip-update, skip-delete"}),
		)

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))

		assert.Empty(t, left(t, client, jobs, testNamespace))
		assert.Empty(t, left(t, client, secrets, testNamespace))
	})

	t.Run("keeps objects with the keep Helm resource policy", func(t *testing.T) {
		client := newClient(t,
			newObject("v1", "Secret", testNamespace, "d8a-app-kept", testInstance,
				map[string]string{"helm.sh/resource-policy": "keep"}),
			newObject("v1", "Secret", testNamespace, "d8a-app-shared", testInstance,
				map[string]string{"werf.io/ownership": "anyone", "helm.sh/resource-policy": "keep"}),
			// whatever the werf policy beside it says
			newObject("v1", "Secret", testNamespace, "d8a-app-updatable", testInstance,
				map[string]string{"werf.io/resource-policy": "skip-update", "helm.sh/resource-policy": "keep"}),
		)

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))

		assert.ElementsMatch(t, []string{"d8a-app-kept", "d8a-app-shared", "d8a-app-updatable"}, left(t, client, secrets, testNamespace))
	})

	t.Run("does not delete an object already being deleted", func(t *testing.T) {
		terminating := newObject("batch/v1", "Job", testNamespace, "d8a-app-migrate", testInstance, nil)
		terminating.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		terminating.Finalizers = []string{"example.com/hold"}

		client := newClient(t, terminating)

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))
		assert.Empty(t, deletes(client))
	})

	t.Run("skips kinds it cannot delete by the instance label", func(t *testing.T) {
		client := newClient(t, newObject("v1", "Namespace", "", "d8a-app", testInstance, nil))

		err := NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, []Resource{
			{Kind: "VirtualMachine", Version: "v1"},
			{Kind: "Job", Version: "v2"},
			{Kind: "Binding", Version: "v1"},
			{Kind: "Namespace", Version: "v1"},
			{Kind: "Job"},
			{Version: "v1"},
		})
		require.NoError(t, err)

		assert.Empty(t, client.metadata.Actions())
		assert.ElementsMatch(t, []string{"d8a-app"}, left(t, client, namespaces, ""))
	})

	t.Run("reads discovery afresh for a kind missing from the cache", func(t *testing.T) {
		client := newClient(t, newObject("example.com/v1", "Widget", testNamespace, "d8a-app-widget", testInstance, nil))

		// the cache is filled before the CRD serving widgets is installed
		_, _, err := client.cached.ServerGroupsAndResources()
		require.NoError(t, err)

		client.discovery.Resources = append(client.discovery.Resources, &metav1.APIResourceList{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true, Verbs: allVerbs}},
		})

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, []Resource{{Kind: "Widget", Version: "v1"}}))
		assert.Empty(t, left(t, client, widgets, testNamespace))
	})

	t.Run("fails on what a retry can fix, after deleting the rest", func(t *testing.T) {
		client := newClient(t,
			newObject("batch/v1", "Job", testNamespace, "d8a-app-migrate", testInstance, nil),
			newObject("v1", "Pod", testNamespace, "d8a-app-migrate-x7k2p", testInstance, nil),
		)
		client.metadata.PrependReactor("list", "jobs", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewServiceUnavailable("unavailable")
		})

		err := NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared)
		require.Error(t, err)
		assert.True(t, apierrors.IsServiceUnavailable(err))

		assert.Empty(t, left(t, client, pods, testNamespace))
	})

	t.Run("ignores objects gone or replaced since the list", func(t *testing.T) {
		client := newClient(t,
			newObject("batch/v1", "Job", testNamespace, "d8a-app-gone", testInstance, nil),
			newObject("batch/v1", "Job", testNamespace, "d8a-app-replaced", testInstance, nil),
		)
		client.metadata.PrependReactor("delete", "jobs", func(action clienttesting.Action) (bool, runtime.Object, error) {
			name := action.(clienttesting.DeleteAction).GetName()
			if name == "d8a-app-gone" {
				return true, nil, apierrors.NewNotFound(jobs.GroupResource(), name)
			}

			return true, nil, apierrors.NewConflict(jobs.GroupResource(), name, errors.New("the UID in the precondition does not match"))
		})

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))
		assert.Len(t, deletes(client), 2)
	})

	t.Run("fails when discovery fails", func(t *testing.T) {
		client := newClient(t, newObject("batch/v1", "Job", testNamespace, "d8a-app-migrate", testInstance, nil))
		client.discovery.PrependReactor("*", "*", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("connection refused")
		})

		require.Error(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, testInstance, declared))
		assert.ElementsMatch(t, []string{"d8a-app-migrate"}, left(t, client, jobs, testNamespace))
	})

	t.Run("an instance name no label carries has nothing to delete", func(t *testing.T) {
		client := newClient(t)

		require.NoError(t, NewService(client, log.NewNop()).Delete(ctx, testNamespace, strings.Repeat("a", 64), declared))
		assert.Empty(t, client.metadata.Actions())
	})
}

// Copyright 2024 Flant JSC
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

package resources

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type fakeKubeClient struct {
	client.KubeClient
}

func (c *fakeKubeClient) Discovery() discovery.DiscoveryInterface {
	return &cachedDiscoveryClient{c.KubeClient.Discovery()}
}

type cachedDiscoveryClient struct {
	discovery.DiscoveryInterface
}

func (*cachedDiscoveryClient) Fresh() bool {
	return true
}

func (*cachedDiscoveryClient) Invalidate() {}

func TestResourcesWatcher(t *testing.T) {
	apiResources := []metav1.APIResource{
		{
			Kind:    "NodeGroup",
			Name:    "nodegroups",
			Verbs:   metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
			Group:   "deckhouse.io",
			Version: "v1",
		},
		{
			Kind:    "StaticInstance",
			Name:    "staticinstances",
			Verbs:   metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
			Group:   "deckhouse.io",
			Version: "v1alpha1",
		},
	}

	var (
		resources []*metav1.APIResourceList
		gvr       = make(map[schema.GroupVersionResource]string)
	)

	for _, apiResource := range apiResources {
		gvr[schema.GroupVersionResource{
			Group:    apiResource.Group,
			Version:  apiResource.Version,
			Resource: apiResource.Name,
		}] = apiResource.Kind + "List"

		resources = append(resources, &metav1.APIResourceList{
			GroupVersion: apiResource.Group + "/" + apiResource.Version,
			APIResources: []metav1.APIResource{apiResource},
		})
	}

	newFakeClient := func() *client.KubernetesClient {
		fakeClient := client.NewFakeKubernetesClientWithListGVR(gvr)

		discovery := fakeClient.Discovery().(*fakediscovery.FakeDiscovery)
		discovery.Resources = append(discovery.Resources, resources...)

		fakeClient.KubeClient = &fakeKubeClient{fakeClient.KubeClient}

		return fakeClient
	}

	getGroupVersionResource := func(resource *template.Resource) schema.GroupVersionResource {
		for _, apiResource := range apiResources {
			if apiResource.Kind == resource.GVK.Kind && apiResource.Group == resource.GVK.Group && apiResource.Version == resource.GVK.Version {
				return schema.GroupVersionResource{
					Group:    apiResource.Group,
					Version:  apiResource.Version,
					Resource: apiResource.Name,
				}
			}
		}

		panic("api resource not found")
	}

	const resourcesYAML = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: test
`

	t.Run("By default not ready", func(t *testing.T) {
		resources, err := template.ParseResourcesContent(resourcesYAML, nil)
		require.NoError(t, err)
		require.Len(t, resources, 2)

		fakeClient := newFakeClient()

		for _, resource := range resources {
			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).Create(context.TODO(), &resource.Object, metav1.CreateOptions{})
			require.NoError(t, err)

			checker, err := newResourceIsReadyChecker(fakeClient, resource)
			require.NoError(t, err)

			var ready bool

			for i := 0; i < 5; i++ {
				ready, err = checker.IsReady(context.TODO())
				require.NoError(t, err)
				if !ready {
					break
				}

				time.Sleep(time.Second)
			}

			require.False(t, ready)
		}
	})

	const nodeGroupYAML = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
`

	t.Run("NodeGroup is ready", func(t *testing.T) {
		resources, err := template.ParseResourcesContent(nodeGroupYAML, nil)
		require.NoError(t, err)
		require.Len(t, resources, 1)

		fakeClient := newFakeClient()

		for _, resource := range resources {
			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).Create(context.TODO(), &resource.Object, metav1.CreateOptions{})
			require.NoError(t, err)

			checker, err := newResourceIsReadyChecker(fakeClient, resource)
			require.NoError(t, err)

			resource.Object.Object["status"] = map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			}

			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).UpdateStatus(context.TODO(), &resource.Object, metav1.UpdateOptions{})
			require.NoError(t, err)

			var ready bool

			for i := 0; i < 5; i++ {
				ready, err = checker.IsReady(context.TODO())
				require.NoError(t, err)
				if ready {
					break
				}

				time.Sleep(time.Second)
			}

			require.True(t, ready)

			resource.Object.Object["status"] = map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "False",
					},
				},
			}

			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).UpdateStatus(context.TODO(), &resource.Object, metav1.UpdateOptions{})
			require.NoError(t, err)

			for i := 0; i < 5; i++ {
				ready, err = checker.IsReady(context.TODO())
				require.NoError(t, err)
				if !ready {
					break
				}

				time.Sleep(time.Second)
			}

			require.False(t, ready)
		}
	})

	const staticInstanceYAML = `
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: test
`

	t.Run("StaticInstance is ready", func(t *testing.T) {
		resources, err := template.ParseResourcesContent(staticInstanceYAML, nil)
		require.NoError(t, err)
		require.Len(t, resources, 1)

		fakeClient := newFakeClient()

		for _, resource := range resources {
			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).Create(context.TODO(), &resource.Object, metav1.CreateOptions{})
			require.NoError(t, err)

			checker, err := newResourceIsReadyChecker(fakeClient, resource)
			require.NoError(t, err)

			resource.Object.Object["status"] = map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			}

			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).UpdateStatus(context.TODO(), &resource.Object, metav1.UpdateOptions{})
			require.NoError(t, err)

			var ready bool

			for i := 0; i < 5; i++ {
				ready, err = checker.IsReady(context.TODO())
				require.NoError(t, err)
				if ready {
					break
				}

				time.Sleep(time.Second)
			}

			require.True(t, ready)

			resource.Object.Object["status"] = map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "False",
					},
				},
			}

			_, err = fakeClient.Dynamic().Resource(getGroupVersionResource(resource)).UpdateStatus(context.TODO(), &resource.Object, metav1.UpdateOptions{})
			require.NoError(t, err)

			for i := 0; i < 5; i++ {
				ready, err = checker.IsReady(context.TODO())
				require.NoError(t, err)
				if !ready {
					break
				}

				time.Sleep(time.Second)
			}

			require.False(t, ready)
		}
	})
}

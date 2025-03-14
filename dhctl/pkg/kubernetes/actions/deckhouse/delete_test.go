// Copyright 2021 Flant JSC
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

package deckhouse

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestDeleteMachinesIfResourcesExist(t *testing.T) {
	log.InitLogger("json")

	t.Run("Without sap API registration", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		err := checkMCMMachinesAPI(fakeClient)
		require.EqualError(t, err, "the server could not find the requested resource, GroupVersion \"machine.sapcloud.io/v1alpha1\" not found")
	})

	t.Run("With sap API registration", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		discovery := fakeClient.Discovery().(*fakediscovery.FakeDiscovery)
		discovery.Resources = append(discovery.Resources, &metav1.APIResourceList{
			GroupVersion: "machine.sapcloud.io/v1alpha1",
			APIResources: []metav1.APIResource{},
		})

		err := checkMCMMachinesAPI(fakeClient)
		require.EqualError(t, err, "0 of 2 resources found in the cluster")
	})

	t.Run("With only machines registration", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		discovery := fakeClient.Discovery().(*fakediscovery.FakeDiscovery)
		discovery.Resources = append(discovery.Resources, &metav1.APIResourceList{
			GroupVersion: "machine.sapcloud.io/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Machine",
					Name:       "machines",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "machine.sapcloud.io",
					Version:    "v1alpha1",
					Namespaced: true,
				},
			},
		})

		err := checkMCMMachinesAPI(fakeClient)
		require.EqualError(t, err, "1 of 2 resources found in the cluster")
	})

	t.Run("With machines and machinedeployments registration", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		discovery := fakeClient.Discovery().(*fakediscovery.FakeDiscovery)
		discovery.Resources = append(discovery.Resources, &metav1.APIResourceList{
			GroupVersion: "machine.sapcloud.io/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Kind:       "Machine",
					Name:       "machines",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "machine.sapcloud.io",
					Version:    "v1alpha1",
					Namespaced: true,
				},
				{
					Kind:       "MachineDeployment",
					Name:       "machinedeployments",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					Group:      "machine.sapcloud.io",
					Version:    "v1alpha1",
					Namespaced: true,
				},
			},
		})

		err := checkMCMMachinesAPI(fakeClient)
		require.NoError(t, err)
	})
}

func TestDeletePods(t *testing.T) {
	ctx := context.Background()
	log.InitLogger("json")

	t.Run("Without pods", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		err := DeletePods(ctx, fakeClient)
		require.NoError(t, err)
	})

	t.Run("With different pods", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "withPv",
				Namespace: "default",
			},
			Spec: v1.PodSpec{
				Volumes: []v1.Volume{{
					Name: "test",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test",
						},
					},
				}},
			},
		}
		_, err := fakeClient.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		pod2 := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "withDifferentPv",
				Namespace: "default",
			},
			Spec: v1.PodSpec{
				Volumes: []v1.Volume{{
					Name: "test",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				}},
			},
		}
		_, err = fakeClient.CoreV1().Pods("default").Create(context.TODO(), pod2, metav1.CreateOptions{})
		require.NoError(t, err)

		pod3 := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "withTwoPv",
				Namespace: "default",
			},
			Spec: v1.PodSpec{
				Volumes: []v1.Volume{
					{
						Name: "test",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "test",
							},
						},
					},
					{
						Name: "test2",
						VolumeSource: v1.VolumeSource{
							EmptyDir: &v1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		}
		_, err = fakeClient.CoreV1().Pods("default").Create(context.TODO(), pod3, metav1.CreateOptions{})
		require.NoError(t, err)

		pod4 := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "withoutPv",
				Namespace: "test-ns",
			},
			Spec: v1.PodSpec{},
		}
		_, err = fakeClient.CoreV1().Pods("test-ns").Create(context.TODO(), pod4, metav1.CreateOptions{})
		require.NoError(t, err)

		err = DeletePods(ctx, fakeClient)
		require.NoError(t, err)

		pods, err := fakeClient.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		require.NoError(t, err)

		require.Len(t, pods.Items, 2)
		require.Equal(t, "withDifferentPv", pods.Items[0].Name)
		require.Equal(t, "withoutPv", pods.Items[1].Name)
	})
}

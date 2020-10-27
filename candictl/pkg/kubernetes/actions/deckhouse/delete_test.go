package deckhouse

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/log"
)

func TestDeleteMachinesIfResourcesExist(t *testing.T) {
	log.InitLogger("simple")

	t.Run("Without sap API registration", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		err := checkMachinesAPI(fakeClient)
		require.EqualError(t, err, "GroupVersion \"machine.sapcloud.io/v1alpha1\" not found")
	})

	t.Run("With sap API registration", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		discovery := fakeClient.Discovery().(*fakediscovery.FakeDiscovery)
		discovery.Resources = append(discovery.Resources, &metav1.APIResourceList{
			GroupVersion: "machine.sapcloud.io/v1alpha1",
			APIResources: []metav1.APIResource{},
		})

		err := checkMachinesAPI(fakeClient)
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

		err := checkMachinesAPI(fakeClient)
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

		err := checkMachinesAPI(fakeClient)
		require.NoError(t, err)
	})
}

func TestDeletePods(t *testing.T) {
	log.InitLogger("simple")

	t.Run("Without pods", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		err := DeletePods(fakeClient)
		require.NoError(t, err)
	})

	t.Run("With different pods", func(t *testing.T) {
		fakeClient := client.NewFakeKubernetesClient()

		_, err := fakeClient.CoreV1().Pods("default").Create(&v1.Pod{
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
		})
		require.NoError(t, err)

		_, err = fakeClient.CoreV1().Pods("default").Create(&v1.Pod{
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
		})
		require.NoError(t, err)

		_, err = fakeClient.CoreV1().Pods("default").Create(&v1.Pod{
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
							}},
					},
					{
						Name: "test2",
						VolumeSource: v1.VolumeSource{
							EmptyDir: &v1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = fakeClient.CoreV1().Pods("test-ns").Create(&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "withoutPv",
				Namespace: "test-ns",
			},
			Spec: v1.PodSpec{},
		})
		require.NoError(t, err)

		err = DeletePods(fakeClient)
		require.NoError(t, err)

		pods, err := fakeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
		require.NoError(t, err)

		require.Len(t, pods.Items, 2)
		require.Equal(t, "withDifferentPv", pods.Items[0].Name)
		require.Equal(t, "withoutPv", pods.Items[1].Name)
	})
}

package deckhouse

import (
	"testing"

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

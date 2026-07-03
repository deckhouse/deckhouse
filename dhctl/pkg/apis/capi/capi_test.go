/*
Copyright 2025 Flant JSC

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

package capi

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

func machineResourceList(version string) *metav1.APIResourceList {
	return &metav1.APIResourceList{
		GroupVersion: group + "/" + version,
		APIResources: []metav1.APIResource{
			{Kind: "Machine", Name: machinesName},
			{Kind: "MachineDeployment", Name: deploymentsName},
		},
	}
}

func fakeDiscovery(resources ...*metav1.APIResourceList) *fakediscovery.FakeDiscovery {
	return &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
}

func TestResolve(t *testing.T) {
	t.Run("prefers v1beta2 when both are served", func(t *testing.T) {
		disco := fakeDiscovery(machineResourceList(versionV1beta1), machineResourceList(versionV1beta2))

		gvrs, err := Resolve(disco)
		require.NoError(t, err)
		require.Equal(t, versionV1beta2, gvrs.Version)
		require.Equal(t, V1beta2.MachineGVR, gvrs.MachineGVR)
	})

	t.Run("falls back to v1beta1 when v1beta2 is not served", func(t *testing.T) {
		disco := fakeDiscovery(machineResourceList(versionV1beta1))

		gvrs, err := Resolve(disco)
		require.NoError(t, err)
		require.Equal(t, versionV1beta1, gvrs.Version)
		require.Equal(t, V1beta1.MachineGVR, gvrs.MachineGVR)
		require.Equal(t, V1beta1.ClusterGVR, gvrs.ClusterGVR)
	})

	t.Run("uses v1beta2 when only v1beta2 is served", func(t *testing.T) {
		disco := fakeDiscovery(machineResourceList(versionV1beta2))

		gvrs, err := Resolve(disco)
		require.NoError(t, err)
		require.Equal(t, versionV1beta2, gvrs.Version)
	})

	t.Run("errors when neither version is served", func(t *testing.T) {
		disco := fakeDiscovery()

		_, err := Resolve(disco)
		require.Error(t, err)
	})

	t.Run("errors when version is served without machine resources", func(t *testing.T) {
		disco := fakeDiscovery(&metav1.APIResourceList{
			GroupVersion: V1beta2.GV.String(),
			APIResources: []metav1.APIResource{{Kind: "Cluster", Name: clustersName}},
		})

		_, err := Resolve(disco)
		require.Error(t, err)
	})
}

func TestSetForceDeleteDrainTimeout(t *testing.T) {
	t.Run("v1beta1 sets spec.nodeDrainTimeout duration", func(t *testing.T) {
		machine := map[string]interface{}{}

		require.NoError(t, V1beta1.SetForceDeleteDrainTimeout(machine))

		got, found, err := unstructured.NestedString(machine, "spec", "nodeDrainTimeout")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "10s", got)

		_, found, err = unstructured.NestedInt64(machine, "spec", "deletion", "nodeDrainTimeoutSeconds")
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("v1beta2 sets spec.deletion.nodeDrainTimeoutSeconds int", func(t *testing.T) {
		machine := map[string]interface{}{}

		require.NoError(t, V1beta2.SetForceDeleteDrainTimeout(machine))

		got, found, err := unstructured.NestedInt64(machine, "spec", "deletion", "nodeDrainTimeoutSeconds")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, int64(10), got)

		_, found, err = unstructured.NestedString(machine, "spec", "nodeDrainTimeout")
		require.NoError(t, err)
		require.False(t, found)
	})
}

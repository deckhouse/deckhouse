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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis"
)

const (
	group = "cluster.x-k8s.io"

	versionV1beta1 = "v1beta1"
	versionV1beta2 = "v1beta2"

	clustersName    = "clusters"
	machinesName    = "machines"
	deploymentsName = "machinedeployments"
)

// preferredVersions lists the served CAPI API versions in priority order.
// v1beta2 is preferred to avoid deprecation warnings on upgraded clusters;
// v1beta1 is kept as a fallback for clusters that do not serve v1beta2 yet.
var preferredVersions = []string{versionV1beta2, versionV1beta1}

// GVRs holds CAPI resources bound to a concrete served API version.
type GVRs struct {
	Version              string
	GV                   schema.GroupVersion
	ClusterGVR           schema.GroupVersionResource
	MachineGVR           schema.GroupVersionResource
	MachineDeploymentGVR schema.GroupVersionResource
}

func gvrsForVersion(version string) GVRs {
	return GVRs{
		Version:              version,
		GV:                   schema.GroupVersion{Group: group, Version: version},
		ClusterGVR:           schema.GroupVersionResource{Group: group, Version: version, Resource: clustersName},
		MachineGVR:           schema.GroupVersionResource{Group: group, Version: version, Resource: machinesName},
		MachineDeploymentGVR: schema.GroupVersionResource{Group: group, Version: version, Resource: deploymentsName},
	}
}

// V1beta1 and V1beta2 are the static resource sets for each supported version.
var (
	V1beta1 = gvrsForVersion(versionV1beta1)
	V1beta2 = gvrsForVersion(versionV1beta2)
)

// SetForceDeleteDrainTimeout sets the machine node drain timeout to a small
// value so the machine can be force-deleted (without draining). The field path
// differs between versions: v1beta1 uses spec.nodeDrainTimeout (a duration
// string), v1beta2 uses spec.deletion.nodeDrainTimeoutSeconds (int seconds).
func (g GVRs) SetForceDeleteDrainTimeout(machine map[string]interface{}) error {
	if g.Version == versionV1beta1 {
		return unstructured.SetNestedField(machine, "10s", "spec", "nodeDrainTimeout")
	}
	return unstructured.SetNestedField(machine, int64(10), "spec", "deletion", "nodeDrainTimeoutSeconds")
}

// Resolve detects the CAPI API version served by the cluster, preferring
// v1beta2 and falling back to v1beta1. It returns an error if neither version
// serves both the Machine and MachineDeployment resources.
func Resolve(disco discovery.DiscoveryInterface) (GVRs, error) {
	var lastErr error
	for _, version := range preferredVersions {
		gv := schema.GroupVersion{Group: group, Version: version}
		list, err := disco.ServerResourcesForGroupVersion(gv.String())
		if err != nil {
			lastErr = err
			continue
		}
		if hasMachineResources(list) {
			return gvrsForVersion(version), nil
		}
	}
	if lastErr != nil {
		return GVRs{}, lastErr
	}
	return GVRs{}, fmt.Errorf("no served %s version provides Machine and MachineDeployment resources", group)
}

func hasMachineResources(list *metav1.APIResourceList) bool {
	var found int
	for _, resource := range list.APIResources {
		if resource.Kind == "Machine" || resource.Kind == "MachineDeployment" {
			found++
		}
	}
	return found >= 2
}

var (
	clusterAPIResource = metav1.APIResource{
		Kind:       "Cluster",
		Name:       clustersName,
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Namespaced: true,
	}
	machineAPIResource = metav1.APIResource{
		Kind:       "Machine",
		Name:       machinesName,
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Namespaced: true,
	}
	machineDeploymentAPIResource = metav1.APIResource{
		Kind:       "MachineDeployment",
		Name:       deploymentsName,
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Namespaced: true,
	}
)

// ListsGVRs returns the list-kind to GVR mapping for the preferred (v1beta2)
// version. It is used to register CAPI resources with fake clients in tests.
func ListsGVRs() apis.ListKindToGVR {
	return apis.CopyListKindToGVR(apis.ListKindToGVR{
		"ClusterList":           V1beta2.ClusterGVR,
		"MachineDeploymentList": V1beta2.MachineDeploymentGVR,
		"MachineList":           V1beta2.MachineGVR,
	})
}

// APIResourcesList returns the preferred (v1beta2) API resources list, used to
// populate fake discovery in tests.
func APIResourcesList() *metav1.APIResourceList {
	return &metav1.APIResourceList{
		GroupVersion: V1beta2.GV.String(),
		APIResources: []metav1.APIResource{clusterAPIResource, machineAPIResource, machineDeploymentAPIResource},
	}
}

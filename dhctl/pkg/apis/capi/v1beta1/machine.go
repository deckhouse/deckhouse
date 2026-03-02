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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis"
)

const (
	group        = "cluster.x-k8s.io"
	groupVersion = "v1beta1"

	machinesName    = "machines"
	deploymentsName = "machinedeployments"
)

var (
	MachineDeploymentGVR = schema.GroupVersionResource{Group: group, Version: groupVersion, Resource: deploymentsName}
	MachineGVR           = schema.GroupVersionResource{Group: group, Version: groupVersion, Resource: machinesName}

	listKindsToGVR = apis.ListKindToGVR{
		"MachineDeploymentList": MachineDeploymentGVR,
		"MachineList":           MachineGVR,
	}

	GV = schema.GroupVersion{
		Group:   group,
		Version: groupVersion,
	}

	MachineDeploymentAPIResource = metav1.APIResource{
		Kind:       "MachineDeployment",
		Name:       deploymentsName,
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Version:    groupVersion,
		Namespaced: true,
	}
	MachineAPIResource = metav1.APIResource{
		Kind:       "Machine",
		Name:       machinesName,
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Version:    groupVersion,
		Namespaced: true,
	}
)

func ListsGVRs() apis.ListKindToGVR {
	return apis.CopyListKindToGVR(listKindsToGVR)
}

func APIResourcesList() *metav1.APIResourceList {
	return &metav1.APIResourceList{
		GroupVersion: GV.String(),
		APIResources: []metav1.APIResource{MachineAPIResource, MachineDeploymentAPIResource},
	}
}

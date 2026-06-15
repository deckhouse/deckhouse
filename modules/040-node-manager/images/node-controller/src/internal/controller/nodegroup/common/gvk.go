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

package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	MCMMachineDeploymentGVK = schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeployment",
	}
	CAPIMachineDeploymentGVK = schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeployment",
	}
)

func NewUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

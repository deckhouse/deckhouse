/*
Copyright 2021 Flant JSC

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

package template

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeGroupConfiguration is an user scripts for node configuration.
type NodeGroupConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node group.
	Spec NodeGroupConfigurationSpec `json:"spec"`

	// Most recently observed status of the node.
	// Populated by the system.

	Status NodeGroupConfigurationStatus `json:"status,omitempty"`
}

type NodeGroupConfigurationSpec struct {
	Content    string   `json:"content"`
	Weight     int      `json:"weight"`
	NodeGroups []string `json:"nodeGroups"`
	Bundles    []string `json:"bundles"`
}

func (ngc NodeGroupConfigurationSpec) IsEqual(newSpec NodeGroupConfigurationSpec) bool {
	if ngc.Weight != newSpec.Weight {
		return false
	}

	if ngc.Content != newSpec.Content {
		return false
	}

	oldNGs := ngc.NodeGroups[:]
	newNGs := newSpec.NodeGroups[:]

	if len(oldNGs) != len(newNGs) {
		return false
	}

	if len(newNGs) == 0 {
		return true
	}

	sort.Strings(oldNGs)
	sort.Strings(newNGs)

	for i := 0; i < len(newNGs); i++ {
		if oldNGs[i] != newNGs[i] {
			return false
		}
	}

	oldBundles := ngc.Bundles[:]
	newBundles := newSpec.Bundles[:]

	if len(oldBundles) != len(newBundles) {
		return false
	}

	if len(newBundles) == 0 {
		return true
	}

	sort.Strings(oldBundles)
	sort.Strings(newBundles)

	for i := 0; i < len(newBundles); i++ {
		if oldBundles[i] != newBundles[i] {
			return false
		}
	}

	return true
}

type NodeGroupConfigurationStatus struct {
}

/*
Copyright 2023 Flant JSC

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
	"fmt"

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

// GenerateScriptName generates name for a bash script like xxx_some_name.sh, We have to specify integer part with 3 digits
func (ng NodeGroupConfiguration) GenerateScriptName() string {
	return fmt.Sprintf("%03d_%s", ng.Spec.Weight, ng.Name)
}

type NodeGroupConfigurationSpec struct {
	Content    string   `json:"content"`
	Weight     int      `json:"weight"`
	NodeGroups []string `json:"nodeGroups"`
}

func (ngc NodeGroupConfigurationSpec) IsEqual(newSpec NodeGroupConfigurationSpec) bool {
	if ngc.Weight != newSpec.Weight {
		return false
	}

	if ngc.Content != newSpec.Content {
		return false
	}

	if slicesIsEqual(ngc.NodeGroups, newSpec.NodeGroups) {
		return false
	}

	return true
}

type NodeGroupConfigurationStatus struct {
}

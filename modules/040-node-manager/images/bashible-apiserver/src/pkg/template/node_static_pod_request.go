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

package template

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeStaticPodRequest asks kubelet on the selected nodes to run one pod
// outside the scheduler. Owned by node-controller; read here as a
// NodeGroupConfiguration is read.
type NodeStaticPodRequest struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeStaticPodRequestSpec `json:"spec"`
}

type NodeStaticPodRequestSpec struct {
	// NodeGroupSelector narrows the pod to the named NodeGroups. Empty selects
	// every group.
	NodeGroupSelector NodeGroupSelector `json:"nodeGroupSelector,omitempty"`
	// Manifest is the whole Pod document; $MY_IP is the node's address.
	Manifest string `json:"manifest"`
}

type NodeGroupSelector struct {
	MatchNames []string `json:"matchNames,omitempty"`
}

func (r NodeStaticPodRequestSpec) IsEqual(newSpec NodeStaticPodRequestSpec) bool {
	if r.Manifest != newSpec.Manifest {
		return false
	}

	return slices.Equal(r.NodeGroupSelector.MatchNames, newSpec.NodeGroupSelector.MatchNames)
}

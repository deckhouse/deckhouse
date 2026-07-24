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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeExtensionRequest asks for a system extension (a sysext image, optionally
// with kernel modules) to be merged onto the nodes it selects.
//
// The image is not named directly but templated: ImageTemplate carries ${KEY}
// placeholders that node-controller fills in — KERNEL_VERSION from the node the
// extension is being built for, the rest from Params — before resolving the
// result to an image@digest recorded in the status.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ner
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Image,jsonPath=.status.resolvedImage,type=string
// +kubebuilder:printcolumn:name=Age,jsonPath=.metadata.creationTimestamp,type=date
type NodeExtensionRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeExtensionRequestSpec   `json:"spec"`
	Status NodeExtensionRequestStatus `json:"status,omitempty"`
}

// NodeExtensionRequestSpec describes what to merge and where.
type NodeExtensionRequestSpec struct {
	// ImageTemplate is the sysext image reference with ${KEY} placeholders, for
	// example "registry.deckhouse.io/deckhouse/sysext/drbd:${DRBD_VERSION}-k${KERNEL_VERSION}".
	// KERNEL_VERSION is reserved and resolved by node-controller for each node;
	// every other key is taken from Params.
	// +kubebuilder:validation:MinLength=1
	ImageTemplate string `json:"imageTemplate"`

	// Params provides the values for the ${KEY} placeholders in ImageTemplate,
	// except the reserved KERNEL_VERSION.
	// +optional
	Params map[string]string `json:"params,omitempty"`

	// NodeGroupSelector narrows the extension to nodes of the named NodeGroups.
	// +optional
	NodeGroupSelector NodeGroupSelector `json:"nodeGroupSelector,omitempty"`

	// NodeSelector narrows the extension to nodes carrying the given labels.
	// +optional
	NodeSelector NodeSelector `json:"nodeSelector,omitempty"`

	// KernelModules are the modules to load once the extension is merged.
	// +optional
	KernelModules []KernelModule `json:"kernelModules,omitempty"`
}

// NodeGroupSelector selects NodeGroups by name.
type NodeGroupSelector struct {
	// MatchNames is the set of NodeGroup names the extension applies to.
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`
}

// NodeSelector selects nodes by label.
type NodeSelector struct {
	// MatchLabels is the set of node labels a node must carry to be selected.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// KernelModule is a module to load once the extension is merged, with optional
// parameters.
type KernelModule struct {
	// Name is the kernel module name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`
	Name string `json:"name"`

	// Params are the module parameters, passed as "key=value" strings.
	// +optional
	Params []string `json:"params,omitempty"`
}

// NodeExtensionRequestStatus is written by node-controller as it resolves the
// image and matches the selectors.
type NodeExtensionRequestStatus struct {
	// ObservedGeneration is the generation of the spec this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions carry the details of the request's progress.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ResolvedImage is the last image@digest ImageTemplate resolved to.
	// +optional
	ResolvedImage string `json:"resolvedImage,omitempty"`

	// MatchedNodeGroups are the NodeGroups the selectors currently match.
	// +optional
	MatchedNodeGroups []string `json:"matchedNodeGroups,omitempty"`
}

// NodeExtensionRequestList is a list of NodeExtensionRequest objects.
//
// +kubebuilder:object:root=true
type NodeExtensionRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeExtensionRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeExtensionRequest{}, &NodeExtensionRequestList{})
}

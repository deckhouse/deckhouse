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
// The image is a module's sysext. It is not named by a full reference: the
// registry-packages-proxy already reaches a module's repo from its ModuleSource,
// so a request only pins the digest and the repo path (which defaults to the
// module name), and node-controller records the resolved repo@digest in status.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ner
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Phase,jsonPath=.status.phase,type=string
// +kubebuilder:printcolumn:name=Age,jsonPath=.metadata.creationTimestamp,type=date
type NodeExtensionRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeExtensionRequestSpec   `json:"spec"`
	Status NodeExtensionRequestStatus `json:"status,omitempty"`
}

// NodeExtensionRequestSpec describes what to merge and where.
type NodeExtensionRequestSpec struct {
	// Sysext locates the extension image to merge.
	Sysext Sysext `json:"sysext"`

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

// Sysext locates a module's system-extension image. The image lives in the
// module's registry repo, which the registry-packages-proxy already reaches from
// the ModuleSource, so a request pins only the digest and — when it differs from
// the module name — the path.
type Sysext struct {
	// Name is the sysext name: it is matched against the image's
	// extension-release and installed on the node as "<name>.raw".
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Digest is the sysext image manifest digest.
	// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
	Digest string `json:"digest"`

	// Path is the image's repo path under the ModuleSource registry. Defaults to
	// the module name (the module.deckhouse.io/name label on the request).
	// +optional
	Path string `json:"path,omitempty"`

	// ModuleSource names the ModuleSource whose registry hosts the image; the
	// proxy resolves its credentials from that source. Defaults to "deckhouse".
	// +optional
	ModuleSource string `json:"moduleSource,omitempty"`
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
// sysext and matches the selectors. It mirrors the NodeConfig status shape: a
// phase, typed conditions, and the observed generation.
type NodeExtensionRequestStatus struct {
	// ObservedGeneration is the generation of the spec this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is Ready when the sysext resolves to an image the selected nodes can
	// pull, or Degraded when it cannot — the Ready condition carries the reason.
	// +kubebuilder:validation:Enum=Ready;Degraded
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions carry the details of the request's progress. Ready answers
	// whether the sysext resolved.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

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

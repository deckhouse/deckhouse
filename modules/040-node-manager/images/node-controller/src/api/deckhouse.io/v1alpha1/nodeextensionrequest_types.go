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
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeExtensionRequest asks for an extension to be installed on the nodes it selects: a
// system extension (a sysext image, optionally with kernel modules) or a registry package.
// The requester says which in spec.extension.kind; no module is resolved.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ner
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Kind,jsonPath=.spec.extension.kind,type=string
// +kubebuilder:printcolumn:name=Phase,jsonPath=.status.phase,type=string
// +kubebuilder:printcolumn:name=Matched,jsonPath=.status.matchedNodes,type=integer
// +kubebuilder:printcolumn:name=Applied,jsonPath=.status.appliedNodes,type=integer
// +kubebuilder:printcolumn:name=Failed,jsonPath=.status.failedNodes,type=integer
// +kubebuilder:printcolumn:name=Age,jsonPath=.metadata.creationTimestamp,type=date
type NodeExtensionRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeExtensionRequestSpec   `json:"spec"`
	Status NodeExtensionRequestStatus `json:"status,omitempty"`
}

// NodeExtensionRequestSpec describes what to install and where.
type NodeExtensionRequestSpec struct {
	// Extension locates the image to install.
	Extension NodeExtension `json:"extension"`

	// NodeGroupSelector narrows the extension to nodes of the named NodeGroups.
	// +optional
	NodeGroupSelector NodeGroupSelector `json:"nodeGroupSelector,omitempty"`

	// NodeSelector narrows the extension to nodes carrying the given labels.
	// +optional
	NodeSelector NodeSelector `json:"nodeSelector,omitempty"`

	// KernelModules are the modules to load once a system extension is merged. A
	// registry package cannot load any, and such a request reaches no node.
	// +optional
	KernelModules []KernelModule `json:"kernelModules,omitempty"`
}

// NodeExtension locates an image the way the registry-packages-proxy locates any package:
// by name and digest, optionally narrowed to a repository and path, and says what it is.
// The location is forwarded verbatim into the NodeConfig extension; no module resolved.
type NodeExtension struct {
	// Name is copied verbatim into the NodeConfig extension name: a system extension is
	// installed as "<name>.raw", a registry package is recorded under
	// /var/cache/registrypackages/<name>.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// Digest is the image manifest digest.
	// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
	Digest string `json:"digest"`

	// Kind is what the image is, chosen by the requester and immutable: a Package goes to
	// nodes bashible configures, a SystemExtension to Deckhouse Engine nodes.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Package;SystemExtension
	Kind ExtensionKind `json:"kind"`

	// Repository is the key the registry-packages-proxy resolves credentials by
	// (its --rpp-repository), for example a ModuleSource's spec.registry.repo. Left
	// empty, the image is pulled from the cluster's main registry.
	// +optional
	Repository string `json:"repository,omitempty"`

	// Path is the image's path within the repository (the proxy's --rpp-path).
	// Left empty, the repository root is used.
	// +optional
	Path string `json:"path,omitempty"`
}

// IsReservedExtensionName reports whether an extension name belongs to a platform
// extension and so may not be requested. Kept next to the NodeExtension contract so the
// admission webhook and the controller backstop enforce one list.
func IsReservedExtensionName(name string) bool {
	return slices.Contains([]string{"containerd", "kubelet", "kubernetes-cni", "nodelet"}, name)
}

// ExtensionKind is what the image of a request is, as its requester declares it.
type ExtensionKind string

const (
	// ExtensionKindSystemExtension is a sysext image, merged by the agent on an Engine node.
	ExtensionKindSystemExtension ExtensionKind = "SystemExtension"
	// ExtensionKindPackage is a registry package with install scripts, installed by the
	// agent on a node bashible configures.
	ExtensionKindPackage ExtensionKind = "Package"
)

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

// NodeExtensionRequestStatus is written by node-controller as it matches the selectors.
// It mirrors the NodeConfig status shape: a phase, typed conditions, and the observed
// generation.
type NodeExtensionRequestStatus struct {
	// ObservedGeneration is the generation of the spec this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is Ready when the extension reaches the nodes it selects, or Degraded
	// when it does not — the Ready condition carries the reason.
	// +kubebuilder:validation:Enum=Ready;Degraded
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions carry the details of the request's progress. Ready answers
	// whether the extension reaches its nodes.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// MatchedNodeGroups are the NodeGroups the selectors currently match.
	// +optional
	MatchedNodeGroups []string `json:"matchedNodeGroups,omitempty"`

	// AppliedNodes is how many of the selected nodes report the extension installed at the digest
	// their NodeConfig carries, the previous one while a node waits for a rollout slot; FailedNodes
	// how many report it refused. No omitempty on the counters: 0 is an answer the printer columns show.
	// +optional
	AppliedNodes int32 `json:"appliedNodes"`
	// +optional
	FailedNodes int32 `json:"failedNodes"`

	// MatchedNodes is how many nodes of the groups that install this kind of image the
	// selectors pick; PendingNodes how many of them have reported nothing yet.
	// +optional
	MatchedNodes int32 `json:"matchedNodes"`
	// +optional
	PendingNodes int32 `json:"pendingNodes"`

	// FailureMessage is what the nodes say about the refusal, taken from one of
	// them. A bad image fails the same way everywhere, so one message is the
	// whole story — and it is the operator's shortcut past a serial console.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
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

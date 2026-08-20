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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type VirtualControlPlaneDatastoreRef struct {
	// Name is the datastore configuration name used by the tenant control plane.
	Name string `json:"name"`
}

type VirtualControlPlaneExpose struct {
	// Type is how the tenant Kubernetes API is published. Only LoadBalancer is supported for now
	// (the per-VCP ALB uses a LoadBalancer inlet).
	// +kubebuilder:validation:Enum=LoadBalancer
	// +kubebuilder:default=LoadBalancer
	// +optional
	Type string `json:"type,omitempty"`
}

type VirtualControlPlaneKubeconfigSecretRef struct {
	// Namespace is the namespace that contains the kubeconfig Secret.
	Namespace string `json:"namespace,omitempty"`

	// Name is the kubeconfig Secret name.
	Name string `json:"name,omitempty"`
}

// VirtualControlPlaneNetworking is the tenant cluster's network configuration. It is the single
// source of truth for the tenant's Service/Pod address space: the apiserver's
// --service-cluster-ip-range and --service-account-issuer, kube-controller-manager's
// --cluster-cidr/--node-cidr-mask-size, cilium's IPAM configuration, the tenant DNS ClusterIP
// (derived, not stored here) and the tenant ClusterConfiguration all flow from it.
//
// Every field is immutable after creation (enforced per-field below so the CEL message can name
// the remedy): the tenant apiserver's serving certificate SAN, already-allocated Service
// ClusterIPs, already-allocated node PodCIDRs and the tenant's --service-account-issuer are all
// derived from these values once, at PKI/manifest render time, and do not react to a later change.
// There is no in-place migration path — recreate the VirtualControlPlane instead.
type VirtualControlPlaneNetworking struct {
	// ServiceSubnetCIDR is the address space of the tenant cluster's Services (kube-apiserver
	// --service-cluster-ip-range and kube-controller-manager --service-cluster-ip-range). The
	// tenant apiserver serving certificate carries the range's 1st address as an IP SAN and the
	// tenant DNS Service is addressed at the range's 10th address.
	//
	// Warning: changing this value on a running tenant is unsafe and is blocked by immutability.
	// Existing Services keep ClusterIPs from the current range, the apiserver serving certificate
	// would silently regenerate with a different SAN (dropping the old kubernetes.default SAN),
	// and the tenant DNS Service stays stranded at the old address. Recreate the
	// VirtualControlPlane and migrate workloads instead.
	// Must be an IPv4 CIDR between /12 and /24. /12 is the largest range kube-apiserver accepts
	// (it rejects anything above 2^20 addresses); /24 leaves room for the kubernetes Service, the
	// DNS Service and every Deckhouse module Service.
	// +kubebuilder:validation:Pattern=`^((25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])/(1[2-9]|2[0-4])$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceSubnetCIDR is immutable: existing Services hold ClusterIPs from the current range and the apiserver serving certificate cannot be re-addressed in place. Create a new VirtualControlPlane and migrate workloads."
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`

	// PodSubnetCIDR is the address space of the tenant cluster's Pods, passed to
	// kube-controller-manager as --cluster-cidr and to cilium as the Kubernetes-IPAM Pod CIDR
	// source.
	//
	// Warning: changing this value on a running tenant is unsafe and is blocked by immutability.
	// Existing nodes and Pods already hold addresses from the current range; switching it requires
	// re-allocating every node's PodCIDR, which in practice means recreating the nodes. Recreate
	// the VirtualControlPlane and migrate workloads instead.
	// Must be an IPv4 CIDR between /8 and /24, and shorter than podSubnetNodeCIDRPrefix so that at
	// least one node subnet fits (see the rule on the networking block itself).
	// +kubebuilder:validation:Pattern=`^((25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])/([89]|1[0-9]|2[0-4])$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="podSubnetCIDR is immutable: existing nodes and Pods already hold addresses from the current range. Create a new VirtualControlPlane and migrate workloads."
	PodSubnetCIDR string `json:"podSubnetCIDR"`

	// PodSubnetNodeCIDRPrefix is the prefix size of the Pod network allocated to each node out of
	// PodSubnetCIDR, passed to kube-controller-manager as --node-cidr-mask-size.
	//
	// Warning: changing this value on a running tenant is unsafe and is blocked by immutability.
	// Nodes that already received a PodCIDR sized from the previous prefix keep it; a mismatched
	// prefix on new nodes silently fragments the Pod address space. Recreate the
	// VirtualControlPlane and migrate workloads instead.
	// This is a policy choice, not something derivable from podSubnetCIDR: the gap between the two
	// trades maximum node count against Pod addresses per node. With podSubnetCIDR /16 — /23 gives
	// 128 nodes x 510 addresses, /24 gives 256 x 254, /25 gives 512 x 126.
	// +kubebuilder:validation:Pattern=`^(1[6-9]|2[0-9]|30)$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="podSubnetNodeCIDRPrefix is immutable: nodes already hold PodCIDRs sized from the previous prefix. Create a new VirtualControlPlane and migrate workloads."
	PodSubnetNodeCIDRPrefix string `json:"podSubnetNodeCIDRPrefix"`

	// ClusterDomain is the tenant cluster's DNS domain. It is used to build the apiserver's
	// --service-account-issuer and the "kubernetes.default.svc.<ClusterDomain>" certificate SAN.
	//
	// Warning: changing this value on a running tenant is unsafe and is blocked by immutability.
	// The apiserver serving certificate SAN and --service-account-issuer are derived from it once,
	// at creation. Recreate the VirtualControlPlane and migrate workloads instead.
	// +kubebuilder:default=cluster.virtual
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDomain is immutable: the apiserver serving certificate and --service-account-issuer are derived from it at creation. Create a new VirtualControlPlane and migrate workloads."
	// +optional
	ClusterDomain string `json:"clusterDomain,omitempty"`
}

type VirtualControlPlaneSpec struct {
	// KubernetesVersion is the desired Kubernetes version for the tenant control plane.
	KubernetesVersion string `json:"kubernetesVersion"`

	// Replicas is the desired number of control plane replicas.
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Networking is the tenant cluster's network configuration (Service/Pod CIDRs, cluster
	// domain). Required so that the "clusterDomain" default always materialises and its
	// immutability CEL rule always has an oldSelf to compare against.
	//
	// The cross-field rule below is the one relationship the per-field patterns cannot express:
	// each node is carved a podSubnetNodeCIDRPrefix-sized slice out of podSubnetCIDR, so the node
	// prefix must be strictly longer than the pod CIDR's own prefix, otherwise not a single node
	// subnet fits. The gap is log2(max nodes): /16 with /24 per node is 2^8 = 256 nodes.
	// +kubebuilder:validation:XValidation:rule="int(self.podSubnetNodeCIDRPrefix) > int(self.podSubnetCIDR.split('/')[1])",message="podSubnetNodeCIDRPrefix must be greater than the prefix length of podSubnetCIDR, otherwise no node subnet fits into the Pod address space"
	Networking VirtualControlPlaneNetworking `json:"networking"`

	// DatastoreRef points to the datastore configuration used by the tenant control plane.
	// +optional
	DatastoreRef *VirtualControlPlaneDatastoreRef `json:"datastoreRef,omitempty"`

	// Expose describes how the tenant Kubernetes API should be published.
	// +optional
	Expose *VirtualControlPlaneExpose `json:"expose,omitempty"`
}

type VirtualControlPlaneStatus struct {
	// Endpoint is the published Kubernetes API endpoint for this virtual control plane.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// KubeconfigSecretRef points to a Secret with a kubeconfig for this virtual control plane.
	// +optional
	KubeconfigSecretRef *VirtualControlPlaneKubeconfigSecretRef `json:"kubeconfigSecretRef,omitempty"`

	// ObservedKubernetesVersion is the version currently observed by the controller.
	// +optional
	ObservedKubernetesVersion string `json:"observedKubernetesVersion,omitempty"`

	// Conditions describe the current state of the virtual control plane.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vcp
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetesVersion",description="Desired Kubernetes version"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="Desired number of control plane replicas"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Virtual control plane readiness"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type VirtualControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualControlPlaneSpec   `json:"spec,omitempty"`
	Status VirtualControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type VirtualControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualControlPlane{}, &VirtualControlPlaneList{})
}

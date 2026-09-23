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

// RegistryUpstreamSpec declares an additional registry the cluster should route
// to, on top of the primary.
//
// This is the multi-writer part of the contract. Any writer creates these
// objects: a module pulling images from its vendor registry (with an ownerRef on
// itself so the object is garbage-collected with the module), an administrator,
// or a user proxying ghcr/docker-hub. One object per upstream, so a GitOps tool
// owning ModuleConfig never fights a module over field ownership.
//
// Traffic through these upstreams is ALWAYS transit: the agent proxies the pull,
// substituting credentials and CA, and streams it through without writing to
// disk or to the cache. Cache space is finite and reserved for Deckhouse
// components, all the more so in air-gap where the cache is the only source.
type RegistryUpstreamSpec struct {
	// Match is the registry host intercepted on the nodes, e.g.
	// "images.virtualization.example.com".
	//
	// A match already claimed by another writer, or one that would shadow the
	// primary path, is rejected by the controller rather than silently merged.
	// +kubebuilder:validation:MinLength=1
	Match string `json:"match"`

	// Upstream is where the intercepted pulls are actually sent.
	Upstream Upstream `json:"upstream"`
}

// RegistryUpstreamStatus is the arbitration result, written by the controller.
type RegistryUpstreamStatus struct {
	// ObservedGeneration is the spec generation the controller last acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Accepted means the controller compiled this upstream into the per-node
	// layout.
	// +optional
	Accepted bool `json:"accepted,omitempty"`

	// Conflict explains why the object was rejected: who already holds the match,
	// or that it shadows the primary path. Empty when accepted.
	// +optional
	Conflict string `json:"conflict,omitempty"`

	// Conditions carry Accepted with a machine-readable reason.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,path=registryupstreams,singular=registryupstream,shortName=rup

// RegistryUpstream is an additional registry routed through the cluster in
// transit, declared by any writer.
type RegistryUpstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryUpstreamSpec   `json:"spec,omitempty"`
	Status RegistryUpstreamStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryUpstreamList is a list of RegistryUpstream.
type RegistryUpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryUpstream `json:"items"`
}

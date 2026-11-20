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

// RegistryMirrorSpec declares a registry near a group of nodes that holds a copy of the
// primary image set, for those nodes to pull from instead of the in-cluster storage.
//
// Meant for nodes far from the control plane: workers in another region pulling large images
// across a slow or expensive link. The administrator runs and fills the mirror (a Harbor
// proxy-cache or replication project, `d8 mirror push`, anything that serves the image set by
// digest); the cluster only decides which nodes ask it, and for what.
//
// Asked by digest only: blobs and manifests named by their digest, never tags. A tag is
// resolved where it is resolved without a mirror, so what version runs stays the cluster's
// decision, and content served by digest is verified against the digest by the runtime and by
// the agent. A mirror therefore needs no more trust than its availability, and a miss costs one
// request to a nearby registry before the pull goes the usual way.
type RegistryMirrorSpec struct {
	// NodeSelector picks the nodes that ask this mirror first. An empty selector picks no node: a
	// mirror for every node is a choice to spell out, not one to make by leaving a field empty.
	//
	// A selector that picks no node is accepted all the same, with MatchedNodes at zero: a node
	// group scaled to zero still has its mirror waiting for it.
	//
	// A node is served by one mirror at most. An object that picks a node already served by an
	// older one is rejected whole, with the reason in its Accepted condition.
	NodeSelector metav1.LabelSelector `json:"nodeSelector"`

	// Source is the mirror itself: where it is, under which path it serves the image set, and how
	// to authenticate to it. Its mirrors are other addresses of the same content, asked in order
	// after it.
	//
	// The path replaces the image set's own prefix: an image the cluster names
	// `<in-cluster address>/<repository>@<digest>` is asked as `<host>/<path>/<repository>@<digest>`.
	//
	// The limits are the ones of RegistryUpstream.spec.upstream; see the hand-written CRD
	// (modules/038-registry/crds/registrymirror.yaml) and go_lib/registry/apis/mirrors.
	Source Upstream `json:"source"`
}

// RegistryMirrorStatus is written by the controller.
type RegistryMirrorStatus struct {
	// ObservedGeneration is the spec generation the controller last acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// MatchedNodes is how many nodes the mirror is laid out for. Zero while it is not accepted.
	// +optional
	MatchedNodes int32 `json:"matchedNodes"`

	// Conditions carry Accepted, with the reason of a rejection.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,path=registrymirrors,singular=registrymirror

// RegistryMirror is a nearby copy of the primary image set for a group of nodes.
type RegistryMirror struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryMirrorSpec   `json:"spec,omitempty"`
	Status RegistryMirrorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryMirrorList is a list of RegistryMirror.
type RegistryMirrorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryMirror `json:"items"`
}

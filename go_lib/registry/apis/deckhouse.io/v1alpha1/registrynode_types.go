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

// RegistryNodeSpec is the complete layout for one node, compiled by the
// controller. Named after the node, with an ownerRef on it.
//
// The object is deliberately flat and self-contained: credentials, CA bundles
// and routes are inlined, with no references to other objects. The agent keeps a
// full copy on disk and works from it while the API server is unreachable, and
// at that point there is nothing left to dereference.
//
// It carries no per-node secrets. Everything here is global configuration,
// identical on every node, which is what makes it safe for the whole
// `system:nodes` group to read.
//
// The controller compiles every accepted RegistryUpstream into AdditionalRoutes
// so the agent watches ONE object instead of N: consistency across many objects
// cannot be guaranteed when the API is down, which would break the
// self-sufficiency the agent depends on.
type RegistryNodeSpec struct {
	// Cache is the cache axis for this node. True sends the agent to the storage,
	// false straight to the primary upstream.
	// +optional
	Cache bool `json:"cache,omitempty"`

	// Backends serve the PRIMARY source, in priority order.
	//
	// With the cache on, this is the storage followed by the upstream as a
	// fallback while the cache is still filling; the upstream entry disappears
	// when the cluster goes air-gap. With the cache off, it is the upstream
	// alone.
	// +optional
	// +listType=map
	// +listMapKey=name
	Backends []Backend `json:"backends,omitempty"`

	// AdditionalRoutes are the accepted RegistryUpstream objects compiled by the
	// controller. Always transit, never cached.
	// +optional
	// +listType=map
	// +listMapKey=match
	AdditionalRoutes []Route `json:"additionalRoutes,omitempty"`
}

// Backend is one source of the primary image set.
type Backend struct {
	// Name identifies the backend's role.
	Name BackendName `json:"name"`

	// Endpoint is how to reach it.
	Endpoint `json:",inline"`

	// Mirrors are additional addresses serving the same content, tried on
	// failure.
	// +optional
	Mirrors []Endpoint `json:"mirrors,omitempty"`
}

// Route sends pulls for one intercepted host to an upstream, in transit.
type Route struct {
	// Match is the registry host intercepted on this node.
	Match string `json:"match"`

	// Endpoint is where the intercepted pulls go.
	Endpoint `json:",inline"`

	// Mirrors are additional addresses serving the same content.
	// +optional
	Mirrors []Endpoint `json:"mirrors,omitempty"`
}

// RegistryNodeStatus is written by the agent ONLY, and reports containerd-side
// facts.
//
// It carries no storage completeness: that is a property of the storage and
// lives in RegistryStorage.status.replicas. Any node can patch any
// RegistryNode's status (RBAC cannot scope list/watch by field selector), so
// nothing here may be load-bearing for the pull path — it is observability.
type RegistryNodeStatus struct {
	// ObservedGeneration is the spec generation the agent last applied.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Reconciled means the agent has applied the spec it currently sees.
	// +optional
	Reconciled bool `json:"reconciled,omitempty"`

	// ProxyListening means the agent is serving the registry endpoint.
	// +optional
	ProxyListening bool `json:"proxyListening,omitempty"`

	// ActiveBackends are the backends the agent is currently able to use.
	// +optional
	ActiveBackends []string `json:"activeBackends,omitempty"`

	// StaleStorageDataBytes is how much of the in-cluster cache's data is still on
	// this node while no cache is configured.
	//
	// Reported by the agent, which is the only thing the module runs on every node. The
	// blobs are deliberately left behind when the cache is turned off — turning it back
	// on then refills from what is already there rather than from scratch, and on a
	// slow link that difference is hours. But nothing else would ever mention the disk
	// they occupy, so it is said here instead of quietly kept.
	//
	// Zero, and therefore absent, in every other case: while the cache is configured
	// the data is not stale, and on a node that never held it there is nothing to
	// report.
	// +optional
	StaleStorageDataBytes int64 `json:"staleStorageDataBytes,omitempty"`

	// Conditions carry Reconciled, including the case where the agent is running
	// from its on-disk copy because the API is unreachable.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Backend returns the named backend, or nil when this node has no such backend.
func (s *RegistryNodeSpec) Backend(name BackendName) *Backend {
	for i := range s.Backends {
		if s.Backends[i].Name == name {
			return &s.Backends[i]
		}
	}
	return nil
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,path=registrynodes,singular=registrynode,shortName=rnode

// RegistryNode is the registry layout of a single node, as compiled for its
// agent.
type RegistryNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryNodeSpec   `json:"spec,omitempty"`
	Status RegistryNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryNodeList is a list of RegistryNode.
type RegistryNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryNode `json:"items"`
}

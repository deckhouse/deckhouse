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

// RegistryConfigSpec is the resolved registry configuration.
//
// Written by the module from ModuleConfig/registry in the normal case, and by
// an administrator directly when the Deckhouse operator is down. The CR is a
// contract, not Deckhouse property: a hand edit is applied by the controller
// and the syncer on the fly and holds until the module comes back.
type RegistryConfigSpec struct {
	// Mode is whether the module manages the image pull path.
	// +kubebuilder:default=Managed
	// +optional
	Mode Mode `json:"mode,omitempty"`

	// Primary is the single source of Deckhouse components. All storage and fill
	// logic is about this upstream. Additional registries are declared as
	// separate RegistryUpstream objects instead, because more than one real
	// primary would make both "is the cache full" and "what fills the cache in
	// air-gap" ambiguous.
	// +optional
	Primary PrimarySource `json:"primary,omitempty"`

	// Storage configures the in-cluster cache.
	// +optional
	Storage StorageConfig `json:"storage,omitempty"`
}

// PrimarySource is the authoritative source of Deckhouse images.
type PrimarySource struct {
	// Upstream is the registry to pull from. Absent means air-gap: the cache is
	// authoritative and is populated out of band by `d8 mirror push`.
	// +optional
	Upstream *Upstream `json:"upstream,omitempty"`
}

// StorageConfig is the cache half of the two configuration axes.
type StorageConfig struct {
	// Cache is the first axis. False sends the agent straight to the upstream
	// and no storage is needed; true routes it through the in-cluster cache on
	// the master nodes.
	//
	// The second axis is whether Primary.Upstream is set. Together the two
	// replace the old mode state machine: (false, set) forwards directly,
	// (true, set) is a pass-through cache, (true, unset) is air-gap.
	// +optional
	Cache bool `json:"cache,omitempty"`

	// Size is the size of the persistent volume backing the cache.
	// +optional
	Size string `json:"size,omitempty"`

	// GarbageCollection is when the cache reclaims the disk taken by releases the cluster
	// has moved past. Compiled into RegistryStorage, which is what the replicas read.
	// +optional
	GarbageCollection *GarbageCollection `json:"garbageCollection,omitempty"`

	// Source describes the image set the cache is expected to hold. Required in
	// air-gap, where there is no upstream to fall back on and completeness must
	// be decidable.
	// +optional
	Source *StorageSource `json:"source,omitempty"`
}

// RegistryConfigStatus is the health summary written by the controller.
type RegistryConfigStatus struct {
	// ObservedGeneration is the spec generation the controller last acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// EffectiveUpstream is the upstream actually in effect, which is not
	// necessarily the configured one.
	//
	// This is the controller's own record of the last upstream that passed its
	// preflight probe, and it is what makes a bad change survivable: entering
	// credentials that do not work, or switching to a license that has no access,
	// leaves this field — and the cluster — on the previous working upstream while
	// UpstreamValid reports why.
	//
	// It also holds the upstream through the air-gap transition, where the
	// configuration already says "none" but the cache is not complete yet.
	//
	// Empty means no upstream is in effect: either the cluster is air-gapped, or
	// nothing has been configured yet.
	// Carries no credentials. This is a record, and a record of a credential is one
	// more place it can be read from — this resource is cluster-scoped, so that means
	// anyone allowed to look at the module's own configuration. The credentials that
	// belong to it are in the module's auth Secret, and EffectiveUpstreamAuthDigest is
	// what tells them apart without revealing them.
	// +optional
	EffectiveUpstream *Upstream `json:"effectiveUpstream,omitempty"`

	// EffectiveUpstreamAuthDigest identifies the credentials in effect without
	// disclosing them.
	//
	// Needed because "has the configuration changed?" has to include the credentials:
	// a new license under an unchanged address is exactly the change that must be
	// probed before the cluster is switched to it. With the credentials no longer
	// recorded, comparing this against a digest of the configured ones is what makes
	// that question answerable.
	// +optional
	EffectiveUpstreamAuthDigest string `json:"effectiveUpstreamAuthDigest,omitempty"`

	// Conditions carry Valid, UpstreamValid and StorageConverged.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// IsAirGap reports whether the configuration asks for air-gap: a cache with no
// upstream behind it.
//
// Note this is the DESIRED state. The controller keeps serving the last applied
// upstream until the storage leader is actually full, so desired and effective
// air-gap differ for as long as the fill takes.
func (s *RegistryConfigSpec) IsAirGap() bool {
	return s.Storage.Cache && s.Primary.Upstream == nil
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,path=registryconfigs,singular=registryconfig,shortName=rcfg

// RegistryConfig is the resolved registry configuration of the cluster.
type RegistryConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryConfigSpec   `json:"spec,omitempty"`
	Status RegistryConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryConfigList is a list of RegistryConfig.
type RegistryConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryConfig `json:"items"`
}

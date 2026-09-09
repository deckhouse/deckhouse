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

// Package v1alpha1 contains the API types of the registry module.
//
// The package lives in go_lib rather than inside a module because three
// separate Go modules consume it: the controller and the syncer
// (modules/038-registry) and the agent (image built in modules/038-registry,
// packaged for node delivery in modules/007-registrypackages).
//
// +kubebuilder:object:generate=true
// +groupName=deckhouse.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the API group of every type in this package.
const GroupName = "deckhouse.io"

// GroupVersion is the group/version of every type in this package.
var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

// SchemeGroupVersion is an alias of GroupVersion, kept for the client-go
// convention some generators and callers expect.
var SchemeGroupVersion = GroupVersion

var (
	// SchemeBuilder registers the types of this package into a runtime.Scheme.
	//
	// Built on apimachinery rather than controller-runtime's scheme helper on
	// purpose: the agent links this package and must stay a lightweight client,
	// so the API package pulls in no controller machinery. controller-runtime
	// accepts this AddToScheme just the same.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types of this package to a runtime.Scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&RegistryConfig{}, &RegistryConfigList{},
		&RegistryStorage{}, &RegistryStorageList{},
		&RegistryUpstream{}, &RegistryUpstreamList{},
		&RegistryNode{}, &RegistryNodeList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

// Kinds served by this package.
const (
	RegistryConfigKind   = "RegistryConfig"
	RegistryStorageKind  = "RegistryStorage"
	RegistryUpstreamKind = "RegistryUpstream"
	RegistryNodeKind     = "RegistryNode"
)

// SingletonName is the name of the cluster-wide singleton objects
// RegistryConfig and RegistryStorage. RegistryNode is named after its node and
// RegistryUpstream is named by whoever creates it, so neither uses this.
const SingletonName = "registry"

// Resource returns a fully-qualified GroupResource for the given resource.
func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

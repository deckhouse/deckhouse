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

// Package v1alpha1 contains the virtual resources of the registry module: served by the aggregated API
// of registry-controller, assembled on every request and never stored.
//
// A group of its own, and not deckhouse.io: an APIService takes a whole group, and deckhouse.io is
// served by custom resources. The group belongs to the aggregated API for good — a custom resource in it
// would never be served.
//
// +kubebuilder:object:generate=true
// +groupName=registry.deckhouse.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the API group of every type in this package.
const GroupName = "registry.deckhouse.io"

// GroupVersion is the group/version of every type in this package.
var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

// SchemeGroupVersion is an alias of GroupVersion, for the client-go convention.
var SchemeGroupVersion = GroupVersion

var (
	// SchemeBuilder registers the types of this package into a runtime.Scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types of this package to a runtime.Scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &RegistryImage{}, &RegistryImageList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

const (
	// RegistryImageKind is the kind of RegistryImage.
	RegistryImageKind = "RegistryImage"

	// RegistryImageResource is the resource RegistryImage is served as.
	RegistryImageResource = "registryimages"

	// RegistryImageSingular is the singular name of the resource.
	RegistryImageSingular = "registryimage"

	// RegistryImageShortName is what `kubectl get rimg` answers to.
	RegistryImageShortName = "rimg"
)

// Resource returns a fully-qualified GroupResource for the given resource.
func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

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

// Package v1alpha1 contains the API Schema of the
// templates.internal.deckhouse.io v1alpha1 API group: the resources the
// aggregated API server renders on the fly and never stores.
//
// A group of its own, and that is the whole point of it. kube-aggregator routes
// by group and version: an APIService that names a service takes the entire
// group version with it, and the CRDs of that group version stop being served.
// NodeConfig is a CRD in internal.deckhouse.io/v1alpha1, so a template
// aggregated there would take the object every olcedar node reads out of the
// cluster.
// +kubebuilder:object:generate=true
// +groupName=templates.internal.deckhouse.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group version these objects are registered under.
	GroupVersion = schema.GroupVersion{Group: "templates.internal.deckhouse.io", Version: "v1alpha1"}

	// SchemeBuilder adds the go types of this group version to a scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types of this group version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

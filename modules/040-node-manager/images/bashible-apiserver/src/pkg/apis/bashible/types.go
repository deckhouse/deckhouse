/*
Copyright 2017 The Kubernetes Authors.

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

package bashible

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReferenceType defines the type of an object reference.
type ReferenceType string

const (
	BashibleReferenceType        = ReferenceType("Bashible")
	NodeGroupBundleReferenceType = ReferenceType("NodeGroupBundle")
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bashible contains bashible entrypoint script
type Bashible struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Data map[string]string
}

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BashibleList is a list of Bashible objects.
type BashibleList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items is a list of Bashibles
	Items []Bashible
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeGroupBundle represents the set of bashible steps for a node group
type NodeGroupBundle struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Data contains bashible scripts by name
	Data map[string]string
}

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeGroupBundleList is a list of NodeGroupBundle objects.
type NodeGroupBundleList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []NodeGroupBundle
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bootstrap object contains script to perform initialization of a Kubernetes Node
type Bootstrap struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Bootstrap fields contains the actual script
	Bootstrap string
}

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BootstrapList is a List of Bootstrap object
type BootstrapList struct {
	metav1.TypeMeta
	metav1.ListMeta

	// Items is a List of Bootstraps
	Items []Bootstrap
}

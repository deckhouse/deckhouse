/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	APIGroup         = "network.deckhouse.io"
	InternalAPIGroup = "internal.network.deckhouse.io"
	APIVersion       = "v1alpha1"
	RTKind           = "RoutingTable"
	NRTKind          = "SDNInternalNodeRoutingTable"
	IRSKind          = "IPRuleSet"
	NIRSKind         = "SDNInternalNodeIPRuleSet"
)

// SchemeGroupVersion is group version used to register these objects
var (
	SchemeGroupVersion = schema.GroupVersion{
		Group:   APIGroup,
		Version: APIVersion,
	}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme

	InternalSchemeGroupVersion = schema.GroupVersion{
		Group:   InternalAPIGroup,
		Version: APIVersion,
	}
	InternalSchemeBuilder = runtime.NewSchemeBuilder(addKnownInternalTypes)
	AddInternalToScheme   = InternalSchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&RoutingTable{},
		&RoutingTableList{},
		&IPRuleSet{},
		&IPRuleSetList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// Adds the list of known Internal types to Scheme.
func addKnownInternalTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalSchemeGroupVersion,
		&SDNInternalNodeRoutingTable{},
		&SDNInternalNodeRoutingTableList{},
		&SDNInternalNodeIPRuleSet{},
		&SDNInternalNodeIPRuleSetList{},
	)
	metav1.AddToGroupVersion(scheme, InternalSchemeGroupVersion)
	return nil
}

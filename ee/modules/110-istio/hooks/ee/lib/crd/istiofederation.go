/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package crd

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type IstioFederation struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node group.
	Spec IstioFederationSpec `json:"spec"`

	Status IstioFederationStatus `json:"status"`
}

type IstioFederationSpec struct {
	MetadataEndpoint string `json:"metadataEndpoint"`
	TrustDomain      string `json:"trustDomain,omitempty"`
}

type IstioFederationStatus struct {
	MetadataCache struct {
		Public                    *AlliancePublicMetadata    `json:"public"`
		Private                   *FederationPrivateMetadata `json:"private"`
		PublicLastFetchTimestamp  string                     `json:"publicLastFetchTimestamp"`
		PrivateLastFetchTimestamp string                     `json:"privateLastFetchTimestamp"`
	} `json:"metadataCache,omitempty"`
}

// Warning! This struct is duplicated in images/metadata-exporter
type FederationPrivateMetadata struct {
	IngressGateways *[]FederationIngressGateways `json:"ingressGateways"`
	PublicServices  *[]FederationPublicServices  `json:"publicServices"`
}

type FederationIngressGateways struct {
	Address string `json:"address"`
	Port    uint   `json:"port"`
}

type FederationPublicServices struct {
	Hostname string `json:"hostname"`
	Ports    []struct {
		Name string `json:"name"`
		Port uint   `json:"port"`
	} `json:"ports"`
	VirtualIP string `json:"virtualIP,omitempty"`
}

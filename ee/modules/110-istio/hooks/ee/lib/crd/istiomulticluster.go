/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
package crd

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type IstioMulticluster struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node group.
	Spec IstioMulticlusterSpec `json:"spec"`

	Status IstioMulticlusterStatus `json:"status"`
}

type IstioMulticlusterSpec struct {
	MetadataEndpoint     string `json:"metadataEndpoint"`
	EnableIngressGateway bool   `json:"enableIngressGateway"`
}

type IstioMulticlusterStatus struct {
	MetadataCache struct {
		Public                    *AlliancePublicMetadata      `json:"public"`
		Private                   *MulticlusterPrivateMetadata `json:"private"`
		PublicLastFetchTimestamp  string                       `json:"publicLastFetchTimestamp"`
		PrivateLastFetchTimestamp string                       `json:"privateLastFetchTimestamp"`
	} `json:"metadataCache,omitempty"`
}

// Warning! This struct is duplicated in images/metadata-exporter
type MulticlusterPrivateMetadata struct {
	IngressGateways *[]MulticlusterIngressGateways `json:"ingressGateways"`
	APIHost         string                         `json:"apiHost,omitempty"`
	NetworkName     string                         `json:"networkName,omitempty"`
}

type MulticlusterIngressGateways struct {
	Address string `json:"address"`
	Port    uint   `json:"port"`
}

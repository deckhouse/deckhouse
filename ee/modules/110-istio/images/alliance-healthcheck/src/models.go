/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

type FederationPrivateMetadata struct {
	IngressGateways *[]FederationIngressGateway `json:"ingressGateways"`
	PublicServices  *[]FederationPublicService  `json:"publicServices"`
}

type FederationIngressGateway struct {
	Address string `json:"address"`
	Port    uint   `json:"port"`
}

type FederationPublicService struct {
	Hostname string `json:"hostname"`
	Ports    []struct {
		Name     string `json:"name"`
		Port     uint   `json:"port"`
		Protocol string `json:"protocol"`
	} `json:"ports"`
}

type AlliancePublicMetadata struct {
	ClusterUUID string `json:"clusterUUID"`
	AuthnKeyPub string `json:"authnKeyPub"`
	RootCA      string `json:"rootCA"`
}
